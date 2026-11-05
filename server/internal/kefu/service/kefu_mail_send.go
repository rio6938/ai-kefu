package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"

	mailclient "admin/server/internal/kefu/mail"
	"admin/server/internal/kefu/repository"
	systemrepository "admin/server/internal/system/repository"
	systemservice "admin/server/internal/system/service"
	"admin/server/model"
	"admin/server/pkg/log"
	pkgmail "admin/server/pkg/mail"
)

// mailMaxRetry 是自动发送失败的最大重试次数
const mailMaxRetry = 3

// 出站邮件相关的业务错误
var (
	// ErrMailSendDisabled 是邮箱渠道或邮箱已停用，不能发信
	ErrMailSendDisabled = errors.New("邮箱已停用，不能发信")
	// ErrMailAttachmentTooLarge 是附件超过单封上限
	ErrMailAttachmentTooLarge = errors.New("附件超过单封邮件大小限制")
	// ErrMailAttachmentNotFound 是附件文件不存在
	ErrMailAttachmentNotFound = errors.New("附件不存在")
	// ErrMailLogNotFound 是发信记录不存在
	ErrMailLogNotFound = repository.ErrMailLogNotFound
	// ErrMailResendInvalid 是只能重发失败的出站邮件
	ErrMailResendInvalid = errors.New("只能重发失败的邮件")
)

// KefuMailSendService 是出站邮件的编排：MIME 拼装、SMTP 发送与发信记录
type KefuMailSendService struct {
	// 邮箱配置的业务处理，取解密后的 SMTP 配置
	mailboxService *KefuMailboxService
	// 邮件线程的数据访问
	threadRepository *repository.KefuMailThreadRepository
	// 邮件收发记录的数据访问
	logRepository *repository.KefuMailLogRepository
	// 会话消息的数据访问
	messageRepository *repository.KefuMessageRepository
	// 会话的数据访问
	sessionRepository *repository.KefuSessionRepository
	// 访客的数据访问，取出收件邮箱
	visitorRepository *repository.KefuVisitorRepository
	// AI 回复记录的数据访问，草稿发送后回写状态
	replyRepository *repository.KefuAiReplyRepository
	// 系统文件的数据访问，读取发信附件
	fileRepository *systemrepository.SysFileRepository
	// 渠道的数据访问，发信前按渠道加载邮箱
	channelRepository *repository.KefuChannelRepository
}

// NewKefuMailSendService 创建出站邮件的编排
func NewKefuMailSendService() *KefuMailSendService {
	return &KefuMailSendService{
		mailboxService:    NewKefuMailboxService(),
		threadRepository:  repository.NewKefuMailThreadRepository(),
		logRepository:     repository.NewKefuMailLogRepository(),
		messageRepository: repository.NewKefuMessageRepository(),
		sessionRepository: repository.NewKefuSessionRepository(),
		visitorRepository: repository.NewKefuVisitorRepository(),
		replyRepository:   repository.NewKefuAiReplyRepository(),
		fileRepository:    systemrepository.NewSysFileRepository(),
		channelRepository: repository.NewKefuChannelRepository(),
	}
}

// KefuMailSendOptions 是一封出站邮件的内容
type KefuMailSendOptions struct {
	// 主题
	Subject string
	// 纯文本正文
	Text string
	// 附件
	Attachments []model.KefuMailAttachmentForm
	// 是否是自动回复，自动回复统一加防环头
	Auto bool
	// 发送人角色, visitor/agent/ai/system
	FromRole string
	// 发送者标识
	FromID string
	// 是否为 AI 生成内容
	AiGenerated int
	// 关联的 AI 回复id
	ReplyID int64
	// 是否引用原文
	QuoteOriginal bool
}

// SendAgentMessage 客服从邮箱会话回复客户：先落消息再发信，失败时撤销消息避免假成功
func (s *KefuMailSendService) SendAgentMessage(
	ctx context.Context, agent *model.KefuAgent, session *model.KefuSession,
	form model.KefuMessageForm,
) (*model.KefuMessage, error) {
	content := strings.TrimSpace(form.Content)
	if content == "" {
		return nil, ErrMessageContentRequired
	}
	attachments := parseMailAttachments(form.Payload)
	return s.send(ctx, session, KefuMailSendOptions{
		Text:          content,
		Attachments:   attachments,
		FromRole:      model.KefuMessageFromAgent,
		FromID:        agentUID(agent.UserID),
		QuoteOriginal: true,
	})
}

// SendDraft 发送 AI 草稿或自动回复，成功后回写 kefu_ai_reply 的发送状态
func (s *KefuMailSendService) SendDraft(
	ctx context.Context, session *model.KefuSession, reply *model.KefuAiReply,
	subject, content string, attachments []model.KefuMailAttachmentForm, auto bool,
) (*model.KefuMessage, error) {
	message, err := s.send(ctx, session, KefuMailSendOptions{
		Subject:       subject,
		Text:          content,
		Attachments:   attachments,
		Auto:          auto,
		FromRole:      model.KefuMessageFromAi,
		FromID:        "mail_ai",
		AiGenerated:   1,
		ReplyID:       reply.ReplyID,
		QuoteOriginal: true,
	})
	if err != nil {
		if reply != nil {
			_ = s.replyRepository.UpdateFields(reply.ReplyID, map[string]any{
				"send_status": model.KefuAiReplySendFailed,
			})
		}
		return nil, err
	}
	if reply != nil {
		_ = s.replyRepository.UpdateFields(reply.ReplyID, map[string]any{
			"send_status": model.KefuAiReplySendSent,
			"subject":     subject,
		})
	}
	return message, nil
}

// send 执行一次出站：落消息 → 发信 → 写发信记录；
// 发信失败时软删除消息并写 failed 记录，工作台不会出现「假成功」
func (s *KefuMailSendService) send(
	ctx context.Context, session *model.KefuSession, options KefuMailSendOptions,
) (*model.KefuMessage, error) {
	if !mailEnabled() {
		return nil, ErrMailDisabled
	}
	channel, err := s.channelRepository.GetByCode(session.ChannelCode)
	if err != nil {
		return nil, err
	}
	if channel.Status != model.KefuChannelStatusNormal {
		return nil, ErrMailSendDisabled
	}
	mailbox, err := s.mailboxService.mailboxOfChannel(channel.ChannelID)
	if err != nil {
		return nil, err
	}
	thread, err := s.threadRepository.GetBySessionID(session.SessionID)
	if err != nil {
		return nil, err
	}
	visitor, err := s.visitorRepository.GetByID(session.VisitorID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(visitor.Email) == "" {
		return nil, errors.New("会话访客没有邮箱地址，不能发信")
	}

	subject := normalizeMailSubject(options.Subject, thread.Subject)
	text := options.Text
	language := mailReplyLanguage(mailbox, text)
	quote := ""
	if options.QuoteOriginal && mailbox.QuoteOriginal == 1 {
		quote = mailQuoteOriginal(s.lastInboundText(session.SessionID), language)
	}
	signature := mailSignatureOf(mailbox, language)
	plain := mailPlainText(text, signature, quote)
	htmlBody := mailTextToHTML(plain)

	mailAttachments, metaAttachments, err := s.buildAttachments(options.Attachments, mailbox)
	if err != nil {
		return nil, err
	}
	if err := ensureMailSize(mailbox, plain, mailAttachments); err != nil {
		return nil, err
	}

	now := model.Now()
	message := &model.KefuMessage{
		SessionID:   session.SessionID,
		ClientMsgNo: generateClientMsgNo("mail"),
		FromRole:    options.FromRole,
		FromID:      options.FromID,
		ContentType: model.KefuMessageTypeText,
		Content:     strings.TrimSpace(text),
		AiGenerated: options.AiGenerated,
		AiReplyID:   options.ReplyID,
		CreateTime:  now,
		UpdateTime:  now,
	}
	payload := map[string]any{
		"version":   1,
		"type":      "text",
		"role":      options.FromRole,
		"content":   text,
		"sessionId": session.SessionID,
		"meta": map[string]any{
			"mail":        true,
			"subject":     subject,
			"attachments": metaAttachments,
		},
	}
	if options.Auto {
		payload["autoSubmitted"] = true
	}
	if raw, err := json.Marshal(payload); err == nil {
		message.Payload = string(raw)
	}
	if err := s.messageRepository.Create(message); err != nil {
		return nil, err
	}

	record := &model.KefuMailLog{
		MailboxID:       mailbox.MailboxID,
		ChannelID:       mailbox.ChannelID,
		ThreadID:        thread.ThreadID,
		SessionID:       session.SessionID,
		KefuMessageID:   message.MessageID,
		Direction:       model.KefuMailDirectionOut,
		MailKey:         fmt.Sprintf("out:%d", message.MessageID),
		ToAddr:          visitor.Email,
		FromAddr:        mailbox.EmailAddress,
		FromName:        mailbox.DisplayName,
		Subject:         subject,
		SentTime:        now,
		AttachmentCount: len(mailAttachments),
		Status:          model.KefuMailStatusSending,
		CreateTime:      now,
		UpdateTime:      now,
	}
	if err := s.logRepository.Create(record); err != nil {
		_ = s.messageRepository.Remove(message.MessageID)
		return nil, err
	}

	headers := map[string]string{}
	if options.Auto {
		headers["Auto-Submitted"] = "auto-replied"
		headers["X-Auto-Response-Suppress"] = "All"
	}
	references := strings.Fields(thread.References)
	if thread.LastInboundMessageID != "" {
		references = append(references, thread.LastInboundMessageID)
	}
	mailMessage := &pkgmail.Message{
		To:          []string{visitor.Email},
		Subject:     subject,
		TextBody:    plain,
		HTMLBody:    htmlBody,
		Attachments: mailAttachments,
		Headers:     headers,
		InReplyTo:   thread.LastInboundMessageID,
		References:  references,
		ReplyTo:     mailbox.EmailAddress,
		FromName:    mailbox.DisplayName,
	}
	messageID, sendErr := s.deliver(ctx, mailbox, mailMessage)
	if sendErr != nil {
		_ = s.messageRepository.Remove(message.MessageID)
		_ = s.logRepository.UpdateFields(record.MailID, map[string]any{
			"status": model.KefuMailStatusFailed,
			"error":  trimMailError(sendErr.Error()),
		})
		return nil, fmt.Errorf("发送失败，请重试: %w", sendErr)
	}

	// 开启「已发送同步」时把同一封邮件追加到服务商已发送文件夹，失败只记日志
	if mailbox.SyncSent == 1 {
		s.appendSent(ctx, mailbox, mailMessage, messageID)
	}
	if err := s.logRepository.UpdateFields(record.MailID, map[string]any{
		"status":      model.KefuMailStatusSent,
		"message_id":  messageID,
		"in_reply_to": thread.LastInboundMessageID,
	}); err != nil {
		log.Logger.Warn("update kefu mail log failed",
			zap.Int64("mailId", record.MailID), zap.Error(err))
	}
	if err := s.threadRepository.UpdateFields(thread.ThreadID, map[string]any{
		"mail_count":     thread.MailCount + 1,
		"last_mail_time": now,
		"references":     strings.Join(references, " "),
	}); err != nil {
		log.Logger.Warn("update kefu mail thread failed",
			zap.Int64("threadId", thread.ThreadID), zap.Error(err))
	}
	if err := s.sessionRepository.UpdateMessageStat(session.SessionID, now, 0); err != nil {
		log.Logger.Warn("update kefu session stat failed",
			zap.Int64("sessionId", session.SessionID), zap.Error(err))
	}
	// 附件单独落一条文件消息，工作台才能按文件卡片展示与下载
	for index, item := range metaAttachments {
		contentType := model.KefuMessageTypeFile
		if strings.HasPrefix(item.ContentType, "image/") {
			contentType = model.KefuMessageTypeImage
		}
		attachmentMessage := &model.KefuMessage{
			SessionID:   session.SessionID,
			ClientMsgNo: fmt.Sprintf("mail_out_%d_a%d", message.MessageID, index),
			FromRole:    options.FromRole,
			FromID:      options.FromID,
			ContentType: contentType,
			Content:     item.URL,
			Payload:     marshalMailAttachmentPayload(contentType, item, session.SessionID, message.MessageID),
			CreateTime:  now,
			UpdateTime:  now,
		}
		inserted, err := s.messageRepository.CreateIfAbsent(attachmentMessage)
		if err != nil {
			log.Logger.Warn("save kefu outbound attachment message failed",
				zap.Int64("messageId", message.MessageID), zap.Error(err))
			continue
		}
		if inserted {
			if err := s.sessionRepository.UpdateMessageStat(session.SessionID, now, 0); err != nil {
				log.Logger.Warn("update kefu session stat failed",
					zap.Int64("sessionId", session.SessionID), zap.Error(err))
			}
		}
	}
	return message, nil
}

// marshalMailAttachmentPayload 拼装出站附件消息的 payload
func marshalMailAttachmentPayload(
	contentType string, item model.KefuMailAttachment, sessionID, messageID int64,
) string {
	payload := map[string]any{
		"version":   1,
		"type":      contentType,
		"role":      model.KefuMessageFromAi,
		"content":   item.URL,
		"messageId": messageID,
		"sessionId": sessionID,
		"meta": map[string]any{
			"mail":        true,
			"fileId":      item.FileID,
			"name":        item.Name,
			"size":        item.Size,
			"contentType": item.ContentType,
			"url":         item.URL,
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(raw)
}

// deliver 调用 SMTP 发信，失败时按指数退避重试，仍失败返回最后一次错误
func (s *KefuMailSendService) deliver(
	ctx context.Context, mailbox *model.KefuMailbox, message *pkgmail.Message,
) (string, error) {
	config, err := s.mailboxService.smtpConfig(ctx, mailbox)
	if err != nil {
		return "", err
	}
	client, err := pkgmail.New(config)
	if err != nil {
		return "", err
	}

	var lastErr error
	for attempt := 0; attempt < mailMaxRetry; attempt++ {
		if attempt > 0 {
			delay := time.Duration(attempt) * 3 * time.Second
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(delay):
			}
		}
		messageID, err := client.SendMessage(*message)
		if err == nil {
			return messageID, nil
		}
		lastErr = err
	}
	return "", lastErr
}

// lastInboundText 取会话最近一封客户来信的完整文本，用于回复引用
func (s *KefuMailSendService) lastInboundText(sessionID int64) string {
	messages, err := s.messageRepository.ListRecent(sessionID, 20)
	if err != nil {
		return ""
	}
	for i := len(messages) - 1; i >= 0; i-- {
		item := messages[i]
		if item.FromRole != model.KefuMessageFromVisitor || item.Deleted == 1 {
			continue
		}
		var payload struct {
			Meta struct {
				FullText string `json:"fullText"`
			} `json:"meta"`
		}
		if json.Unmarshal([]byte(item.Payload), &payload) == nil &&
			strings.TrimSpace(payload.Meta.FullText) != "" {
			return payload.Meta.FullText
		}
		return item.Content
	}
	return ""
}

// buildAttachments 从系统文件读取附件并校验后缀与大小，同时生成消息里回显的附件列表
func (s *KefuMailSendService) buildAttachments(
	forms []model.KefuMailAttachmentForm, mailbox *model.KefuMailbox,
) ([]pkgmail.Attachment, []model.KefuMailAttachment, error) {
	attachments := make([]pkgmail.Attachment, 0, len(forms))
	metas := make([]model.KefuMailAttachment, 0, len(forms))
	for _, form := range forms {
		if form.FileID <= 0 {
			continue
		}
		record, err := s.fileRepository.GetByID(form.FileID)
		if err != nil {
			return nil, nil, ErrMailAttachmentNotFound
		}
		name := strings.TrimSpace(form.Name)
		if name == "" {
			name = record.Name
		}
		if !mailAttachmentAllowed(name) {
			return nil, nil, fmt.Errorf("附件 %s 的类型不允许发送", name)
		}
		fullPath := filepath.Join(systemservice.UploadDir, filepath.FromSlash(record.Path))
		content, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, nil, ErrMailAttachmentNotFound
		}
		attachments = append(attachments, pkgmail.Attachment{
			FileName:    name,
			ContentType: record.ContentType,
			Content:     content,
		})
		metas = append(metas, model.KefuMailAttachment{
			FileID:      record.ID,
			Name:        name,
			Size:        int64(len(content)),
			ContentType: record.ContentType,
			URL:         record.DownloadURL,
		})
	}
	return attachments, metas, nil
}

// EnsureMailSize 校验单封邮件大小，按 base64 编码后约 4/3 估算，前后端口径一致
func ensureMailSize(
	mailbox *model.KefuMailbox, text string, attachments []pkgmail.Attachment,
) error {
	limit := int64(mailboxMaxSizeMB(mailbox)) << 20
	total := int64(len(text))
	for _, item := range attachments {
		total += int64(len(item.Content)) * 4 / 3
	}
	if total > limit {
		return fmt.Errorf("%w（%d MB）", ErrMailAttachmentTooLarge, mailboxMaxSizeMB(mailbox))
	}
	return nil
}

// normalizeMailSubject 补 Re: 前缀，空主题用原线程主题
func normalizeMailSubject(subject, threadSubject string) string {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		subject = strings.TrimSpace(threadSubject)
	}
	if subject == "" {
		subject = "Re:"
	}
	if !strings.HasPrefix(strings.ToLower(subject), "re:") {
		subject = "Re: " + subject
	}
	return subject
}

// parseMailAttachments 解析消息 payload 里的附件列表
func parseMailAttachments(payload string) []model.KefuMailAttachmentForm {
	value := strings.TrimSpace(payload)
	if value == "" {
		return nil
	}
	parsed := struct {
		Attachments []model.KefuMailAttachmentForm `json:"attachments"`
	}{}
	if err := json.Unmarshal([]byte(value), &parsed); err != nil {
		return nil
	}
	return parsed.Attachments
}

// trimMailError 截断过长的发信错误，避免超出字段长度
func trimMailError(message string) string {
	runes := []rune(message)
	if len(runes) > 500 {
		return string(runes[:500])
	}
	return message
}

// Resend 重发一条失败的出站邮件，只允许 failed 状态，重发不会产生重复邮件
func (s *KefuMailSendService) Resend(ctx context.Context, mailID int64) error {
	record, err := s.logRepository.GetByID(mailID)
	if err != nil {
		return err
	}
	if record.Direction != model.KefuMailDirectionOut ||
		record.Status != model.KefuMailStatusFailed {
		return ErrMailResendInvalid
	}
	session, err := s.sessionRepository.GetByID(record.SessionID)
	if err != nil {
		return err
	}
	// 发送失败时消息被软删除，重发要取回原文，成功后恢复消息
	message, err := s.messageRepository.GetByIDAny(record.KefuMessageID)
	if err != nil {
		return err
	}
	channel, err := s.channelRepository.GetByCode(session.ChannelCode)
	if err != nil {
		return err
	}
	mailbox, err := s.mailboxService.mailboxOfChannel(channel.ChannelID)
	if err != nil {
		return err
	}
	thread, err := s.threadRepository.GetBySessionID(session.SessionID)
	if err != nil {
		return err
	}

	language := mailReplyLanguage(mailbox, message.Content)
	quote := ""
	if mailbox.QuoteOriginal == 1 {
		quote = mailQuoteOriginal(s.lastInboundText(session.SessionID), language)
	}
	plain := mailPlainText(message.Content, mailSignatureOf(mailbox, language), quote)
	messageID, err := s.deliver(ctx, mailbox, &pkgmail.Message{
		To:         []string{record.ToAddr},
		Subject:    record.Subject,
		TextBody:   plain,
		HTMLBody:   mailTextToHTML(plain),
		InReplyTo:  thread.LastInboundMessageID,
		References: strings.Fields(thread.References),
		ReplyTo:    mailbox.EmailAddress,
		FromName:   mailbox.DisplayName,
	})
	if err != nil {
		_ = s.logRepository.UpdateFields(record.MailID, map[string]any{
			"status": model.KefuMailStatusFailed,
			"error":  trimMailError(err.Error()),
		})
		return fmt.Errorf("重发失败: %w", err)
	}
	if err := s.messageRepository.Restore(message.MessageID); err != nil {
		log.Logger.Warn("restore kefu mail message failed",
			zap.Int64("messageId", message.MessageID), zap.Error(err))
	}
	return s.logRepository.UpdateFields(record.MailID, map[string]any{
		"status":     model.KefuMailStatusSent,
		"message_id": messageID,
		"error":      "",
	})
}

// SendAuto 发送一封系统模板邮件（如自动确认），自动回复统一加防环头
func (s *KefuMailSendService) SendAuto(
	ctx context.Context, session *model.KefuSession, subject, content string,
) (*model.KefuMessage, error) {
	return s.send(ctx, session, KefuMailSendOptions{
		Subject:  subject,
		Text:     content,
		Auto:     true,
		FromRole: model.KefuMessageFromAi,
		FromID:   "mail_auto",
	})
}

// appendSent 把已发送邮件追加到 IMAP 已发送文件夹，默认关闭以免与 SMTP 自动保存重复
func (s *KefuMailSendService) appendSent(
	ctx context.Context, mailbox *model.KefuMailbox, message *pkgmail.Message, messageID string,
) {
	smtpConfig, err := s.mailboxService.smtpConfig(ctx, mailbox)
	if err != nil {
		log.Logger.Warn("build kefu sent mail failed",
			zap.Int64("mailboxId", mailbox.MailboxID), zap.Error(err))
		return
	}
	sent := *message
	sent.MessageID = messageID
	raw, _, err := pkgmail.Build(smtpConfig, sent)
	if err != nil {
		log.Logger.Warn("build kefu sent mail failed",
			zap.Int64("mailboxId", mailbox.MailboxID), zap.Error(err))
		return
	}
	imapConfig, err := s.mailboxService.imapConfig(ctx, mailbox)
	if err != nil {
		log.Logger.Warn("connect kefu imap for sent sync failed",
			zap.Int64("mailboxId", mailbox.MailboxID), zap.Error(err))
		return
	}
	session, err := mailclient.New(imapConfig).Connect()
	if err != nil {
		log.Logger.Warn("connect kefu imap for sent sync failed",
			zap.Int64("mailboxId", mailbox.MailboxID), zap.Error(err))
		return
	}
	defer session.Close()
	if err := session.Append(mailbox.SentFolder, raw); err != nil {
		log.Logger.Warn("append kefu sent mail failed",
			zap.Int64("mailboxId", mailbox.MailboxID), zap.Error(err))
	}
}
