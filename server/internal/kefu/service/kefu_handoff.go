package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"admin/server/internal/kefu/im"
	"admin/server/internal/kefu/repository"
	"admin/server/model"
	"admin/server/pkg/log"
	"admin/server/pkg/setting"
)

// 转人工使用到的配置项与常量
const (
	// handoffKeywordConfigKey 是转人工关键词的配置键，多个关键词用逗号分隔
	handoffKeywordConfigKey = "kefu.handoff.keywords"
	// handoffWorkTimeConfigKey 是人工客服在线时间的配置键，直接作为提示文案下发
	handoffWorkTimeConfigKey = "kefu.handoff.work_time"
	// defaultHandoffWorkTime 是未配置在线时间时的默认文案
	defaultHandoffWorkTime = "上午 9:00-12:00，下午 14:00-19:00"
	// handoffKeywordDictCode 是转人工关键词字典标识，配置项为空时回退到字典
	handoffKeywordDictCode = "kefu_handoff_keyword"
	// handoffNoticeText 是有人工客服在线时发给客户的提示
	handoffNoticeText = "正在为您转人工，请稍后"
	// handoffOfflineText 是没有人工客服在线时发给客户的提示，带上在线时间
	handoffOfflineText = "人工客服在线时间：%s，您可先留言，客服上线后会尽快回复您"
	// queueNoticeText 是推给在线客服的转人工提醒文案
	queueNoticeText = "客户要求转人工，请尽快接待"
)

// 转人工的业务错误
var (
	// ErrHandoffFailed 是转人工写库失败，调用方按服务端错误返回
	ErrHandoffFailed = errors.New("转人工失败，请稍后重试")
)

// KefuHandoffService 处理客户主动要求转人工：
// AI 进行中的会话命中关键词或访客主动点击「转人工」后停止 AI 回复，并把会话转为待接待
type KefuHandoffService struct {
	// 会话的数据访问
	sessionRepository *repository.KefuSessionRepository
	// AI 回复记录的数据访问，转人工时中断进行中的回复
	replyRepository *repository.KefuAiReplyRepository
	// 字典的数据访问，配置项为空时读取转人工关键词
	dictionaryRepository *repository.KefuDictionaryRepository
	// 客服挑选的业务处理，判断是否有人工客服在线
	assignService *KefuAssignService
	// 会话的业务处理，访客主动转人工时校验会话归属
	sessionService *KefuSessionService
}

// NewKefuHandoffService 创建转人工的业务处理
func NewKefuHandoffService() *KefuHandoffService {
	return &KefuHandoffService{
		sessionRepository:    repository.NewKefuSessionRepository(),
		replyRepository:      repository.NewKefuAiReplyRepository(),
		dictionaryRepository: repository.NewKefuDictionaryRepository(),
		assignService:        NewKefuAssignService(),
		sessionService:       NewKefuSessionService(),
	}
}

// Handoff 判断客户消息是否要求转人工，命中且当前由 AI 接待时停止 AI 回复、
// 把会话转为待接待；返回 true 表示已经转人工，调用方不要再触发 AI 回复
func (s *KefuHandoffService) Handoff(
	ctx context.Context, session *model.KefuSession, content string,
) bool {
	// 关键词只在 AI 进行中检测，人工进行中与待接待的会话不再重复转人工
	if session == nil || session.Status != model.KefuSessionStatusAi {
		return false
	}
	if matchTransferKeyword(content, s.keywords()) == "" {
		return false
	}
	return s.toQueue(ctx, session, s.noticeText(s.assignService.HasOnlineAgent(ctx)))
}

// HandoffByVisitor 是访客点击「转人工」的转接：不校验关键词，AI 进行中与待接待都允许；
// 已经是待接待时直接返回当前状态，保证重复点击不会重复写系统消息
func (s *KefuHandoffService) HandoffByVisitor(
	ctx context.Context, sessionID int64, visitorKey string,
) (*model.KefuSessionHandoffResult, error) {
	session, err := s.sessionService.ClientSession(sessionID, visitorKey)
	if err != nil {
		return nil, err
	}
	// 人工进行中、已结束与留言的会话不再允许转回待接待
	if session.Status != model.KefuSessionStatusAi &&
		session.Status != model.KefuSessionStatusQueueing {
		return nil, ErrSessionStatusInvalid
	}

	online := s.assignService.HasOnlineAgent(ctx)
	if session.Status == model.KefuSessionStatusQueueing {
		return &model.KefuSessionHandoffResult{
			SessionID:   session.SessionID,
			Status:      session.Status,
			OnlineAgent: online,
		}, nil
	}

	if !s.toQueue(ctx, session, s.noticeText(online)) {
		return nil, ErrHandoffFailed
	}
	return &model.KefuSessionHandoffResult{
		SessionID:   session.SessionID,
		Status:      model.KefuSessionStatusQueueing,
		OnlineAgent: online,
	}, nil
}

// HandoffByAi 是 AI 上报无法回答时的转人工：不校验关键词，
// 固定话术已由 AI 回复发出去，因此不再补系统消息提示，只做转待接待与客服提醒
func (s *KefuHandoffService) HandoffByAi(
	ctx context.Context, session *model.KefuSession,
) bool {
	if session == nil || session.Status != model.KefuSessionStatusAi {
		return false
	}
	return s.toQueue(ctx, session, "")
}

// toQueue 把 AI 进行中的会话转为待接待，并通知在线客服；
// notice 为空时不往会话里补系统消息
func (s *KefuHandoffService) toQueue(
	ctx context.Context, session *model.KefuSession, notice string,
) bool {
	// 先中断进行中的 AI 回复，避免转人工后 AI 还在继续输出
	if replyID, err := s.interrupt(ctx, session.SessionID); err != nil {
		log.Logger.Warn("interrupt kefu ai reply for handoff failed",
			zap.Int64("sessionId", session.SessionID), zap.Error(err))
	} else if replyID > 0 {
		log.Logger.Debug("interrupted kefu ai reply for handoff",
			zap.Int64("sessionId", session.SessionID), zap.Int64("replyId", replyID))
	}

	now := model.Now()
	if err := s.sessionRepository.UpdateFields(session.SessionID, map[string]any{
		"agent_id":    0,
		"status":      model.KefuSessionStatusQueueing,
		"ai_mode":     model.KefuSessionAiModeOff,
		"queue_time":  now,
		"start_time":  model.Time{},
		"update_time": now,
	}); err != nil {
		log.Logger.Warn("handoff kefu session failed",
			zap.Int64("sessionId", session.SessionID), zap.Error(err))
		return false
	}
	session.AgentID = 0
	session.Status = model.KefuSessionStatusQueueing
	session.AiMode = model.KefuSessionAiModeOff

	if notice != "" {
		if err := s.assignService.sendSystemMessage(ctx, session.SessionID, notice); err != nil {
			log.Logger.Warn("send kefu handoff message failed",
				zap.Int64("sessionId", session.SessionID), zap.Error(err))
		}
	}
	// 客服没有订阅待接待会话的 IM 频道，靠这条推送才能及时收到提醒
	s.notifyQueue(ctx, session)
	return true
}

// notifyQueue 给在线的客服推送一条转人工通知，
// 工作台收到后播放提示音并刷新会话列表，推送失败只记日志不影响转人工
func (s *KefuHandoffService) notifyQueue(ctx context.Context, session *model.KefuSession) {
	agents := s.assignService.OnlineAgents(ctx)
	if len(agents) == 0 {
		return
	}

	payload, err := json.Marshal(&im.Payload{
		Version:   1,
		Type:      im.PayloadTypeQueue,
		Role:      im.PayloadRoleSystem,
		Content:   queueNoticeText,
		SessionID: session.SessionID,
	})
	if err != nil {
		log.Logger.Warn("marshal kefu queue notice failed",
			zap.Int64("sessionId", session.SessionID), zap.Error(err))
		return
	}

	for _, agent := range agents {
		// 通知不需要漫游历史，不落库只推给在线端
		if _, err := im.Default.SendMessage(ctx, &im.MessageSendReq{
			Header:      im.MessageHeader{NoPersist: 1},
			ClientMsgNo: generateClientMsgNo("queue"),
			FromUID:     im.AiUID(),
			ChannelID:   agentUID(agent.UserID),
			ChannelType: im.ChannelTypePerson,
			Payload:     payload,
		}); err != nil {
			log.Logger.Warn("send kefu queue notice failed",
				zap.Int64("sessionId", session.SessionID),
				zap.Int64("agentId", agent.AgentID), zap.Error(err))
		}
	}
}

// keywords 读取转人工关键词，配置项为空时回退到字典，便于兼容已维护的字典数据
func (s *KefuHandoffService) keywords() []string {
	value := strings.TrimSpace(setting.GetString(handoffKeywordConfigKey, ""))
	if value != "" {
		list := make([]string, 0, 8)
		for _, item := range strings.Split(value, ",") {
			if item = strings.TrimSpace(item); item != "" {
				list = append(list, item)
			}
		}
		return list
	}

	keywords, err := s.dictionaryRepository.ListCodes(handoffKeywordDictCode)
	if err != nil {
		log.Logger.Warn("load kefu handoff keywords failed", zap.Error(err))
		return nil
	}
	return keywords
}

// noticeText 按当前是否有人工客服在线返回转人工的提示文案
func (s *KefuHandoffService) noticeText(online bool) string {
	if online {
		return handoffNoticeText
	}
	workTime := strings.TrimSpace(setting.GetString(handoffWorkTimeConfigKey, ""))
	if workTime == "" {
		workTime = defaultHandoffWorkTime
	}
	return fmt.Sprintf(handoffOfflineText, workTime)
}

// interrupt 给会话进行中的 AI 回复置中断标记，没有进行中的回复时返回 0
func (s *KefuHandoffService) interrupt(ctx context.Context, sessionID int64) (int64, error) {
	reply, err := s.replyRepository.GetStreamingBySession(sessionID)
	if err != nil {
		if errors.Is(err, repository.ErrAiReplyNotFound) {
			return 0, nil
		}
		return 0, err
	}
	if err := im.MarkInterrupt(ctx, reply.ReplyID, aiInterruptByHandoff); err != nil {
		return reply.ReplyID, err
	}
	return reply.ReplyID, nil
}
