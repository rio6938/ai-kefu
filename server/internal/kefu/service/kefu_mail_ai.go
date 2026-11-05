package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"go.uber.org/zap"

	aiclient "admin/server/internal/kefu/ai/client"
	"admin/server/internal/kefu/repository"
	"admin/server/model"
	"admin/server/pkg/log"
	"admin/server/pkg/setting"
)

// 邮件 AI 生成用到的配置项与常量
const (
	// configKeyMailContextEmails 是组装邮件上下文时带上的最近邮件数
	configKeyMailContextEmails = "kefu.mail.context_emails"
	// defaultMailContextEmails 是邮件上下文默认封数
	defaultMailContextEmails = 5
)

// 邮件 AI 相关的业务错误
var (
	// ErrMailDraftNotFound 是当前会话没有待发送的邮件草稿
	ErrMailDraftNotFound = errors.New("当前会话没有可用的邮件草稿")
	// ErrMailDraftContentRequired 是邮件正文不能为空
	ErrMailDraftContentRequired = errors.New("邮件正文不能为空")
	// ErrMailDraftStatusInvalid 是草稿状态不允许该操作
	ErrMailDraftStatusInvalid = errors.New("草稿状态不允许该操作")
	// ErrMailSendForbidden 是只有接待客服可以发送草稿
	ErrMailSendForbidden = errors.New("只有接待客服可以发送该会话的邮件")
)

// KefuMailAiService 是邮件 AI 生成与草稿的业务处理：
// 复用提示词、RAG、关联链、对话库与敏感词，按邮箱的回复模式决定草稿、延时或立即发送
type KefuMailAiService struct {
	// AI 生成层
	aiService *KefuAiService
	// 邮件线程的数据访问
	threadRepository *repository.KefuMailThreadRepository
	// AI 回复记录的数据访问
	replyRepository *repository.KefuAiReplyRepository
	// 会话消息的数据访问
	messageRepository *repository.KefuMessageRepository
	// 会话的数据访问
	sessionRepository *repository.KefuSessionRepository
	// 客服档案的数据访问，取接待客服
	agentRepository *repository.KefuAgentRepository
	// 渠道的数据访问
	channelRepository *repository.KefuChannelRepository
	// 邮箱配置的数据访问
	mailboxRepository *repository.KefuMailboxRepository
	// 访客的数据访问，邮箱被退信后停止自动回复
	visitorRepository *repository.KefuVisitorRepository
	// 邮件出站编排
	sendService *KefuMailSendService
	// 会话业务处理，校验草稿操作权限
	sessionService *KefuSessionService
}

// NewKefuMailAiService 创建邮件 AI 生成与草稿的业务处理
func NewKefuMailAiService() *KefuMailAiService {
	return &KefuMailAiService{
		aiService:         NewKefuAiService(),
		threadRepository:  repository.NewKefuMailThreadRepository(),
		replyRepository:   repository.NewKefuAiReplyRepository(),
		messageRepository: repository.NewKefuMessageRepository(),
		sessionRepository: repository.NewKefuSessionRepository(),
		agentRepository:   repository.NewKefuAgentRepository(),
		channelRepository: repository.NewKefuChannelRepository(),
		mailboxRepository: repository.NewKefuMailboxRepository(),
		visitorRepository: repository.NewKefuVisitorRepository(),
		sendService:       NewKefuMailSendService(),
		sessionService:    NewKefuSessionService(),
	}
}

// GenerateAndDispatch 为最新一封来信生成回复，并按邮箱配置的模式处理
func (s *KefuMailAiService) GenerateAndDispatch(
	ctx context.Context, mailbox *model.KefuMailbox, thread *model.KefuMailThread,
	session *model.KefuSession, query string,
) {
	if !mailEnabled() {
		return
	}
	if mailbox.Status != model.KefuMailboxStatusNormal {
		return
	}
	if mailbox.ReplyMode == model.KefuMailReplyModeOff {
		return
	}
	if session.Status != model.KefuSessionStatusAi ||
		session.AiMode != model.KefuSessionAiModeOn {
		return
	}
	// 发件人邮箱已被退信标记为无效，停止自动回复并提醒人工
	if visitor, err := s.visitorRepository.GetByID(session.VisitorID); err == nil &&
		visitor.MailInvalid == 1 {
		_ = s.sessionRepository.UpdateFields(session.SessionID, map[string]any{
			"ai_failed": 1,
		})
		return
	}

	reply, err := s.generate(ctx, mailbox, thread, session, query)
	if err != nil {
		log.Logger.Warn("generate kefu mail ai reply failed",
			zap.Int64("sessionId", session.SessionID), zap.Error(err))
		return
	}

	language := mailReplyLanguage(mailbox, query)
	replyLanguage := mailLanguageByText(reply.Content)
	// 语言不一致时降级为草稿，避免中文来信收到纯英文回复
	if language != replyLanguage {
		s.forceDraft(session, reply, true)
		return
	}
	// 命中投诉、退款、法律等敏感场景不自动发送
	if mailSensitiveHit(query) {
		s.forceDraft(session, reply, true)
		return
	}
	// 工作时间内人工在线，自动回复统一降级为草稿，非工作时间才允许自动发送
	if mailbox.WorkTimeEnabled == 1 && mailInWorkTime(mailbox, time.Now()) {
		s.forceDraft(session, reply, false)
		return
	}
	// 限流：同线程次数、同发件人每日次数、最小间隔
	if limited, reason := mailReplyLimited(s.replyRepository, thread, session.VisitorID); limited {
		log.Logger.Debug("kefu mail auto reply limited",
			zap.Int64("sessionId", session.SessionID), zap.String("reason", reason))
		s.forceDraft(session, reply, false)
		return
	}

	switch mailbox.ReplyMode {
	case model.KefuMailReplyModeAuto:
		// 没有知识依据时只出草稿，避免凭空作答
		if !hasMailRagHits(reply) {
			s.forceDraft(session, reply, true)
			return
		}
		if _, err := s.sendService.SendDraft(ctx, session, reply,
			reply.Subject, reply.Content, nil, true); err != nil {
			log.Logger.Warn("send kefu mail auto reply failed",
				zap.Int64("sessionId", session.SessionID), zap.Error(err))
		}
	case model.KefuMailReplyModeDelay:
		if !hasMailRagHits(reply) {
			s.forceDraft(session, reply, true)
			return
		}
		delay := mailbox.DelayMinutes
		if delay <= 0 {
			delay = setting.GetInt(configKeyMailDelayMinutes, defaultMailDelayMinutes)
		}
		if delay <= 0 {
			delay = defaultMailDelayMinutes
		}
		scheduled := model.Time{Time: time.Now().Add(time.Duration(delay) * time.Minute)}
		if err := s.replyRepository.UpdateFields(reply.ReplyID, map[string]any{
			"send_status":    model.KefuAiReplySendScheduled,
			"scheduled_time": scheduled,
		}); err != nil {
			log.Logger.Warn("schedule kefu mail reply failed",
				zap.Int64("replyId", reply.ReplyID), zap.Error(err))
		}
	default:
		s.forceDraft(session, reply, false)
	}
}

// generate 组装线程上下文并调用生成层，返回落库后的 AI 回复记录
func (s *KefuMailAiService) generate(
	ctx context.Context, mailbox *model.KefuMailbox, thread *model.KefuMailThread,
	session *model.KefuSession, query string,
) (*model.KefuAiReply, error) {
	history, trigger := s.mailHistory(session.SessionID)
	language := mailReplyLanguage(mailbox, query)
	reply, err := s.aiService.GenerateMailReply(ctx, session, trigger, thread, history, query, language)
	if err != nil {
		return nil, err
	}
	// 超出最大字符数时截断，避免邮件过长
	maxChars := setting.GetInt(configKeyMailReplyMaxChars, defaultMailReplyMaxChars)
	if maxChars > 0 {
		runes := []rune(reply.Content)
		if len(runes) > maxChars {
			reply.Content = strings.TrimSpace(string(runes[:maxChars]))
			_ = s.replyRepository.UpdateFields(reply.ReplyID, map[string]any{
				"content": reply.Content,
			})
		}
	}
	return reply, nil
}

// mailHistory 把会话最近几封邮件组装成对话上下文，只带清洗后的正文，不带内部备注
func (s *KefuMailAiService) mailHistory(
	sessionID int64,
) ([]aiclient.Message, *model.KefuMessage) {
	limit := setting.GetInt(configKeyMailContextEmails, defaultMailContextEmails)
	if limit <= 0 {
		limit = defaultMailContextEmails
	}
	messages, err := s.messageRepository.ListRecent(sessionID, limit*2)
	if err != nil {
		log.Logger.Warn("load kefu mail history failed",
			zap.Int64("sessionId", sessionID), zap.Error(err))
		return nil, nil
	}

	history := make([]aiclient.Message, 0, len(messages))
	var trigger *model.KefuMessage
	for i := range messages {
		item := messages[i]
		content := strings.TrimSpace(item.Content)
		// 系统提示与附件消息不进 AI 上下文
		if content == "" || item.ContentType == model.KefuMessageTypeSystem ||
			item.ContentType == model.KefuMessageTypeFile ||
			item.ContentType == model.KefuMessageTypeImage {
			continue
		}
		switch item.FromRole {
		case model.KefuMessageFromVisitor:
			history = append(history, aiclient.Message{Role: aiclient.RoleUser, Content: content})
			trigger = &messages[i]
		case model.KefuMessageFromAi, model.KefuMessageFromAgent:
			history = append(history, aiclient.Message{Role: aiclient.RoleAssistant, Content: content})
		}
	}
	// 最新的客户来信单独作为本轮提问，避免在上下文里重复出现
	if trigger != nil && len(history) > 0 {
		last := history[len(history)-1]
		if last.Role == aiclient.RoleUser {
			history = history[:len(history)-1]
		}
	}
	return history, trigger
}

// forceDraft 把生成结果降级为草稿，markFailed 为 true 时同时点亮工作台提醒
func (s *KefuMailAiService) forceDraft(
	session *model.KefuSession, reply *model.KefuAiReply, markFailed bool,
) {
	if err := s.replyRepository.UpdateFields(reply.ReplyID, map[string]any{
		"send_status":    model.KefuAiReplySendDraft,
		"scheduled_time": nil,
	}); err != nil {
		log.Logger.Warn("mark kefu mail reply draft failed",
			zap.Int64("replyId", reply.ReplyID), zap.Error(err))
	}
	reply.SendStatus = model.KefuAiReplySendDraft
	if !markFailed {
		return
	}
	if err := s.sessionRepository.UpdateFields(session.SessionID, map[string]any{
		"ai_failed": 1,
	}); err != nil {
		log.Logger.Warn("mark kefu session ai failed",
			zap.Int64("sessionId", session.SessionID), zap.Error(err))
	}
}

// hasMailRagHits 判断 AI 回复是否有知识库命中
func hasMailRagHits(reply *model.KefuAiReply) bool {
	value := strings.TrimSpace(reply.RagHits)
	return value != "" && value != "[]" && value != "null"
}

// GetDraft 查询当前会话的邮件草稿或待发送记录
func (s *KefuMailAiService) GetDraft(
	userID int64, sessionID int64,
) (*model.KefuMailDraftResult, error) {
	if _, err := s.sessionService.Operable(userID, sessionID); err != nil {
		return nil, err
	}
	reply, err := s.replyRepository.GetPendingMailBySession(sessionID)
	if err != nil {
		if errors.Is(err, repository.ErrAiReplyNotFound) {
			return nil, ErrMailDraftNotFound
		}
		return nil, err
	}
	return &model.KefuMailDraftResult{
		ReplyID:       reply.ReplyID,
		SessionID:     reply.SessionID,
		MailThreadID:  reply.MailThreadID,
		Subject:       reply.Subject,
		Content:       reply.Content,
		SendStatus:    reply.SendStatus,
		ScheduledTime: reply.ScheduledTime,
		CreateTime:    reply.CreateTime,
	}, nil
}

// SendDraftByUser 客服采用并发送草稿，可带编辑后的正文与附件
func (s *KefuMailAiService) SendDraftByUser(
	ctx context.Context, userID int64, form model.KefuMailDraftSendForm,
) (*model.KefuMessage, error) {
	session, err := s.sessionService.Operable(userID, form.SessionID)
	if err != nil {
		return nil, err
	}
	agent, err := s.agentRepository.GetByUserID(userID)
	if err != nil {
		return nil, ErrNotAgent
	}
	// 只有接待客服可以代客户回信，主管代发也要先领取
	if session.AgentID != agent.AgentID {
		return nil, ErrMailSendForbidden
	}
	thread, err := s.threadRepository.GetBySessionID(session.SessionID)
	if err != nil {
		return nil, err
	}

	content := strings.TrimSpace(form.Content)
	subject := strings.TrimSpace(form.Subject)

	var reply *model.KefuAiReply
	if form.ReplyID > 0 {
		reply, err = s.replyRepository.GetByID(form.ReplyID)
		if err != nil {
			return nil, err
		}
		if reply.SessionID != session.SessionID {
			return nil, ErrSessionNoPermission
		}
		if reply.SendStatus == model.KefuAiReplySendSent ||
			reply.SendStatus == model.KefuAiReplySendCancelled {
			return nil, ErrMailDraftStatusInvalid
		}
		if content == "" {
			content = strings.TrimSpace(reply.Content)
		}
		if subject == "" {
			subject = reply.Subject
		}
	}
	if content == "" {
		return nil, ErrMailDraftContentRequired
	}
	if subject == "" {
		subject = thread.Subject
	}

	message, err := s.sendService.SendDraft(ctx, session, reply, subject, content,
		form.Attachments, false)
	if err != nil {
		return nil, err
	}
	if err := s.replyRepository.CancelScheduledBySession(session.SessionID); err != nil {
		log.Logger.Warn("cancel kefu scheduled mail failed",
			zap.Int64("sessionId", session.SessionID), zap.Error(err))
	}
	return message, nil
}

// Regenerate 重新生成草稿，旧的待发送记录先取消
func (s *KefuMailAiService) Regenerate(
	ctx context.Context, userID int64, sessionID int64,
) (*model.KefuMailDraftResult, error) {
	session, err := s.sessionService.Operable(userID, sessionID)
	if err != nil {
		return nil, err
	}
	if session.Status != model.KefuSessionStatusAi {
		return nil, ErrAiDisabled
	}
	channel, err := s.channelRepository.GetByCode(session.ChannelCode)
	if err != nil {
		return nil, err
	}
	mailbox, err := s.mailboxRepository.GetByChannelID(channel.ChannelID)
	if err != nil {
		return nil, err
	}
	thread, err := s.threadRepository.GetBySessionID(sessionID)
	if err != nil {
		return nil, err
	}
	if err := s.DiscardBySession(sessionID); err != nil {
		return nil, err
	}

	query := s.latestVisitorText(sessionID)
	if strings.TrimSpace(query) == "" {
		return nil, ErrMailDraftNotFound
	}
	reply, err := s.generate(ctx, mailbox, thread, session, query)
	if err != nil {
		return nil, err
	}
	// 重新生成一律先落草稿，由客服确认后发送
	s.forceDraft(session, reply, false)
	return &model.KefuMailDraftResult{
		ReplyID:      reply.ReplyID,
		SessionID:    reply.SessionID,
		MailThreadID: reply.MailThreadID,
		Subject:      reply.Subject,
		Content:      reply.Content,
		SendStatus:   model.KefuAiReplySendDraft,
		CreateTime:   reply.CreateTime,
	}, nil
}

// Discard 丢弃草稿或取消待发送，返回取消的记录id
func (s *KefuMailAiService) Discard(userID, sessionID int64) error {
	if _, err := s.sessionService.Operable(userID, sessionID); err != nil {
		return err
	}
	return s.DiscardBySession(sessionID)
}

// DiscardBySession 取消会话内所有草稿与待发送记录
func (s *KefuMailAiService) DiscardBySession(sessionID int64) error {
	return s.replyRepository.CancelPendingBySession(sessionID)
}

// latestVisitorText 取会话最近一封客户来信的正文
func (s *KefuMailAiService) latestVisitorText(sessionID int64) string {
	messages, err := s.messageRepository.ListRecent(sessionID, 20)
	if err != nil {
		return ""
	}
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].FromRole == model.KefuMessageFromVisitor {
			return strings.TrimSpace(messages[i].Content)
		}
	}
	return ""
}
