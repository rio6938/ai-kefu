package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"admin/server/internal/kefu/mail"
	"admin/server/internal/kefu/repository"
	systemservice "admin/server/internal/system/service"
	"admin/server/model"
	"admin/server/pkg/log"
	"admin/server/pkg/setting"
)

// 自动回复与后台维护任务的常量
const (
	// mailAutoScanInterval 是延时发送 worker 的扫描间隔
	mailAutoScanInterval = 30 * time.Second
	// mailMaintainInterval 是会话结束与原文清理任务的执行间隔
	mailMaintainInterval = 6 * time.Hour
	// mailMaintainDelay 是维护任务首次执行的延迟
	mailMaintainDelay = 10 * time.Minute
	// mailMaintainBatch 是维护任务单批处理的记录数
	mailMaintainBatch = 200
)

// KefuMailAutoService 是自动回复策略：首次来信确认、延时发送、会话结束与原文清理
type KefuMailAutoService struct {
	// 邮箱配置的数据访问
	mailboxRepository *repository.KefuMailboxRepository
	// 邮件线程的数据访问
	threadRepository *repository.KefuMailThreadRepository
	// AI 回复记录的数据访问
	replyRepository *repository.KefuAiReplyRepository
	// 会话的数据访问
	sessionRepository *repository.KefuSessionRepository
	// 会话消息的数据访问
	messageRepository *repository.KefuMessageRepository
	// 邮件收发记录的数据访问
	logRepository *repository.KefuMailLogRepository
	// 渠道的数据访问
	channelRepository *repository.KefuChannelRepository
	// 邮件 AI 生成
	aiService *KefuMailAiService
	// 邮件出站
	sendService *KefuMailSendService
	// 会话业务处理，结束会话时关闭线程
	sessionService *KefuSessionService
	// 系统文件业务处理，清理邮件附件
	fileService *systemservice.SysFileService
	// 启动标记
	startOnce sync.Once
}

// NewKefuMailAutoService 创建自动回复策略
func NewKefuMailAutoService() *KefuMailAutoService {
	return &KefuMailAutoService{
		mailboxRepository: repository.NewKefuMailboxRepository(),
		threadRepository:  repository.NewKefuMailThreadRepository(),
		replyRepository:   repository.NewKefuAiReplyRepository(),
		sessionRepository: repository.NewKefuSessionRepository(),
		messageRepository: repository.NewKefuMessageRepository(),
		logRepository:     repository.NewKefuMailLogRepository(),
		channelRepository: repository.NewKefuChannelRepository(),
		aiService:         NewKefuMailAiService(),
		sendService:       NewKefuMailSendService(),
		sessionService:    NewKefuSessionService(),
		fileService:       systemservice.NewSysFileService(),
	}
}

// OnInbound 处理一封正常入站邮件：工作时间判断、首次来信自动确认、AI 生成
func (s *KefuMailAutoService) OnInbound(
	ctx context.Context, mailbox *model.KefuMailbox, thread *model.KefuMailThread,
	session *model.KefuSession, parsed *mail.Parsed,
) {
	if !mailEnabled() || mailbox.Status != model.KefuMailboxStatusNormal {
		return
	}
	if mailbox.ReplyMode == model.KefuMailReplyModeOff {
		return
	}

	// 非工作时间只回固定模板，不做 AI 实质回复，提示下一个工作时间处理
	if !mailInWorkTime(mailbox, time.Now()) {
		if thread.MailCount <= 1 {
			s.sendTemplate(ctx, mailbox, session, thread, mailTemplateOffHours, parsed)
		}
		return
	}

	// 首次来信自动确认是固定模板，不属于 AI 草稿
	if mailbox.AutoAckEnabled == 1 && thread.MailCount <= 1 {
		s.sendTemplate(ctx, mailbox, session, thread, mailTemplateAck, parsed)
	}

	s.aiService.GenerateAndDispatch(ctx, mailbox, thread, session, parsed.Text)
}

// sendTemplate 按邮箱语言发送固定模板
func (s *KefuMailAutoService) sendTemplate(
	ctx context.Context, mailbox *model.KefuMailbox, session *model.KefuSession,
	thread *model.KefuMailThread, kind string, parsed *mail.Parsed,
) {
	language := mailReplyLanguage(mailbox, parsed.Text)
	senderName := parsed.FromName
	if strings.TrimSpace(senderName) == "" {
		senderName = parsed.FromAddr
	}
	company := ""
	if channel, err := s.channelRepository.GetByCode(session.ChannelCode); err == nil {
		company = channel.Name
	}
	content := renderMailTemplate(
		mailTemplateOf(mailbox, kind, language),
		mailTemplateVars(mailbox, senderName, thread.Subject, session.SessionNo, "", company),
	)
	if strings.TrimSpace(content) == "" {
		return
	}
	subject := normalizeMailSubject(session.SessionNo, thread.Subject)
	if kind == mailTemplateAck {
		subject = normalizeMailSubject("已收到您的邮件 "+session.SessionNo, thread.Subject)
		if language == model.KefuMailLanguageEn {
			subject = normalizeMailSubject("We received your email "+session.SessionNo, thread.Subject)
		}
	}
	if _, err := s.sendService.SendAuto(ctx, session, subject, content); err != nil {
		log.Logger.Warn("send kefu mail template failed",
			zap.Int64("sessionId", session.SessionID), zap.String("kind", kind), zap.Error(err))
	}
}

// StartTasks 启动延时发送与日常维护任务
func (s *KefuMailAutoService) StartTasks() {
	s.startOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(mailAutoScanInterval)
			defer ticker.Stop()
			for range ticker.C {
				s.processScheduled()
			}
		}()
		go func() {
			time.Sleep(mailMaintainDelay)
			s.maintain()
			ticker := time.NewTicker(mailMaintainInterval)
			defer ticker.Stop()
			for range ticker.C {
				s.maintain()
			}
		}()
		log.Kernel.Info("start kefu mail auto reply worker")
	})
}

// processScheduled 扫描到期的延时邮件，发送前重新校验会话与邮箱状态
func (s *KefuMailAutoService) processScheduled() {
	if !mailEnabled() {
		return
	}
	list, err := s.replyRepository.ListScheduledBefore(model.Now(), mailMaintainBatch)
	if err != nil {
		log.Logger.Error("list kefu scheduled mail failed", zap.Error(err))
		return
	}
	for i := range list {
		reply := list[i]
		s.sendScheduled(&reply)
	}
}

// sendScheduled 发送一条到期的延时邮件，条件不满足时取消
func (s *KefuMailAutoService) sendScheduled(reply *model.KefuAiReply) {
	session, err := s.sessionRepository.GetByID(reply.SessionID)
	if err != nil {
		s.cancelReply(reply.ReplyID)
		return
	}
	// 会话已转人工、关闭 AI 或已结束时不再自动发送
	if session.Status != model.KefuSessionStatusAi ||
		session.AiMode != model.KefuSessionAiModeOn {
		s.cancelReply(reply.ReplyID)
		return
	}
	channel, err := s.channelRepository.GetByCode(session.ChannelCode)
	if err != nil {
		s.cancelReply(reply.ReplyID)
		return
	}
	mailbox, err := s.mailboxRepository.GetByChannelID(channel.ChannelID)
	if err != nil || mailbox.Status != model.KefuMailboxStatusNormal {
		s.cancelReply(reply.ReplyID)
		return
	}
	// 期间已有客服回复时取消，避免重复打扰客户
	if s.hasAgentReplyAfter(session.SessionID, reply.CreateTime) {
		s.cancelReply(reply.ReplyID)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if _, err := s.sendService.SendDraft(ctx, session, reply,
		reply.Subject, reply.Content, nil, true); err != nil {
		log.Logger.Warn("send kefu scheduled mail failed",
			zap.Int64("replyId", reply.ReplyID), zap.Error(err))
	}
}

// hasAgentReplyAfter 判断会话在指定时间之后是否已有客服回复
func (s *KefuMailAutoService) hasAgentReplyAfter(
	sessionID int64, since model.Time,
) bool {
	messages, err := s.messageRepository.ListRecent(sessionID, 20)
	if err != nil {
		return false
	}
	for i := range messages {
		item := messages[i]
		if item.FromRole != model.KefuMessageFromAgent {
			continue
		}
		if item.CreateTime.Time.After(since.Time) {
			return true
		}
	}
	return false
}

// cancelReply 取消一条待发送记录
func (s *KefuMailAutoService) cancelReply(replyID int64) {
	if err := s.replyRepository.UpdateFields(replyID, map[string]any{
		"send_status": model.KefuAiReplySendCancelled,
	}); err != nil {
		log.Logger.Warn("cancel kefu scheduled mail failed",
			zap.Int64("replyId", replyID), zap.Error(err))
	}
}

// maintain 执行邮件会话自动结束与原始邮件、附件清理
func (s *KefuMailAutoService) maintain() {
	s.closeIdleSessions()
	s.cleanRawFiles()
	s.cleanAttachments()
}

// closeIdleSessions 结束超过 kefu.mail.close_days 天无往来的邮件会话
func (s *KefuMailAutoService) closeIdleSessions() {
	days := setting.GetInt(configKeyMailCloseDays, defaultMailCloseDays)
	if days <= 0 {
		return
	}
	before := model.Time{Time: time.Now().AddDate(0, 0, -days)}
	threads, err := s.threadRepository.ListClosedBefore(before, mailMaintainBatch)
	if err != nil {
		log.Logger.Error("list kefu idle mail threads failed", zap.Error(err))
		return
	}
	for i := range threads {
		thread := threads[i]
		if thread.SessionID <= 0 {
			_ = s.threadRepository.UpdateFields(thread.ThreadID, map[string]any{
				"status": model.KefuMailThreadStatusClosed,
			})
			continue
		}
		session, err := s.sessionRepository.GetByID(thread.SessionID)
		if err != nil || session.Status == model.KefuSessionStatusClosed {
			_ = s.threadRepository.UpdateFields(thread.ThreadID, map[string]any{
				"status": model.KefuMailThreadStatusClosed,
			})
			continue
		}
		if err := s.sessionRepository.Close([]int64{session.SessionID},
			"长时间无来信自动结束"); err != nil {
			log.Logger.Warn("close kefu idle mail session failed",
				zap.Int64("sessionId", session.SessionID), zap.Error(err))
			continue
		}
		s.sessionService.closeMailSession(session)
	}
}

// cleanRawFiles 按 kefu.mail.raw_keep_days 清理原始邮件
func (s *KefuMailAutoService) cleanRawFiles() {
	days := setting.GetInt(configKeyMailRawKeepDays, defaultMailRawKeepDays)
	if days <= 0 {
		return
	}
	before := model.Time{Time: time.Now().AddDate(0, 0, -days)}
	list, err := s.logRepository.ListExpiredRaw(before, mailMaintainBatch)
	if err != nil {
		log.Logger.Error("list kefu expired mail raw failed", zap.Error(err))
		return
	}
	for i := range list {
		record := list[i]
		if record.RawPath != "" {
			fullPath := filepath.Join(systemservice.UploadDir, filepath.FromSlash(record.RawPath))
			if err := os.Remove(fullPath); err != nil && !errors.Is(err, os.ErrNotExist) {
				log.Logger.Warn("remove kefu mail raw failed",
					zap.String("path", record.RawPath), zap.Error(err))
			}
		}
		if err := s.logRepository.UpdateFields(record.MailID, map[string]any{
			"raw_path": "",
		}); err != nil {
			log.Logger.Warn("clear kefu mail raw path failed",
				zap.Int64("mailId", record.MailID), zap.Error(err))
		}
	}
}

// cleanAttachments 按 kefu.mail.attachment_keep_days 清理邮件附件，
// 清理后会话文本仍在，消息里的附件标记为已过期
func (s *KefuMailAutoService) cleanAttachments() {
	days := setting.GetInt(configKeyMailAttachmentKeepDays, defaultMailAttachmentKeepDays)
	if days <= 0 {
		return
	}
	before := model.Time{Time: time.Now().AddDate(0, 0, -days)}
	list, err := s.logRepository.ListExpiredInbound(before, mailMaintainBatch)
	if err != nil {
		log.Logger.Error("list kefu expired mail attachments failed", zap.Error(err))
		return
	}
	for i := range list {
		record := list[i]
		if record.AttachmentCount <= 0 || record.KefuMessageID <= 0 {
			continue
		}
		message, err := s.messageRepository.GetByID(record.KefuMessageID)
		if err != nil {
			continue
		}
		fileIDs := mailPayloadFileIDs(message.Payload)
		if len(fileIDs) > 0 {
			if err := s.fileService.Remove(fileIDs); err != nil {
				log.Logger.Warn("remove kefu mail attachments failed",
					zap.Int64("mailId", record.MailID), zap.Error(err))
			}
		}
		_ = s.messageRepository.UpdateContent(message.MessageID, map[string]any{
			"payload": markMailAttachmentsExpired(message.Payload),
		})
		_ = s.logRepository.UpdateFields(record.MailID, map[string]any{
			"attachment_count": 0,
		})
	}
}

// mailPayloadFileIDs 从消息 payload 里取出附件文件 id
func mailPayloadFileIDs(payload string) []int64 {
	parsed := struct {
		Meta struct {
			Attachments []model.KefuMailAttachment `json:"attachments"`
		} `json:"meta"`
	}{}
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
		return nil
	}
	ids := make([]int64, 0, len(parsed.Meta.Attachments))
	for _, item := range parsed.Meta.Attachments {
		if item.FileID > 0 {
			ids = append(ids, item.FileID)
		}
	}
	return ids
}

// markMailAttachmentsExpired 把消息 payload 里的附件标记为已过期
func markMailAttachmentsExpired(payload string) string {
	parsed := make(map[string]any)
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
		return payload
	}
	meta, ok := parsed["meta"].(map[string]any)
	if !ok {
		return payload
	}
	meta["attachments"] = []any{}
	meta["attachmentExpired"] = true
	raw, err := json.Marshal(parsed)
	if err != nil {
		return payload
	}
	return string(raw)
}
