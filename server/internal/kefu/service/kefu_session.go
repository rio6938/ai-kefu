package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"go.uber.org/zap"

	"admin/server/internal/kefu/im"
	"admin/server/internal/kefu/repository"
	"admin/server/model"
	"admin/server/pkg/log"
	rdb "admin/server/pkg/redis"
	"admin/server/pkg/setting"
)

// 客服模块使用到的配置项
const (
	// configKeyAiEnabled 是 AI 自动回复的总开关
	configKeyAiEnabled = "kefu.ai.enabled"
)

// 客服端 IM 凭据的 redis 缓存前缀，后缀是 IM 的设备标识；
// 带设备标识是为了不再命中按 App 设备登记的历史 token，那些 token 用 Web 标识连接会鉴权失败
const agentTokenKeyPrefix = "kefu:agent:imtoken:web:"

// assignNoticeText 是客服接待或接管后发给客户的系统提示，
// 让客户知道 AI 已停止、后续由人工客服回复
const assignNoticeText = "人工客服已接入"

// closedNoticeText 是客服结束会话后发给客户的系统提示，
// 客户端收到这条消息后会刷新会话状态并展示评价卡片
const closedNoticeText = "会话已结束，感谢您的咨询"

// 常见的业务错误
var (
	// ErrSessionNotFound 是会话不存在
	ErrSessionNotFound = repository.ErrSessionNotFound
	// ErrAgentNotFound 是客服档案不存在
	ErrAgentNotFound = repository.ErrAgentNotFound
	// ErrNotAgent 是当前账号没有客服档案
	ErrNotAgent = errors.New("当前账号不是客服")
	// ErrSessionNoPermission 是无权操作该会话
	ErrSessionNoPermission = errors.New("无权操作该会话")
	// ErrSessionStatusInvalid 是会话状态不允许该操作
	ErrSessionStatusInvalid = errors.New("会话状态不允许该操作")
	// ErrSessionAlreadyAssigned 是会话已被其他客服接待
	ErrSessionAlreadyAssigned = errors.New("会话已被其他客服接待")
	// ErrMessageContentRequired 是消息内容不能为空
	ErrMessageContentRequired = errors.New("消息内容不能为空")
	// ErrMessageContentTypeInvalid 是消息内容类型不支持
	ErrMessageContentTypeInvalid = errors.New("消息内容类型不支持")
	// ErrAssignAgentForbidden 是只有客服主管可以指定接待客服
	ErrAssignAgentForbidden = errors.New("只有客服主管可以指定接待客服")
	// ErrAgentDisabled 是接待客服已停用
	ErrAgentDisabled = errors.New("接待客服已停用")
	// ErrSessionIDsRequired 是未选择要结束的会话
	ErrSessionIDsRequired = errors.New("请选择要结束的会话")
	// ErrMessageQuoteInvalid 是引用的消息不属于当前会话
	ErrMessageQuoteInvalid = errors.New("引用的消息不属于该会话")
	// ErrSessionRateDisabled 是会话评价未开启
	ErrSessionRateDisabled = errors.New("会话评价未开启")
	// ErrSessionRateScoreInvalid 是评价星级不正确
	ErrSessionRateScoreInvalid = errors.New("请选择 1-5 星的评价")
	// ErrSessionNotClosed 是会话还没结束，不能评价
	ErrSessionNotClosed = errors.New("会话结束后才能评价")
	// ErrSessionRated 是该会话已经评价过
	ErrSessionRated = errors.New("该会话已评价")
)

// 会话标签与备注相关的常量
const (
	// maxSessionTagCount 是单个会话最多可打的标签数
	maxSessionTagCount = 10
	// maxSessionTagLength 是单个标签编码的最大字数
	maxSessionTagLength = 64
	// maxSessionRemarkLength 是内部备注的最大字数
	maxSessionRemarkLength = 500
)

// 会话评价相关的常量
const (
	// maxRateRemarkLength 是评价备注的最大字数
	maxRateRemarkLength = 200
	// maxRateTagCount 是单次评价最多可选的标签数
	maxRateTagCount = 10
	// maxRateTagLength 是单个标签的最大字数
	maxRateTagLength = 32
)

// KefuSessionService 是会话的业务处理
type KefuSessionService struct {
	// 会话的数据访问
	sessionRepository *repository.KefuSessionRepository
	// 会话消息的数据访问
	messageRepository *repository.KefuMessageRepository
	// 访客的数据访问
	visitorRepository *repository.KefuVisitorRepository
	// 客服档案的数据访问
	agentRepository *repository.KefuAgentRepository
	// 会话转接记录的数据访问
	transferRepository *repository.KefuSessionTransferRepository
	// AI 回复记录的数据访问，客服接待时中断进行中的回复
	replyRepository *repository.KefuAiReplyRepository
	// 会话转接的业务处理，详情里回填建议转接
	transferService *KefuTransferService
	// 客服接待提示的系统消息由分配服务统一写入频道
	assignService *KefuAssignService
	// 关联链的业务处理，新建会话时快照渠道所属的关联链
	routeService *KefuRouteService
	// 会话标签的数据访问
	tagRepository *repository.KefuSessionTagRepository
	// 渠道的数据访问，会话列表回填渠道类型与邮箱信息
	channelRepository *repository.KefuChannelRepository
	// 邮件线程的数据访问，邮件会话回填主题
	threadRepository *repository.KefuMailThreadRepository
	// 邮箱配置的数据访问，邮件会话回填收件人
	mailboxRepository *repository.KefuMailboxRepository
	// 邮件的出站编排，邮件会话发信走 SMTP
	mailSendService *KefuMailSendService
}

// NewKefuSessionService 创建会话的业务处理
func NewKefuSessionService() *KefuSessionService {
	return &KefuSessionService{
		sessionRepository:  repository.NewKefuSessionRepository(),
		messageRepository:  repository.NewKefuMessageRepository(),
		visitorRepository:  repository.NewKefuVisitorRepository(),
		agentRepository:    repository.NewKefuAgentRepository(),
		transferRepository: repository.NewKefuSessionTransferRepository(),
		replyRepository:    repository.NewKefuAiReplyRepository(),
		transferService:    NewKefuTransferService(),
		assignService:      NewKefuAssignService(),
		routeService:       NewKefuRouteService(),
		tagRepository:      repository.NewKefuSessionTagRepository(),
		channelRepository:  repository.NewKefuChannelRepository(),
		threadRepository:   repository.NewKefuMailThreadRepository(),
		mailboxRepository:  repository.NewKefuMailboxRepository(),
		mailSendService:    NewKefuMailSendService(),
	}
}

// EnsureSession 返回访客当前可用的会话，没有时新建会话并创建 IM 群频道；
// 新建会话默认由 AI 接待，只有客户要求转人工后才进入待接待
func (s *KefuSessionService) EnsureSession(
	ctx context.Context, visitor *model.KefuVisitor, channel *model.KefuChannel,
) (*model.KefuSession, error) {
	session, err := s.sessionRepository.GetActiveByVisitor(visitor.VisitorID)
	if err == nil {
		// 复用会话时补齐频道成员，IM 暂不可用不影响访客看历史消息
		if err := s.upsertChannel(ctx, session); err != nil {
			log.Logger.Warn("upsert kefu session channel failed",
				zap.Int64("sessionId", session.SessionID), zap.Error(err))
		}
		return session, nil
	}
	if !errors.Is(err, repository.ErrSessionNotFound) {
		return nil, err
	}

	now := model.Now()
	// AI 总开关关闭时没有人工接待，直接进待接待等客服领取
	aiMode := aiModeFromConfig()
	status := model.KefuSessionStatusAi
	if aiMode != model.KefuSessionAiModeOn {
		status = model.KefuSessionStatusQueueing
	}
	session = &model.KefuSession{
		SessionNo:       generateSessionNo(),
		ChannelCode:     channel.ChannelCode,
		VisitorID:       visitor.VisitorID,
		RouteID:         s.channelRouteID(channel.ChannelID),
		Status:          status,
		AiMode:          aiMode,
		SourceURL:       visitor.SourceURL,
		QueueTime:       now,
		LastMessageTime: now,
		CreateTime:      now,
		UpdateTime:      now,
	}
	if err := s.sessionRepository.Create(session); err != nil {
		return nil, err
	}
	if err := s.upsertChannel(ctx, session); err != nil {
		return nil, err
	}
	return session, nil
}

// Page 分页查询会话，普通客服只能看到自己名下的人工会话，主管看全部；
// 待接待与 AI 进行中的会话没有接待客服，所有客服都能看到，便于领取接待
func (s *KefuSessionService) Page(
	userID int64, param model.KefuSessionParam,
) ([]model.KefuSession, int64, error) {
	scopeAgentID, err := s.scopeAgentID(userID)
	if err != nil {
		return nil, 0, err
	}

	list, count, err := s.sessionRepository.Page(param, param.PageParam, scopeAgentID)
	if err != nil {
		return nil, 0, err
	}
	s.fillSessionExtra(list)
	return list, count, nil
}

// Detail 查询会话详情
func (s *KefuSessionService) Detail(userID int64, sessionID int64) (*model.KefuSession, error) {
	scopeAgentID, err := s.scopeAgentID(userID)
	if err != nil {
		return nil, err
	}
	session, err := s.getAccessible(scopeAgentID, sessionID)
	if err != nil {
		return nil, err
	}

	list := []model.KefuSession{*session}
	s.fillSessionExtra(list)
	// 建议转接由转接规则判断，会话详情直接回填给工作台
	list[0].SuggestTransfer = s.transferService.suggestFor(&list[0])
	// AI 接待与待接待的会话没有客服成员，查看时订阅才能实时收到新消息
	s.subscribeAgent(userID, session.SessionID)
	return &list[0], nil
}

// Messages 查询会话的消息，客服只能查询自己能看到的会话
func (s *KefuSessionService) Messages(
	userID int64, sessionID int64, param model.KefuMessageParam,
) ([]model.KefuMessage, int64, error) {
	scopeAgentID, err := s.scopeAgentID(userID)
	if err != nil {
		return nil, 0, err
	}
	if _, err := s.getAccessible(scopeAgentID, sessionID); err != nil {
		return nil, 0, err
	}
	// AI 接待与待接待的会话没有客服成员，查看时订阅才能实时收到新消息
	s.subscribeAgent(userID, sessionID)
	return s.messageRepository.ListBySession(sessionID, param)
}

// SearchMessages 在会话内搜索消息正文，客服只能搜索自己能看到的会话
func (s *KefuSessionService) SearchMessages(
	userID int64, sessionID int64, keyword string,
) ([]model.KefuMessage, error) {
	scopeAgentID, err := s.scopeAgentID(userID)
	if err != nil {
		return nil, err
	}
	if _, err := s.getAccessible(scopeAgentID, sessionID); err != nil {
		return nil, err
	}
	return s.messageRepository.SearchBySession(sessionID, strings.TrimSpace(keyword), 50)
}

// Transfers 查询会话的转接记录，会话详情页用它回放分配与转接过程
func (s *KefuSessionService) Transfers(
	userID int64, sessionID int64,
) ([]model.KefuSessionTransfer, error) {
	scopeAgentID, err := s.scopeAgentID(userID)
	if err != nil {
		return nil, err
	}
	if _, err := s.getAccessible(scopeAgentID, sessionID); err != nil {
		return nil, err
	}
	list, err := s.transferRepository.ListBySession(sessionID)
	if err != nil {
		return nil, err
	}
	s.fillTransferAgentNames(list)
	return list, nil
}

// fillTransferAgentNames 回填转接记录里转出与转入客服的名称
func (s *KefuSessionService) fillTransferAgentNames(
	list []model.KefuSessionTransfer,
) {
	if len(list) == 0 {
		return
	}

	agentIDs := make([]int64, 0, len(list)*2)
	seen := make(map[int64]struct{}, len(list)*2)
	for _, item := range list {
		for _, agentID := range []int64{item.FromAgentID, item.ToAgentID} {
			if agentID <= 0 {
				continue
			}
			if _, ok := seen[agentID]; ok {
				continue
			}
			seen[agentID] = struct{}{}
			agentIDs = append(agentIDs, agentID)
		}
	}

	// agentID -> 客服名称，账号名称取不到时留空，前端直接显示客服id
	names := make(map[int64]string, len(agentIDs))
	agents, err := s.agentRepository.ListByIDs(agentIDs)
	if err != nil {
		log.Logger.Warn("list kefu transfer agents failed", zap.Error(err))
	}
	userIDs := make([]int64, 0, len(agents))
	for _, agent := range agents {
		userIDs = append(userIDs, agent.UserID)
	}
	users, err := s.agentRepository.ListUsersByIDs(userIDs)
	if err != nil {
		log.Logger.Warn("list kefu transfer agent users failed", zap.Error(err))
	}
	userNames := make(map[int64]string, len(users))
	for _, user := range users {
		name := strings.TrimSpace(user.Nickname)
		if name == "" {
			name = user.Username
		}
		userNames[user.UserID] = name
	}
	for _, agent := range agents {
		names[agent.AgentID] = userNames[agent.UserID]
	}

	for i := range list {
		list[i].FromAgentName = names[list[i].FromAgentID]
		list[i].ToAgentName = names[list[i].ToAgentID]
	}
}

// Read 清空会话未读，客服打开会话时调用
func (s *KefuSessionService) Read(userID int64, sessionID int64) error {
	scopeAgentID, err := s.scopeAgentID(userID)
	if err != nil {
		return err
	}
	if _, err := s.getAccessible(scopeAgentID, sessionID); err != nil {
		return err
	}
	return s.sessionRepository.ResetUnread(sessionID)
}

// Close 结束会话，客服只能结束自己名下的会话，主管可以结束任意会话
func (s *KefuSessionService) Close(userID int64, form model.KefuSessionCloseForm) error {
	if len(form.SessionIDs) == 0 {
		return ErrSessionIDsRequired
	}
	supervisor, err := s.isSupervisor(userID)
	if err != nil {
		return err
	}
	agent, err := s.agentRepository.GetByUserID(userID)
	if err != nil && !errors.Is(err, repository.ErrAgentNotFound) {
		return err
	}
	if !supervisor && agent == nil {
		return ErrNotAgent
	}

	for _, sessionID := range form.SessionIDs {
		session, err := s.sessionRepository.GetByID(sessionID)
		if err != nil {
			return err
		}
		if !supervisor && session.AgentID != agent.AgentID {
			return ErrSessionNoPermission
		}
		if session.Status == model.KefuSessionStatusClosed {
			return ErrSessionStatusInvalid
		}
	}

	reason := strings.TrimSpace(form.CloseReason)
	if reason == "" {
		reason = "客服结束会话"
	}
	if err := s.sessionRepository.Close(form.SessionIDs, reason); err != nil {
		return err
	}

	// 补一条结束提示，网页会话走 IM 通知访客，邮件会话只写本地系统消息
	for _, sessionID := range form.SessionIDs {
		session, err := s.sessionRepository.GetByID(sessionID)
		if err != nil {
			continue
		}
		if s.isMailSession(session) {
			s.closeMailSession(session)
			continue
		}
		if err := s.assignService.sendSystemMessage(
			context.Background(), sessionID, closedNoticeText); err != nil {
			log.Logger.Warn("send kefu closed system message failed",
				zap.Int64("sessionId", sessionID), zap.Error(err))
		}
	}
	return nil
}

// SendMessage 客服发送消息：先落库保证顺序与幂等，再发 IM 并回填消息id；
// IM 发送失败时撤销已落库的消息，避免前端看到"假成功"的消息
func (s *KefuSessionService) SendMessage(
	ctx context.Context, userID int64, sessionID int64, form model.KefuMessageForm,
) (*model.KefuMessage, error) {
	agent, err := s.agentRepository.GetByUserID(userID)
	if err != nil {
		if errors.Is(err, repository.ErrAgentNotFound) {
			return nil, ErrNotAgent
		}
		return nil, err
	}
	session, err := s.sessionRepository.GetByID(sessionID)
	if err != nil {
		return nil, err
	}
	if session.AgentID != agent.AgentID {
		return nil, ErrSessionNoPermission
	}
	if session.Status != model.KefuSessionStatusHuman {
		return nil, ErrSessionStatusInvalid
	}

	content := strings.TrimSpace(form.Content)
	if content == "" {
		return nil, ErrMessageContentRequired
	}
	// 邮件渠道走 SMTP 发信，不走 IM；公共校验与落库顺序保持一致
	if channel, err := s.channelRepository.GetByCode(session.ChannelCode); err == nil &&
		channel.ChannelType == model.KefuChannelTypeEmail {
		return s.SendMailMessage(ctx, agent, session, form)
	}
	contentType := strings.TrimSpace(form.ContentType)
	if contentType == "" {
		contentType = model.KefuMessageTypeText
	}
	if !isAgentContentType(contentType) {
		return nil, ErrMessageContentTypeInvalid
	}

	now := model.Now()
	uid := agentUID(userID)
	// 客服采纳右键 AI 生成的草稿时打上标记, 便于统计 AI 采纳率
	aiGenerated := 0
	if form.AiGenerated == 1 {
		aiGenerated = 1
	}

	// 引用回复: 冗余存一份原消息摘要, 原消息被删后仍能展示
	var quote *model.KefuMessage
	if form.QuoteMessageID > 0 {
		quote, err = s.messageRepository.GetByID(form.QuoteMessageID)
		if err != nil {
			return nil, err
		}
		if quote.SessionID != sessionID {
			return nil, ErrMessageQuoteInvalid
		}
	}

	message := &model.KefuMessage{
		SessionID:      sessionID,
		ClientMsgNo:    generateClientMsgNo("agent"),
		FromRole:       model.KefuMessageFromAgent,
		FromID:         uid,
		ContentType:    contentType,
		Content:        content,
		AiGenerated:    aiGenerated,
		QuoteMessageID: form.QuoteMessageID,
		CreateTime:     now,
		UpdateTime:     now,
	}
	if quote != nil {
		message.QuoteContent = quoteSummary(quote.Content)
		message.QuoteRole = quote.FromRole
	}
	if err := s.messageRepository.Create(message); err != nil {
		return nil, err
	}

	payload := &im.Payload{
		Version:   1,
		Type:      contentType,
		Role:      im.PayloadRoleAgent,
		Content:   content,
		MessageID: message.MessageID,
		SessionID: sessionID,
	}
	// 附件消息的 fileId / name / size / contentType 存在 payload.meta 里，
	// 随 IM 下发给客户，同时作为原始 payload 落库供历史消息回显
	if meta := messageMeta(contentType, form.Payload); meta != nil {
		payload.Meta = meta
	}
	// 引用信息随 IM 下发给客户端, 访客端据此在气泡上方展示引用条
	if quote != nil {
		if payload.Meta == nil {
			payload.Meta = make(map[string]any, 1)
		}
		payload.Meta["quote"] = map[string]any{
			"messageId": quote.MessageID,
			"content":   message.QuoteContent,
			"role":      quote.FromRole,
		}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	message.Payload = string(raw)
	if err := s.messageRepository.UpdateContent(message.MessageID,
		map[string]any{"payload": message.Payload}); err != nil {
		return nil, err
	}

	resp, err := im.Default.SendMessage(ctx, &im.MessageSendReq{
		Header:      im.MessageHeader{RedDot: 1},
		ClientMsgNo: message.ClientMsgNo,
		FromUID:     uid,
		ChannelID:   im.ChannelIDOfSession(sessionID),
		ChannelType: im.ChannelTypeGroup,
		Payload:     raw,
	})
	if err != nil {
		if removeErr := s.messageRepository.Remove(message.MessageID); removeErr != nil {
			log.Logger.Error("remove kefu message after im send failed",
				zap.Int64("messageId", message.MessageID), zap.Error(removeErr))
		}
		return nil, err
	}

	if resp != nil {
		if err := s.messageRepository.UpdateIMInfo(message.MessageID,
			fmt.Sprintf("%d", resp.MessageID), int64(resp.MessageSeq)); err != nil {
			log.Logger.Warn("update kefu message im info failed",
				zap.Int64("messageId", message.MessageID), zap.Error(err))
		}
		message.IMMessageID = fmt.Sprintf("%d", resp.MessageID)
		message.IMMessageSeq = int64(resp.MessageSeq)
	}
	if err := s.sessionRepository.UpdateMessageStat(sessionID, now, 0); err != nil {
		log.Logger.Warn("update kefu session stat failed",
			zap.Int64("sessionId", sessionID), zap.Error(err))
	}
	return message, nil
}

// SendMailMessage 邮件会话的客服回复：落消息后走 SMTP 发信，失败时撤销消息
func (s *KefuSessionService) SendMailMessage(
	ctx context.Context, agent *model.KefuAgent, session *model.KefuSession,
	form model.KefuMessageForm,
) (*model.KefuMessage, error) {
	message, err := s.mailSendService.SendAgentMessage(ctx, agent, session, form)
	if err != nil {
		return nil, err
	}
	// 客服接管后不再自动发送，取消已排队的延时邮件
	if err := s.replyRepository.CancelScheduledBySession(session.SessionID); err != nil {
		log.Logger.Warn("cancel kefu scheduled mail failed",
			zap.Int64("sessionId", session.SessionID), zap.Error(err))
	}
	return message, nil
}

// Assign 领取待接待的会话，普通客服只能领到自己名下，主管可以指定接待客服
func (s *KefuSessionService) Assign(
	ctx context.Context, userID int64, form model.KefuSessionAssignForm,
) error {
	self, err := s.agentRepository.GetByUserID(userID)
	if err != nil {
		if errors.Is(err, repository.ErrAgentNotFound) {
			return ErrNotAgent
		}
		return err
	}
	session, err := s.sessionRepository.GetByID(form.SessionID)
	if err != nil {
		return err
	}
	// 只有待接待与 AI 进行中的会话可以接待，其余状态不允许改接待客服
	if session.Status != model.KefuSessionStatusQueueing &&
		session.Status != model.KefuSessionStatusAi {
		return ErrSessionStatusInvalid
	}

	target := self
	if form.AgentID > 0 && form.AgentID != self.AgentID {
		supervisor, err := s.isSupervisor(userID)
		if err != nil {
			return err
		}
		if !supervisor {
			return ErrAssignAgentForbidden
		}
		target, err = s.agentRepository.GetByID(form.AgentID)
		if err != nil {
			return err
		}
	}
	if target.Status != model.KefuAgentStatusNormal {
		return ErrAgentDisabled
	}

	// 条件更新保证并发领取只有一个客服成功
	ok, err := s.sessionRepository.BindAgent(session.SessionID, target.AgentID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrSessionAlreadyAssigned
	}

	// 接待后不再有 AI 回复，中断进行中的生成并取消待发送邮件
	s.interruptAi(ctx, session.SessionID)
	if err := s.replyRepository.CancelScheduledBySession(session.SessionID); err != nil {
		log.Logger.Warn("cancel kefu scheduled mail failed",
			zap.Int64("sessionId", session.SessionID), zap.Error(err))
	}

	if err := im.Default.AddSubscribers(ctx, im.ChannelIDOfSession(session.SessionID),
		im.ChannelTypeGroup, []string{agentUID(target.UserID)}); err != nil {
		// IM 加成员失败不影响接待关系，重连时会重新对齐频道成员
		log.Logger.Warn("add kefu session subscriber failed",
			zap.Int64("sessionId", session.SessionID), zap.Error(err))
	}

	// 接管后给客户一条系统提示，避免客户不知道已经换成人工客服
	if err := s.assignService.sendSystemMessage(ctx, session.SessionID, assignNoticeText); err != nil {
		log.Logger.Warn("send kefu assign system message failed",
			zap.Int64("sessionId", session.SessionID), zap.Error(err))
	}
	return nil
}

// subscribeAgent 把当前客服加入会话的 IM 群频道：
// AI 进行中与待接待的会话没有客服成员，客服查看时需要订阅才能实时收到新消息
func (s *KefuSessionService) subscribeAgent(userID int64, sessionID int64) {
	agent, err := s.agentRepository.GetByUserID(userID)
	if err != nil {
		// 主管可能没有客服档案，没有 IM 账号可订阅
		if !errors.Is(err, repository.ErrAgentNotFound) {
			log.Logger.Warn("load kefu agent for subscribe failed",
				zap.Int64("userId", userID), zap.Error(err))
		}
		return
	}
	if err := im.Default.AddSubscribers(context.Background(),
		im.ChannelIDOfSession(sessionID), im.ChannelTypeGroup,
		[]string{agentUID(agent.UserID)}); err != nil {
		log.Logger.Warn("subscribe kefu session failed",
			zap.Int64("sessionId", sessionID), zap.Error(err))
	}
}

// interruptAi 给会话进行中的 AI 回复置中断标记，客服接待时停止 AI 继续输出
func (s *KefuSessionService) interruptAi(ctx context.Context, sessionID int64) {
	reply, err := s.replyRepository.GetStreamingBySession(sessionID)
	if err != nil {
		if !errors.Is(err, repository.ErrAiReplyNotFound) {
			log.Logger.Warn("load kefu streaming ai reply failed",
				zap.Int64("sessionId", sessionID), zap.Error(err))
		}
		return
	}
	if err := im.MarkInterrupt(ctx, reply.ReplyID, aiInterruptByAgent); err != nil {
		log.Logger.Warn("interrupt kefu ai reply failed",
			zap.Int64("sessionId", sessionID), zap.Error(err))
	}
}

// Operable 查询会话并校验当前用户能否操作：
// 主管可以操作全部会话，普通客服只能操作自己接待的会话
func (s *KefuSessionService) Operable(userID int64, sessionID int64) (*model.KefuSession, error) {
	scopeAgentID, err := s.scopeAgentID(userID)
	if err != nil {
		return nil, err
	}
	session, err := s.sessionRepository.GetByID(sessionID)
	if err != nil {
		return nil, err
	}
	if scopeAgentID == 0 || session.AgentID == scopeAgentID {
		return session, nil
	}
	return nil, ErrSessionNoPermission
}

// ReadableSession 查询登录用户可查看的会话，不做任何写操作；
// 对话库从会话记录导入时复用会话的可见范围判断
func (s *KefuSessionService) ReadableSession(
	userID int64, sessionID int64,
) (*model.KefuSession, error) {
	scopeAgentID, err := s.scopeAgentID(userID)
	if err != nil {
		return nil, err
	}
	return s.getAccessible(scopeAgentID, sessionID)
}

// Summary 查询客服工作台的顶部统计
func (s *KefuSessionService) Summary(userID int64) (*model.KefuWorkbenchSummary, error) {
	agent, err := s.agentOf(userID)
	if err != nil {
		return nil, err
	}

	// 页签计数与列表查询使用同一套可见范围，保证页签数字与点进去的列表总数一致
	scopeAgentID, err := s.scopeAgentID(userID)
	if err != nil {
		return nil, err
	}
	queueing := model.KefuSessionStatusQueueing
	aiStatus := model.KefuSessionStatusAi
	humanStatus := model.KefuSessionStatusHuman
	closedStatus := model.KefuSessionStatusClosed

	summary := &model.KefuWorkbenchSummary{}
	if summary.QueueCount, err = s.sessionRepository.CountVisible(&queueing, scopeAgentID); err != nil {
		return nil, err
	}
	if summary.AiCount, err = s.sessionRepository.CountVisible(&aiStatus, scopeAgentID); err != nil {
		return nil, err
	}
	if summary.HumanCount, err = s.sessionRepository.CountVisible(&humanStatus, scopeAgentID); err != nil {
		return nil, err
	}
	if summary.ClosedCount, err = s.sessionRepository.CountVisible(&closedStatus, scopeAgentID); err != nil {
		return nil, err
	}
	if summary.AllCount, err = s.sessionRepository.CountVisible(nil, scopeAgentID); err != nil {
		return nil, err
	}
	// 未读与待接管只统计自己名下的会话，用于左侧横幅与浏览器标题角标
	if summary.UnreadCount, err = s.sessionRepository.SumUnread(agent.AgentID); err != nil {
		return nil, err
	}
	if summary.AiFailedCount, err = s.sessionRepository.CountAiFailed(agent.AgentID); err != nil {
		return nil, err
	}

	// 状态条与待接待横幅只与当前客服本人相关
	summary.WorkStatus = agent.WorkStatus
	summary.RestReason = agent.RestReason
	summary.MaxConcurrent = maxConcurrentOf(agent)
	loads, err := s.sessionRepository.CountActiveByAgents([]int64{agent.AgentID})
	if err != nil {
		log.Logger.Warn("count kefu agent sessions failed", zap.Error(err))
	}
	summary.MySessionCount = loads[agent.AgentID]

	// 横幅要展示最长等待与「接待最早」，只有存在待接待时才有值
	if summary.QueueCount > 0 {
		earliest, err := s.sessionRepository.EarliestQueue()
		if err != nil {
			if !errors.Is(err, repository.ErrSessionNotFound) {
				log.Logger.Warn("load kefu earliest queue failed", zap.Error(err))
			}
		} else {
			summary.EarliestQueueID = earliest.SessionID
			if !earliest.QueueTime.IsZero() {
				if wait := int64(time.Since(earliest.QueueTime.Time).Seconds()); wait > 0 {
					summary.QueueMaxWaitSeconds = wait
				}
			}
		}
	}
	return summary, nil
}

// ImCredential 返回客服连接 IM 的凭据，token 缓存在 redis 里避免多标签页互相顶掉
func (s *KefuSessionService) ImCredential(ctx context.Context, userID int64) (*model.KefuImCredential, error) {
	var agentID int64
	if agent, err := s.agentOf(userID); err != nil {
		if !errors.Is(err, ErrNotAgent) {
			return nil, err
		}
		// 主管可能没有客服档案，但仍然需要进入工作台
		supervisor, err := s.isSupervisor(userID)
		if err != nil {
			return nil, err
		}
		if !supervisor {
			return nil, ErrNotAgent
		}
	} else {
		agentID = agent.AgentID
	}

	uid := agentUID(userID)
	token, err := s.agentImToken(ctx, uid)
	if err != nil {
		return nil, err
	}
	return &model.KefuImCredential{
		UID: uid, Token: token, WsURL: im.WsURL(), AgentID: agentID,
	}, nil
}

// ClientDetail 查询会话详情，访客只能查询自己的会话
func (s *KefuSessionService) ClientDetail(sessionID int64, visitorKey string) (*model.KefuSession, error) {
	session, err := s.clientSession(sessionID, visitorKey)
	if err != nil {
		return nil, err
	}
	list := []model.KefuSession{*session}
	s.fillSessionExtra(list)
	return &list[0], nil
}

// ClientMessages 查询会话消息，访客只能查询自己的会话
func (s *KefuSessionService) ClientMessages(
	sessionID int64, visitorKey string, param model.KefuMessageParam,
) ([]model.KefuMessage, int64, error) {
	if _, err := s.clientSession(sessionID, visitorKey); err != nil {
		return nil, 0, err
	}
	return s.messageRepository.ListBySession(sessionID, param)
}

// ClientRead 清空会话未读，访客打开会话时调用
func (s *KefuSessionService) ClientRead(sessionID int64, visitorKey string) error {
	if _, err := s.clientSession(sessionID, visitorKey); err != nil {
		return err
	}
	return s.sessionRepository.ResetUnread(sessionID)
}

// SetTags 覆盖式设置会话标签，客服只能改自己接待的会话，主管可改任意会话
func (s *KefuSessionService) SetTags(
	userID int64, form model.KefuSessionTagsForm,
) error {
	session, err := s.Operable(userID, form.SessionID)
	if err != nil {
		return err
	}
	return s.tagRepository.Replace(session.SessionID, normalizeSessionTags(form.Tags), userID)
}

// SetRemark 保存会话内部备注，只落库不推送，访客与消息回放都看不到
func (s *KefuSessionService) SetRemark(
	userID int64, form model.KefuSessionRemarkForm,
) error {
	session, err := s.Operable(userID, form.SessionID)
	if err != nil {
		return err
	}
	remark := strings.TrimSpace(form.Remark)
	if runes := []rune(remark); len(runes) > maxSessionRemarkLength {
		remark = string(runes[:maxSessionRemarkLength])
	}
	return s.sessionRepository.UpdateFields(session.SessionID, map[string]any{
		"remark": remark,
	})
}

// normalizeSessionTags 归一化会话标签，去空、去重并限制数量与长度
func normalizeSessionTags(tags []string) []string {
	list := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, item := range tags {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		if runes := []rune(value); len(runes) > maxSessionTagLength {
			value = string(runes[:maxSessionTagLength])
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		list = append(list, value)
		if len(list) >= maxSessionTagCount {
			break
		}
	}
	return list
}

// Rate 访客评价已结束的会话，同一会话只能评价一次
func (s *KefuSessionService) Rate(
	sessionID int64, visitorKey string, form model.KefuSessionRateForm,
) error {
	if !setting.GetBool(configKeyRateEnabled, true) {
		return ErrSessionRateDisabled
	}
	session, err := s.clientSession(sessionID, visitorKey)
	if err != nil {
		return err
	}
	if session.Status != model.KefuSessionStatusClosed {
		return ErrSessionNotClosed
	}
	if session.RateScore > 0 {
		return ErrSessionRated
	}
	if form.Score < 1 || form.Score > 5 {
		return ErrSessionRateScoreInvalid
	}

	remark := strings.TrimSpace(form.Remark)
	if runes := []rune(remark); len(runes) > maxRateRemarkLength {
		remark = string(runes[:maxRateRemarkLength])
	}

	ok, err := s.sessionRepository.RateSession(session.SessionID, form.Score,
		normalizeRateTags(form.Tags), remark, model.Now())
	if err != nil {
		return err
	}
	if !ok {
		// 条件更新没命中，说明并发提交已经被另一次请求写入
		return ErrSessionRated
	}
	return nil
}

// normalizeRateTags 归一化评价标签，去空、去重并限制数量与长度
func normalizeRateTags(tags []string) string {
	list := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, item := range tags {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		if runes := []rune(value); len(runes) > maxRateTagLength {
			value = string(runes[:maxRateTagLength])
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		list = append(list, value)
		if len(list) >= maxRateTagCount {
			break
		}
	}
	return strings.Join(list, ",")
}

// ClientSession 查询会话并校验属于该访客，访客端的写操作（转人工、留言、评价）共用这套校验
func (s *KefuSessionService) ClientSession(
	sessionID int64, visitorKey string,
) (*model.KefuSession, error) {
	return s.clientSession(sessionID, visitorKey)
}

// SendTyping 广播一条「正在输入」提示：不落库、不计未读，只给在线端；
// 客户端据节流后的上报调用，失败不影响正常聊天
func (s *KefuSessionService) SendTyping(
	ctx context.Context, sessionID int64, role, fromUID string,
) error {
	raw, err := json.Marshal(&im.Payload{
		Version:   1,
		Type:      im.PayloadTypeTyping,
		Role:      role,
		SessionID: sessionID,
	})
	if err != nil {
		return err
	}
	_, err = im.Default.SendMessage(ctx, &im.MessageSendReq{
		Header:      im.MessageHeader{NoPersist: 1, SyncOnce: 1},
		ClientMsgNo: generateClientMsgNo("typing"),
		FromUID:     fromUID,
		ChannelID:   im.ChannelIDOfSession(sessionID),
		ChannelType: im.ChannelTypeGroup,
		Payload:     raw,
	})
	return err
}

// ClientTyping 访客正在输入，聊天窗节流后上报
func (s *KefuSessionService) ClientTyping(
	ctx context.Context, sessionID int64, visitorKey string,
) error {
	session, err := s.clientSession(sessionID, visitorKey)
	if err != nil {
		return err
	}
	return s.SendTyping(ctx, sessionID, im.PayloadRoleVisitor, visitorUID(session.VisitorID))
}

// AgentTyping 客服正在输入，只允许当前接待的客服上报
func (s *KefuSessionService) AgentTyping(
	ctx context.Context, userID, sessionID int64,
) error {
	session, err := s.Operable(userID, sessionID)
	if err != nil {
		return err
	}
	if session.Status != model.KefuSessionStatusHuman {
		return ErrSessionStatusInvalid
	}
	return s.SendTyping(ctx, sessionID, im.PayloadRoleAgent, agentUID(userID))
}

// clientSession 校验会话属于该访客
func (s *KefuSessionService) clientSession(sessionID int64, visitorKey string) (*model.KefuSession, error) {
	visitor, err := s.visitorRepository.GetByKey(strings.TrimSpace(visitorKey))
	if err != nil {
		return nil, err
	}
	session, err := s.sessionRepository.GetByID(sessionID)
	if err != nil {
		return nil, err
	}
	if session.VisitorID != visitor.VisitorID {
		return nil, ErrSessionNoPermission
	}
	return session, nil
}

// scopeAgentID 返回会话的查询范围：主管返回 0 表示看全部，客服返回自己的客服id
func (s *KefuSessionService) scopeAgentID(userID int64) (int64, error) {
	supervisor, err := s.isSupervisor(userID)
	if err != nil {
		return 0, err
	}
	if supervisor {
		return 0, nil
	}
	agent, err := s.agentRepository.GetByUserID(userID)
	if err != nil {
		if errors.Is(err, repository.ErrAgentNotFound) {
			return 0, ErrNotAgent
		}
		return 0, err
	}
	return agent.AgentID, nil
}

// isSupervisor 判断登录用户是否客服主管或超级管理员
func (s *KefuSessionService) isSupervisor(userID int64) (bool, error) {
	return s.agentRepository.HasRoleCode(userID,
		[]string{model.KefuRoleSupervisor, model.RoleCodeAdmin})
}

// agentOf 查询登录用户的客服档案；已经分配客服角色但还没有档案时自动建档，
// 避免角色授了、档案没建导致工作台一直提示「当前账号不是客服」
func (s *KefuSessionService) agentOf(userID int64) (*model.KefuAgent, error) {
	agent, err := s.agentRepository.GetByUserID(userID)
	if err == nil {
		return agent, nil
	}
	if !errors.Is(err, repository.ErrAgentNotFound) {
		return nil, err
	}
	hasRole, err := s.agentRepository.HasRoleCode(userID, []string{model.KefuRoleAgent})
	if err != nil {
		return nil, err
	}
	if !hasRole {
		return nil, ErrNotAgent
	}

	// 自动建档默认开启接单与 AI，工作状态默认在线，管理员可再到客服管理里调整
	now := model.Now()
	record := &model.KefuAgent{
		UserID:        userID,
		MaxConcurrent: defaultAgentMaxConcurrent,
		AcceptStatus:  model.KefuAgentAcceptOn,
		WorkStatus:    model.KefuAgentWorkOnline,
		StatusTime:    now,
		AiDefault:     1,
		Status:        model.KefuAgentStatusNormal,
		CreateTime:    now,
		UpdateTime:    now,
	}
	if err := s.agentRepository.Create(record); err != nil {
		// 并发首次进入时另一个请求已经建好档案，重新读一次即可
		if again, e := s.agentRepository.GetByUserID(userID); e == nil {
			return again, nil
		}
		return nil, err
	}
	log.Logger.Debug("auto create kefu agent profile", zap.Int64("userId", userID))
	return record, nil
}

// getAccessible 查询会话并校验当前用户是否有权访问
func (s *KefuSessionService) getAccessible(scopeAgentID int64, sessionID int64) (*model.KefuSession, error) {
	session, err := s.sessionRepository.GetByID(sessionID)
	if err != nil {
		return nil, err
	}
	if scopeAgentID == 0 || session.AgentID == scopeAgentID {
		return session, nil
	}
	// 待接待与 AI 进行中的会话没有接待客服，所有客服都可以查看，便于领取接待
	if session.AgentID == 0 && (session.Status == model.KefuSessionStatusQueueing ||
		session.Status == model.KefuSessionStatusAi) {
		return session, nil
	}
	return nil, ErrSessionNoPermission
}

// upsertChannel 创建或更新会话的 IM 群频道，成员为访客、当前客服与 AI 服务号
func (s *KefuSessionService) upsertChannel(ctx context.Context, session *model.KefuSession) error {
	return im.Default.UpsertChannel(ctx, im.ChannelIDOfSession(session.SessionID),
		im.ChannelTypeGroup, s.subscribers(session))
}

// subscribers 返回会话群频道的成员列表
func (s *KefuSessionService) subscribers(session *model.KefuSession) []string {
	list := []string{visitorUID(session.VisitorID)}
	if aiUID := im.AiUID(); aiUID != "" {
		list = append(list, aiUID)
	}
	if session.AgentID > 0 {
		agent, err := s.agentRepository.GetByID(session.AgentID)
		if err != nil {
			log.Logger.Warn("load kefu agent for channel subscriber failed",
				zap.Int64("agentId", session.AgentID), zap.Error(err))
			return list
		}
		list = append(list, agentUID(agent.UserID))
	}
	return list
}

// channelRouteID 返回渠道所属关联链的 id，未绑定或查询失败时返回 0（不报错）
func (s *KefuSessionService) channelRouteID(channelID int64) int64 {
	route, err := s.routeService.ByChannelID(channelID)
	if err != nil {
		log.Logger.Debug("load kefu route of channel failed",
			zap.Int64("channelId", channelID), zap.Error(err))
		return 0
	}
	if route == nil {
		return 0
	}
	return route.RouteID
}

// fillSessionExtra 批量回填会话的访客与客服信息
func (s *KefuSessionService) fillSessionExtra(list []model.KefuSession) {
	if len(list) == 0 {
		return
	}

	visitorIDs := make([]int64, 0, len(list))
	agentIDs := make([]int64, 0, len(list))
	for _, session := range list {
		visitorIDs = append(visitorIDs, session.VisitorID)
		if session.AgentID > 0 {
			agentIDs = append(agentIDs, session.AgentID)
		}
	}

	visitorMap := make(map[int64]model.KefuVisitor, len(visitorIDs))
	visitors, err := s.visitorRepository.ListByIDs(visitorIDs)
	if err != nil {
		log.Logger.Warn("list kefu visitors failed", zap.Error(err))
	}
	for _, visitor := range visitors {
		visitorMap[visitor.VisitorID] = visitor
	}

	agentMap := make(map[int64]model.KefuAgent, len(agentIDs))
	agents, err := s.agentRepository.ListByIDs(agentIDs)
	if err != nil {
		log.Logger.Warn("list kefu agents failed", zap.Error(err))
	}
	userIDs := make([]int64, 0, len(agents))
	for _, agent := range agents {
		agentMap[agent.AgentID] = agent
		userIDs = append(userIDs, agent.UserID)
	}
	users, err := s.agentRepository.ListUsersByIDs(userIDs)
	if err != nil {
		log.Logger.Warn("list kefu agent users failed", zap.Error(err))
	}
	userMap := make(map[int64]model.SysUser, len(users))
	for _, user := range users {
		userMap[user.UserID] = user
	}

	for i := range list {
		if visitor, ok := visitorMap[list[i].VisitorID]; ok {
			list[i].Visitor = &visitor
		}
		agent, ok := agentMap[list[i].AgentID]
		if !ok {
			continue
		}
		if user, ok := userMap[agent.UserID]; ok {
			agent.Username = user.Username
			agent.Nickname = user.Nickname
		}
		list[i].Agent = &agent
	}
	s.fillRouteNames(list)
	s.fillMailInfo(list)
	s.fillVisitorOnline(list, visitorMap)
	s.fillQueueExtra(list)
	s.fillLastMessage(list)
	s.fillTags(list)
}

// fillMailInfo 批量回填渠道类型与邮件会话的主题、发件人、收件人
func (s *KefuSessionService) fillMailInfo(list []model.KefuSession) {
	if len(list) == 0 {
		return
	}

	channelMap := make(map[string]model.KefuChannel)
	for i := range list {
		code := list[i].ChannelCode
		if code == "" {
			continue
		}
		if _, ok := channelMap[code]; ok {
			continue
		}
		channel, err := s.channelRepository.GetByCode(code)
		if err != nil {
			continue
		}
		channelMap[code] = *channel
	}

	sessionIDs := make([]int64, 0, len(list))
	channelIDs := make([]int64, 0, len(list))
	for i := range list {
		channel, ok := channelMap[list[i].ChannelCode]
		if !ok {
			continue
		}
		list[i].ChannelType = channel.ChannelType
		if channel.ChannelType == model.KefuChannelTypeEmail {
			sessionIDs = append(sessionIDs, list[i].SessionID)
			channelIDs = append(channelIDs, channel.ChannelID)
		}
	}
	if len(sessionIDs) == 0 {
		return
	}

	threads, err := s.threadRepository.ListBySessionIDs(sessionIDs)
	if err != nil {
		log.Logger.Warn("list kefu mail threads failed", zap.Error(err))
		return
	}
	threadMap := make(map[int64]model.KefuMailThread, len(threads))
	for _, thread := range threads {
		threadMap[thread.SessionID] = thread
	}

	mailboxes, err := s.mailboxRepository.ListByChannelIDs(channelIDs)
	if err != nil {
		log.Logger.Warn("list kefu mailboxes failed", zap.Error(err))
	}
	mailboxMap := make(map[int64]model.KefuMailbox, len(mailboxes))
	for _, mailbox := range mailboxes {
		mailboxMap[mailbox.ChannelID] = mailbox
	}
	// 历史线程数按访客缓存，同一访客多个会话只查一次
	threadCountMap := make(map[int64]int64)
	for i := range list {
		thread, ok := threadMap[list[i].SessionID]
		if !ok {
			continue
		}
		list[i].MailSubject = thread.Subject
		if list[i].Visitor != nil {
			list[i].MailFrom = list[i].Visitor.Email
		}
		if count, ok := threadCountMap[list[i].VisitorID]; ok {
			list[i].MailThreadCount = count
		} else if count, err := s.threadRepository.CountByVisitor(list[i].VisitorID); err == nil {
			threadCountMap[list[i].VisitorID] = count
			list[i].MailThreadCount = count
		}
		if channel, ok := channelMap[list[i].ChannelCode]; ok {
			if mailbox, ok := mailboxMap[channel.ChannelID]; ok {
				list[i].MailTo = mailbox.EmailAddress
				list[i].MailLastSyncTime = mailbox.LastSyncTime
			}
		}
	}
}

// fillLastMessage 批量回填会话的最后一条消息摘要，图片/附件只显示占位文案
func (s *KefuSessionService) fillLastMessage(list []model.KefuSession) {
	sessionIDs := make([]int64, 0, len(list))
	for _, item := range list {
		sessionIDs = append(sessionIDs, item.SessionID)
	}
	messages, err := s.messageRepository.LastBySessions(sessionIDs)
	if err != nil {
		log.Logger.Warn("load kefu last messages failed", zap.Error(err))
		return
	}
	for i := range list {
		message, ok := messages[list[i].SessionID]
		if !ok {
			continue
		}
		list[i].LastMessage = lastMessageSummary(&message)
	}
}

// fillTags 批量回填会话标签
func (s *KefuSessionService) fillTags(list []model.KefuSession) {
	sessionIDs := make([]int64, 0, len(list))
	for _, item := range list {
		sessionIDs = append(sessionIDs, item.SessionID)
	}
	tags, err := s.tagRepository.ListBySessions(sessionIDs)
	if err != nil {
		log.Logger.Warn("load kefu session tags failed", zap.Error(err))
		return
	}
	for i := range list {
		list[i].Tags = tags[list[i].SessionID]
	}
}

// quoteSummary 截取引用消息的摘要，过长时省略，避免消息体过大
func quoteSummary(content string) string {
	value := strings.TrimSpace(content)
	value = strings.Join(strings.Fields(value), " ")
	if runes := []rune(value); len(runes) > 200 {
		return string(runes[:200]) + "..."
	}
	return value
}

// lastMessageSummary 把最后一条消息压成一行摘要
func lastMessageSummary(message *model.KefuMessage) string {
	switch message.ContentType {
	case model.KefuMessageTypeImage:
		return "[图片]"
	case model.KefuMessageTypeFile:
		return "[附件]"
	}
	content := strings.TrimSpace(message.Content)
	if content == "" {
		return ""
	}
	// 只保留一行，过长时截断，避免列表项被撑高
	content = strings.Join(strings.Fields(content), " ")
	if runes := []rune(content); len(runes) > 60 {
		return string(runes[:60]) + "..."
	}
	return content
}

// fillRouteNames 批量回填会话快照的关联链名称，查不到时留空
func (s *KefuSessionService) fillRouteNames(list []model.KefuSession) {
	if len(list) == 0 {
		return
	}

	routeIDs := make([]int64, 0, len(list))
	for _, session := range list {
		if session.RouteID > 0 {
			routeIDs = append(routeIDs, session.RouteID)
		}
	}
	if len(routeIDs) == 0 {
		return
	}

	names, err := s.routeService.NamesByIDs(routeIDs)
	if err != nil {
		log.Logger.Warn("list kefu route names failed", zap.Error(err))
		return
	}
	for i := range list {
		list[i].RouteName = names[list[i].RouteID]
	}
}

// fillVisitorOnline 批量回填访客的在线状态，以 IM 在线路由为准；
// 查询失败只记日志，在线状态不是列表的必需字段
func (s *KefuSessionService) fillVisitorOnline(
	list []model.KefuSession, visitorMap map[int64]model.KefuVisitor,
) {
	if im.Default == nil || len(visitorMap) == 0 {
		return
	}
	uids := make([]string, 0, len(visitorMap))
	for _, visitor := range visitorMap {
		uids = append(uids, visitorUID(visitor.VisitorID))
	}
	status, err := im.Default.OnlineStatus(context.Background(), uids)
	if err != nil {
		log.Logger.Warn("query kefu visitor online status failed", zap.Error(err))
		return
	}
	for i := range list {
		if list[i].ChannelType == model.KefuChannelTypeEmail {
			list[i].VisitorOnline = false
			continue
		}
		list[i].VisitorOnline = status[visitorUID(list[i].VisitorID)]
	}
}

// fillQueueExtra 批量回填待接待会话的排队名次、队列长度、预计等待与已等待时长；
// 名次按 queue_time 升序在全部待接待会话中实时计算，只做展示，不参与分配
func (s *KefuSessionService) fillQueueExtra(list []model.KefuSession) {
	queueing := make([]int, 0, len(list))
	for i := range list {
		// 邮件会话没有在线状态与排队语义，不展示位次与预计等待
		if list[i].ChannelType == model.KefuChannelTypeEmail {
			continue
		}
		if list[i].Status == model.KefuSessionStatusQueueing {
			queueing = append(queueing, i)
		}
	}
	if len(queueing) == 0 {
		return
	}

	order, err := s.sessionRepository.ListQueueOrder()
	if err != nil {
		log.Logger.Warn("list kefu queue order failed", zap.Error(err))
		return
	}
	positions := make(map[int64]int, len(order))
	for i, sessionID := range order {
		positions[sessionID] = i + 1
	}

	// 预计等待按名次乘人平等待时长估算，仅用于访客端提示
	avgWait := setting.GetInt(configKeyAssignAvgWaitSeconds, defaultAssignAvgWaitSeconds)
	if avgWait <= 0 {
		avgWait = defaultAssignAvgWaitSeconds
	}
	now := time.Now()
	for _, index := range queueing {
		position := positions[list[index].SessionID]
		if position <= 0 {
			position = 1
		}
		list[index].QueuePosition = position
		list[index].QueueSize = int64(len(order))
		list[index].EstimatedWaitSeconds = int64(position) * int64(avgWait)
		if list[index].QueueTime.IsZero() {
			continue
		}
		if wait := int64(now.Sub(list[index].QueueTime.Time).Seconds()); wait > 0 {
			list[index].QueueWaitSeconds = wait
		}
	}
}

// agentImToken 读取或创建客服的 IM token，缓存避免多标签页互相顶掉
func (s *KefuSessionService) agentImToken(ctx context.Context, uid string) (string, error) {
	key := agentTokenKeyPrefix + uid
	if rdb.Client != nil {
		token, err := rdb.Client.Get(ctx, key).Result()
		if err == nil && token != "" {
			return token, nil
		}
	}

	token, err := randomToken()
	if err != nil {
		return "", err
	}
	if err := im.Default.EnsureUserToken(ctx, uid, token); err != nil {
		return "", err
	}
	if rdb.Client != nil {
		if err := rdb.Client.Set(ctx, key, token, 0).Err(); err != nil {
			log.Logger.Warn("cache kefu agent im token failed",
				zap.String("uid", uid), zap.Error(err))
		}
	}
	return token, nil
}

// aiModeFromConfig 按 AI 总开关返回新会话的默认 AI 回复开关
func aiModeFromConfig() int {
	if setting.GetBool(configKeyAiEnabled, true) {
		return model.KefuSessionAiModeOn
	}
	return model.KefuSessionAiModeOff
}

// isAgentContentType 判断客服可以发送的消息类型
func isAgentContentType(contentType string) bool {
	switch contentType {
	case model.KefuMessageTypeText, model.KefuMessageTypeImage, model.KefuMessageTypeFile:
		return true
	default:
		return false
	}
}

// messageMeta 解析附件消息的附加信息，文本消息或 json 不合法时返回 nil
func messageMeta(contentType, payload string) map[string]any {
	if contentType == model.KefuMessageTypeText {
		return nil
	}
	value := strings.TrimSpace(payload)
	if value == "" {
		return nil
	}
	meta := make(map[string]any)
	if err := json.Unmarshal([]byte(value), &meta); err != nil {
		log.Logger.Warn("decode kefu message meta failed", zap.Error(err))
		return nil
	}
	if len(meta) == 0 {
		return nil
	}
	return meta
}

// agentUID 返回客服的 IM 用户标识
func agentUID(userID int64) string {
	return fmt.Sprintf("agent_%d", userID)
}

// visitorUID 返回访客的 IM 用户标识
func visitorUID(visitorID int64) string {
	return fmt.Sprintf("visitor_%d", visitorID)
}

// generateSessionNo 生成会话编号，时间前缀加随机数
func generateSessionNo() string {
	suffix, err := rand.Int(rand.Reader, big.NewInt(10000))
	if err != nil {
		return fmt.Sprintf("S%s%04d", time.Now().Format("20060102150405"), time.Now().UnixNano()%10000)
	}
	return fmt.Sprintf("S%s%04d", time.Now().Format("20060102150405"), suffix.Int64())
}

// generateClientMsgNo 生成 IM 侧幂等消息编号
func generateClientMsgNo(prefix string) string {
	return fmt.Sprintf("%s_%d_%s", prefix, time.Now().UnixNano(), randomHex(4))
}

// randomToken 生成 32 位随机 token，用于访客与客服连接 IM
func randomToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// randomHex 生成 n 字节的随机十六进制字符串，随机源不可用时退回时间戳保证可用
func randomHex(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

// isMailSession 判断会话是否属于邮箱渠道
func (s *KefuSessionService) isMailSession(session *model.KefuSession) bool {
	if session == nil || session.ChannelCode == "" {
		return false
	}
	channel, err := s.channelRepository.GetByCode(session.ChannelCode)
	if err != nil {
		return false
	}
	return channel.ChannelType == model.KefuChannelTypeEmail
}

// closeMailSession 结束邮件会话：取消待发送记录、关闭线程并写本地系统消息，不走 IM
func (s *KefuSessionService) closeMailSession(session *model.KefuSession) {
	if err := s.replyRepository.CancelScheduledBySession(session.SessionID); err != nil {
		log.Logger.Warn("cancel kefu scheduled mail failed",
			zap.Int64("sessionId", session.SessionID), zap.Error(err))
	}
	thread, err := s.threadRepository.GetBySessionID(session.SessionID)
	if err == nil {
		if err := s.threadRepository.UpdateFields(thread.ThreadID, map[string]any{
			"status": model.KefuMailThreadStatusClosed,
		}); err != nil {
			log.Logger.Warn("close kefu mail thread failed",
				zap.Int64("threadId", thread.ThreadID), zap.Error(err))
		}
	}
	s.appendLocalSystemMessage(session.SessionID, closedNoticeText)
}

// appendLocalSystemMessage 只写一条本地系统消息，不推送 IM，邮件会话使用
func (s *KefuSessionService) appendLocalSystemMessage(sessionID int64, content string) {
	now := model.Now()
	message := &model.KefuMessage{
		SessionID:   sessionID,
		ClientMsgNo: generateClientMsgNo("local"),
		FromRole:    model.KefuMessageFromSystem,
		FromID:      "system",
		ContentType: model.KefuMessageTypeSystem,
		Content:     content,
		CreateTime:  now,
		UpdateTime:  now,
	}
	if err := s.messageRepository.Create(message); err != nil {
		log.Logger.Warn("create kefu local system message failed",
			zap.Int64("sessionId", sessionID), zap.Error(err))
		return
	}
	if err := s.sessionRepository.UpdateMessageStat(sessionID, now, 0); err != nil {
		log.Logger.Warn("update kefu session stat failed",
			zap.Int64("sessionId", sessionID), zap.Error(err))
	}
}

// EnsureMailSession 按邮件线程取建会话：已有未结束会话直接复用；
// 已结束且在重开窗口内重开原会话；超过窗口或没有会话时新建，
// 不走「一访客一活跃会话」的判断，网页仍走 EnsureSession
func (s *KefuSessionService) EnsureMailSession(
	visitor *model.KefuVisitor, channel *model.KefuChannel, thread *model.KefuMailThread,
) (*model.KefuSession, error) {
	aiMode := aiModeFromConfig()
	now := model.Now()

	if thread.SessionID > 0 {
		session, err := s.sessionRepository.GetByID(thread.SessionID)
		if err == nil {
			if session.Status != model.KefuSessionStatusClosed {
				return session, nil
			}
			reopenDays := setting.GetInt(configKeyMailReopenDays, defaultMailReopenDays)
			if reopenDays > 0 && !session.EndTime.IsZero() &&
				time.Since(session.EndTime.Time) <= time.Duration(reopenDays)*24*time.Hour {
				status := model.KefuSessionStatusAi
				if aiMode != model.KefuSessionAiModeOn {
					status = model.KefuSessionStatusQueueing
				}
				fields := map[string]any{
					"status":            status,
					"ai_mode":           aiMode,
					"end_time":          nil,
					"close_reason":      "",
					"last_message_time": now,
					"update_time":       now,
				}
				if status == model.KefuSessionStatusQueueing {
					fields["queue_time"] = now
				}
				if err := s.sessionRepository.UpdateFields(session.SessionID, fields); err != nil {
					return nil, err
				}
				session.Status = status
				session.AiMode = aiMode
				session.EndTime = model.Time{}
				session.CloseReason = ""
				s.appendLocalSystemMessage(session.SessionID, "客户再次来信，会话已恢复")
				if err := s.threadRepository.UpdateFields(thread.ThreadID, map[string]any{
					"status": model.KefuMailThreadStatusActive,
				}); err != nil {
					log.Logger.Warn("reopen kefu mail thread failed",
						zap.Int64("threadId", thread.ThreadID), zap.Error(err))
				}
				return session, nil
			}
		}
	}

	status := model.KefuSessionStatusAi
	if aiMode != model.KefuSessionAiModeOn {
		status = model.KefuSessionStatusQueueing
	}
	session := &model.KefuSession{
		SessionNo:       generateSessionNo(),
		ChannelCode:     channel.ChannelCode,
		VisitorID:       visitor.VisitorID,
		RouteID:         s.channelRouteID(channel.ChannelID),
		Status:          status,
		AiMode:          aiMode,
		QueueTime:       now,
		LastMessageTime: now,
		CreateTime:      now,
		UpdateTime:      now,
	}
	if err := s.sessionRepository.Create(session); err != nil {
		return nil, err
	}
	if err := s.threadRepository.UpdateFields(thread.ThreadID, map[string]any{
		"session_id": session.SessionID,
		"status":     model.KefuMailThreadStatusActive,
	}); err != nil {
		log.Logger.Warn("bind kefu mail thread session failed",
			zap.Int64("threadId", thread.ThreadID), zap.Error(err))
	}
	return session, nil
}
