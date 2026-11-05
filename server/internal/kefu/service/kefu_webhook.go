package service

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"admin/server/internal/kefu/im"
	"admin/server/internal/kefu/repository"
	"admin/server/model"
	"admin/server/pkg/log"
)

// aiReplyBudget 是异步 AI 回复的总预算，模型超时在编排内部另有配置
const aiReplyBudget = 5 * time.Minute

// KefuWebhookService 是 IM 回调的业务处理
type KefuWebhookService struct {
	// 会话的数据访问
	sessionRepository *repository.KefuSessionRepository
	// 会话消息的数据访问
	messageRepository *repository.KefuMessageRepository
	// AI 自动回复的编排
	aiService *KefuAiService
	// 客服在线状态的业务处理
	onlineService *KefuOnlineService
	// 客户要求转人工的业务处理
	handoffService *KefuHandoffService
}

// NewKefuWebhookService 创建 IM 回调的业务处理
func NewKefuWebhookService() *KefuWebhookService {
	return &KefuWebhookService{
		sessionRepository: repository.NewKefuSessionRepository(),
		messageRepository: repository.NewKefuMessageRepository(),
		aiService:         NewKefuAiService(),
		onlineService:     NewKefuOnlineService(),
		handoffService:    NewKefuHandoffService(),
	}
}

// ValidateToken 校验 IM 回调的共享密钥，未配置密钥时跳过校验（本地联调）
func (s *KefuWebhookService) ValidateToken(token string) bool {
	expected := im.WebhookToken()
	if expected == "" {
		return true
	}
	return subtle.ConstantTimeCompare([]byte(strings.TrimSpace(token)), []byte(expected)) == 1
}

// HandleMessage 处理 IM 的消息回调：客户与客服消息按 client_msg_no 幂等落库，
// 落库后同步更新会话的最后消息时间与未读数；无法解析的请求只记日志，不阻塞 IM。
// IM 的回调体是消息数组，单条对象也兼容，方便手工回放
func (s *KefuWebhookService) HandleMessage(ctx context.Context, body []byte) {
	notifies := decodeMessageNotifies(body)
	for i := range notifies {
		s.handleMessage(ctx, &notifies[i])
	}
}

// decodeMessageNotifies 解析消息回调体，IM 正常投递是数组，单条对象也兼容
func decodeMessageNotifies(body []byte) []im.MessageNotify {
	var list []im.MessageNotify
	if err := json.Unmarshal(body, &list); err == nil {
		return list
	}

	var notify im.MessageNotify
	if err := json.Unmarshal(body, &notify); err != nil {
		log.Logger.Warn("decode kefu webhook message failed", zap.Error(err))
		return nil
	}
	return []im.MessageNotify{notify}
}

// handleMessage 处理回调里的单条消息
func (s *KefuWebhookService) handleMessage(ctx context.Context, notify *im.MessageNotify) {
	payload, err := notify.DecodePayload()
	if err != nil {
		log.Logger.Warn("decode kefu webhook payload failed",
			zap.String("clientMsgNo", notify.ClientMsgNo), zap.Error(err))
		return
	}

	// 「正在输入」只是瞬时状态，不落库也不触发 AI，直接忽略
	if payload.Type == im.PayloadTypeTyping {
		return
	}

	sessionID := notify.SessionID()
	if sessionID == 0 {
		sessionID = payload.SessionID
	}
	if sessionID == 0 || strings.TrimSpace(notify.ClientMsgNo) == "" {
		log.Logger.Warn("skip kefu webhook message without session or clientMsgNo",
			zap.String("channelId", notify.ChannelID), zap.String("clientMsgNo", notify.ClientMsgNo))
		return
	}

	// AI 回复与系统消息由 AI 编排与业务代码自己落库，回调只负责客户与客服消息
	fromRole := resolveFromRole(payload, notify.FromUID)
	if fromRole != model.KefuMessageFromVisitor && fromRole != model.KefuMessageFromAgent {
		return
	}

	session, err := s.sessionRepository.GetByID(sessionID)
	if err != nil {
		log.Logger.Warn("skip kefu webhook message, session not found",
			zap.Int64("sessionId", sessionID), zap.Error(err))
		return
	}

	now := model.Now()
	raw, err := json.Marshal(payload)
	if err != nil {
		log.Logger.Warn("marshal kefu message payload failed", zap.Error(err))
		return
	}
	message := &model.KefuMessage{
		SessionID:    sessionID,
		ClientMsgNo:  strings.TrimSpace(notify.ClientMsgNo),
		IMMessageID:  strconv.FormatInt(notify.MessageID, 10),
		IMMessageSeq: int64(notify.MessageSeq),
		FromRole:     fromRole,
		FromID:       notify.FromUID,
		ContentType:  messageContentType(payload.Type),
		Content:      payload.Content,
		Payload:      string(raw),
		CreateTime:   now,
		UpdateTime:   now,
	}
	inserted, err := s.messageRepository.CreateIfAbsent(message)
	if err != nil {
		log.Logger.Error("save kefu message failed",
			zap.String("clientMsgNo", message.ClientMsgNo), zap.Error(err))
		return
	}
	// 同一条消息重复投递时不再重复累加会话计数
	if !inserted {
		return
	}

	unreadDelta := 0
	if fromRole == model.KefuMessageFromVisitor {
		unreadDelta = 1
	}
	if err := s.sessionRepository.UpdateMessageStat(sessionID, now, unreadDelta); err != nil {
		log.Logger.Error("update kefu session stat failed",
			zap.Int64("sessionId", sessionID), zap.Error(err))
	}

	// 客户消息落库后先看是不是要求转人工，命中时中断 AI 回复并转待接待，
	// 没有命中再异步触发 AI 编排，失败只记日志，不影响回调返回
	if fromRole == model.KefuMessageFromVisitor {
		if !s.handoffService.Handoff(ctx, session, payload.Content) {
			s.triggerAiReply(sessionID, message.MessageID)
		}
	}
}

// triggerAiReply 异步触发一次 AI 自动回复，开关判断在编排内部完成
func (s *KefuWebhookService) triggerAiReply(sessionID, messageID int64) {
	go func() {
		// 回调已经返回，这里用独立上下文，只用一个总超时防止 goroutine 长期占用
		ctx, cancel := context.WithTimeout(context.Background(), aiReplyBudget)
		defer cancel()
		if err := s.aiService.Reply(ctx, sessionID, messageID); err != nil {
			if errors.Is(err, ErrAiDisabled) {
				log.Logger.Debug("kefu ai reply skipped",
					zap.Int64("sessionId", sessionID), zap.Error(err))
				return
			}
			log.Logger.Error("kefu ai reply failed",
				zap.Int64("sessionId", sessionID),
				zap.Int64("messageId", messageID), zap.Error(err))
		}
	}()
}

// resolveFromRole 解析消息的发送者角色，payload 未带角色时按 uid 前缀推断
func resolveFromRole(payload *im.Payload, fromUID string) string {
	switch payload.Role {
	case im.PayloadRoleVisitor, im.PayloadRoleAgent, im.PayloadRoleAi, im.PayloadRoleSystem:
		return payload.Role
	}
	switch {
	case strings.HasPrefix(fromUID, "visitor_"):
		return model.KefuMessageFromVisitor
	case strings.HasPrefix(fromUID, "agent_"):
		return model.KefuMessageFromAgent
	case fromUID != "" && fromUID == im.AiUID():
		return model.KefuMessageFromAi
	default:
		return model.KefuMessageFromSystem
	}
}

// messageContentType 归一化消息的内容类型，未知类型按文本处理
func messageContentType(contentType string) string {
	switch contentType {
	case model.KefuMessageTypeText, model.KefuMessageTypeImage,
		model.KefuMessageTypeFile, model.KefuMessageTypeSystem,
		model.KefuMessageTypeAiAnswer:
		return contentType
	default:
		return model.KefuMessageTypeText
	}
}

// HandleOnline 处理 IM 的在线状态回调，只更新在线缓存；
// 待接待的会话由客服在工作台主动领取，不做自动分配
func (s *KefuWebhookService) HandleOnline(ctx context.Context, body []byte) {
	s.onlineService.HandleNotify(ctx, body)
}
