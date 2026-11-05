package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"

	"admin/server/internal/kefu/mail"
	"admin/server/internal/kefu/repository"
	systemservice "admin/server/internal/system/service"
	"admin/server/model"
	"admin/server/pkg/log"
	"admin/server/pkg/setting"
)

// 邮件附件的最大数量，超出部分不再保存
const mailMaxAttachmentCount = 20

// KefuMailInboundService 是入站邮件的编排：解析、防环、线程匹配、访客与会话落库
type KefuMailInboundService struct {
	// 邮件线程的数据访问
	threadRepository *repository.KefuMailThreadRepository
	// 邮件收发记录的数据访问
	logRepository *repository.KefuMailLogRepository
	// 会话消息的数据访问
	messageRepository *repository.KefuMessageRepository
	// 会话的数据访问，入站后更新消息统计
	sessionRepository *repository.KefuSessionRepository
	// 渠道的数据访问，取渠道标识
	channelRepository *repository.KefuChannelRepository
	// 邮箱配置的数据访问，重新解析时按 id 取邮箱
	mailboxRepository *repository.KefuMailboxRepository
	// 访客的业务处理，按邮箱取建访客
	visitorService *KefuVisitorService
	// 会话的业务处理，按线程取建会话
	sessionService *KefuSessionService
	// 系统文件业务处理，保存附件
	fileService *systemservice.SysFileService
	// 自动回复策略，入站后触发自动确认与 AI 生成
	autoService *KefuMailAutoService
}

// NewKefuMailInboundService 创建入站邮件的编排
func NewKefuMailInboundService() *KefuMailInboundService {
	return &KefuMailInboundService{
		threadRepository:  repository.NewKefuMailThreadRepository(),
		logRepository:     repository.NewKefuMailLogRepository(),
		messageRepository: repository.NewKefuMessageRepository(),
		sessionRepository: repository.NewKefuSessionRepository(),
		channelRepository: repository.NewKefuChannelRepository(),
		mailboxRepository: repository.NewKefuMailboxRepository(),
		visitorService:    NewKefuVisitorService(),
		sessionService:    NewKefuSessionService(),
		fileService:       systemservice.NewSysFileService(),
		autoService:       NewKefuMailAutoService(),
	}
}

// kefuMailMessagePayload 是邮件消息 payload 的结构，
// 工作台「显示原文」读 meta.fullText，附件下载读 meta.attachments
type kefuMailMessagePayload struct {
	// 协议版本
	Version int `json:"version"`
	// 消息类型
	Type string `json:"type"`
	// 发送者角色
	Role string `json:"role"`
	// 正文
	Content string `json:"content"`
	// 消息id
	MessageID int64 `json:"messageId"`
	// 会话id
	SessionID int64 `json:"sessionId"`
	// 附加信息
	Meta kefuMailMessageMeta `json:"meta"`
}

// kefuMailMessageMeta 是邮件消息的附加信息
type kefuMailMessageMeta struct {
	// 是否邮件消息
	Mail bool `json:"mail"`
	// 邮件记录id
	MailID int64 `json:"mailId"`
	// 线程id
	MailThreadID int64 `json:"mailThreadId"`
	// 邮件 Message-ID
	MailMessageID string `json:"messageId"`
	// In-Reply-To
	InReplyTo string `json:"inReplyTo"`
	// References
	References string `json:"references"`
	// 主题
	Subject string `json:"subject"`
	// 发件人
	From string `json:"from"`
	// 收件人
	To string `json:"to"`
	// 抄送
	Cc string `json:"cc"`
	// 完整正文, 保留引用与签名
	FullText string `json:"fullText"`
	// HTML 正文
	HTML string `json:"html"`
	// 附件列表
	Attachments []model.KefuMailAttachment `json:"attachments"`
	// 附件是否因超限被跳过
	AttachmentSkipped bool `json:"attachmentSkipped"`
	// 附件文件名, 单个附件消息使用
	Name string `json:"name,omitempty"`
	// 附件大小
	Size int64 `json:"size,omitempty"`
	// 附件类型
	ContentType string `json:"contentType,omitempty"`
	// 附件id
	FileID int64 `json:"fileId,omitempty"`
	// 附件地址
	URL string `json:"url,omitempty"`
}

// Handle 处理一封拉取到的邮件，返回处理状态；
// 单封失败只写 failed 记录并继续，游标由调用方前进
func (s *KefuMailInboundService) Handle(
	ctx context.Context, mailbox *model.KefuMailbox, uid uint32, size int64, raw []byte,
) (string, error) {
	return s.handle(ctx, mailbox, uid, size, raw, false)
}

// HandleRescan 在 UIDVALIDITY 变化后按首次同步窗口重扫时使用，
// 此时 UID 会从头开始，不能按 UID 去重，改为按 Message-ID 去重
func (s *KefuMailInboundService) HandleRescan(
	ctx context.Context, mailbox *model.KefuMailbox, uid uint32, size int64, raw []byte,
) (string, error) {
	return s.handle(ctx, mailbox, uid, size, raw, true)
}

// handle 是入站邮件的实际处理，rescan 为 true 时跳过 UID 幂等检查
func (s *KefuMailInboundService) handle(
	ctx context.Context, mailbox *model.KefuMailbox, uid uint32, size int64,
	raw []byte, rescan bool,
) (string, error) {
	mailKey := fmt.Sprintf("in:%d", uid)
	if !rescan {
		if _, err := s.logRepository.GetByMailKey(mailbox.MailboxID, mailKey); err == nil {
			return model.KefuMailStatusSkipped, nil
		} else if !errors.Is(err, repository.ErrMailLogNotFound) {
			return model.KefuMailStatusFailed, err
		}
	}

	now := model.Now()
	rawPath := saveMailRaw(mailbox.MailboxID, uid, raw)
	record := &model.KefuMailLog{
		MailboxID:  mailbox.MailboxID,
		ChannelID:  mailbox.ChannelID,
		Direction:  model.KefuMailDirectionIn,
		MailKey:    mailKey,
		UID:        int64(uid),
		Size:       size,
		Status:     model.KefuMailStatusReceived,
		RawPath:    rawPath,
		SentTime:   now,
		CreateTime: now,
		UpdateTime: now,
	}

	parsed, err := mail.Parse(raw)
	if err != nil {
		record.Status = model.KefuMailStatusFailed
		record.Error = trimMailError(err.Error())
		if createErr := s.logRepository.Create(record); createErr != nil {
			// 并发重复投递时唯一索引会失败，按重复处理
			return model.KefuMailStatusSkipped, nil
		}
		return model.KefuMailStatusFailed, err
	}
	// 以 Message-ID 去重：重复投递、UIDVALIDITY 变化重扫都靠它避免重复建会话
	if parsed.MessageID != "" {
		if _, err := s.logRepository.GetByMessageID(mailbox.MailboxID, parsed.MessageID); err == nil {
			removeMailRaw(rawPath)
			return model.KefuMailStatusSkipped, nil
		}
	}
	record.MessageID = parsed.MessageID
	record.InReplyTo = parsed.InReplyTo
	record.References = parsed.References
	record.FromAddr = parsed.FromAddr
	record.FromName = parsed.FromName
	record.ToAddr = parsed.ToAddr
	record.CcAddr = parsed.CcAddr
	record.Subject = parsed.Subject
	record.SentTime = model.Time{Time: parsed.Date}
	record.Size = parsed.Size
	record.AttachmentCount = len(parsed.Attachments)
	if createErr := s.logRepository.Create(record); createErr != nil {
		return model.KefuMailStatusSkipped, nil
	}

	channel, err := s.channelRepository.GetByID(mailbox.ChannelID)
	if err != nil {
		s.markMailFailed(record, err)
		return model.KefuMailStatusFailed, err
	}
	if channel.Status != model.KefuChannelStatusNormal {
		s.skipMail(record, model.KefuMailSkipOther)
		return model.KefuMailStatusSkipped, nil
	}

	// 退信：把原收件人邮箱标记为无效并停止自动回复，退信本身不建会话
	if parsed.IsBounce() {
		s.visitorService.MarkMailInvalid(parsed.BounceRecipient)
		s.skipMail(record, model.KefuMailSkipBounce)
		return model.KefuMailStatusSkipped, nil
	}

	// 防环与垃圾过滤：只写跳过记录，不建会话、不触发 AI
	if reason := mailSkipReason(mailbox, parsed); reason != "" {
		s.skipMail(record, reason)
		return model.KefuMailStatusSkipped, nil
	}

	visitor, err := s.visitorService.EnsureMailVisitor(
		channel.ChannelCode, parsed.FromAddr, parsed.FromName)
	if err != nil {
		s.markMailFailed(record, err)
		return model.KefuMailStatusFailed, err
	}

	thread, err := s.ensureThread(mailbox, channel, visitor, parsed)
	if err != nil {
		s.markMailFailed(record, err)
		return model.KefuMailStatusFailed, err
	}
	session, err := s.sessionService.EnsureMailSession(visitor, channel, thread)
	if err != nil {
		s.markMailFailed(record, err)
		return model.KefuMailStatusFailed, err
	}

	attachments, skipped := s.saveAttachments(mailbox, parsed)
	content := parsed.Text
	if strings.TrimSpace(content) == "" {
		content = "客户发送了一封空邮件"
	}

	now = model.Now()
	meta := kefuMailMessageMeta{
		Mail:              true,
		MailID:            record.MailID,
		MailThreadID:      thread.ThreadID,
		MailMessageID:     parsed.MessageID,
		InReplyTo:         parsed.InReplyTo,
		References:        parsed.References,
		Subject:           parsed.Subject,
		From:              parsed.FromAddr,
		To:                parsed.ToAddr,
		Cc:                parsed.CcAddr,
		FullText:          parsed.FullText,
		HTML:              parsed.HTML,
		Attachments:       attachments,
		AttachmentSkipped: skipped,
	}
	textMessage := &model.KefuMessage{
		SessionID:   session.SessionID,
		ClientMsgNo: fmt.Sprintf("mail_in_%d", record.MailID),
		FromRole:    model.KefuMessageFromVisitor,
		FromID:      visitorUID(visitor.VisitorID),
		ContentType: model.KefuMessageTypeText,
		Content:     content,
		Payload:     marshalMailPayload("text", content, session.SessionID, meta),
		CreateTime:  model.Time{Time: parsed.Date},
		UpdateTime:  now,
	}
	inserted, err := s.messageRepository.CreateIfAbsent(textMessage)
	if err != nil {
		s.markMailFailed(record, err)
		return model.KefuMailStatusFailed, err
	}
	if inserted {
		if err := s.sessionRepository.UpdateMessageStat(
			session.SessionID, model.Time{Time: parsed.Date}, 1); err != nil {
			log.Logger.Warn("update kefu session stat failed",
				zap.Int64("sessionId", session.SessionID), zap.Error(err))
		}
	}

	for index, item := range attachments {
		contentType := model.KefuMessageTypeFile
		if strings.HasPrefix(item.ContentType, "image/") {
			contentType = model.KefuMessageTypeImage
		}
		fileMeta := meta
		fileMeta.Name = item.Name
		fileMeta.Size = item.Size
		fileMeta.ContentType = item.ContentType
		fileMeta.FileID = item.FileID
		fileMeta.URL = item.URL
		attachmentMessage := &model.KefuMessage{
			SessionID:   session.SessionID,
			ClientMsgNo: fmt.Sprintf("mail_in_%d_a%d", record.MailID, index),
			FromRole:    model.KefuMessageFromVisitor,
			FromID:      visitorUID(visitor.VisitorID),
			ContentType: contentType,
			Content:     item.URL,
			Payload:     marshalMailPayload(contentType, item.URL, session.SessionID, fileMeta),
			CreateTime:  model.Time{Time: parsed.Date},
			UpdateTime:  now,
		}
		insertedAttachment, err := s.messageRepository.CreateIfAbsent(attachmentMessage)
		if err != nil {
			log.Logger.Warn("save kefu mail attachment message failed",
				zap.Int64("mailId", record.MailID), zap.Error(err))
			continue
		}
		if insertedAttachment {
			if err := s.sessionRepository.UpdateMessageStat(
				session.SessionID, model.Time{Time: parsed.Date}, 1); err != nil {
				log.Logger.Warn("update kefu session stat failed",
					zap.Int64("sessionId", session.SessionID), zap.Error(err))
			}
		}
	}

	references := strings.TrimSpace(parsed.References)
	if parsed.MessageID != "" {
		references = strings.TrimSpace(references + " " + parsed.MessageID)
	}
	if err := s.threadRepository.UpdateFields(thread.ThreadID, map[string]any{
		"last_inbound_message_id": parsed.MessageID,
		"references":              references,
		"mail_count":              thread.MailCount + 1,
		"last_mail_time":          model.Time{Time: parsed.Date},
		"status":                  model.KefuMailThreadStatusActive,
	}); err != nil {
		log.Logger.Warn("update kefu mail thread failed",
			zap.Int64("threadId", thread.ThreadID), zap.Error(err))
	}
	if err := s.logRepository.UpdateFields(record.MailID, map[string]any{
		"thread_id":       thread.ThreadID,
		"session_id":      session.SessionID,
		"kefu_message_id": textMessage.MessageID,
	}); err != nil {
		log.Logger.Warn("update kefu mail log failed",
			zap.Int64("mailId", record.MailID), zap.Error(err))
	}

	thread.SessionID = session.SessionID
	thread.Subject = parsed.Subject
	thread.RootMessageID = parsed.MessageID
	// AI 生成比较慢，放到后台执行，不阻塞收信
	go s.autoService.OnInbound(context.Background(), mailbox, thread, session, parsed)
	return model.KefuMailStatusReceived, nil
}

// ensureThread 按引用链、主题兜底匹配线程，匹配不到时新建
func (s *KefuMailInboundService) ensureThread(
	mailbox *model.KefuMailbox, channel *model.KefuChannel,
	visitor *model.KefuVisitor, parsed *mail.Parsed,
) (*model.KefuMailThread, error) {
	// 先用 In-Reply-To / References 找线程根邮件或历史邮件的 Message-ID
	for _, reference := range parsed.ReferenceList() {
		if reference == "" {
			continue
		}
		thread, err := s.threadRepository.GetByRootMessageID(mailbox.ChannelID, reference)
		if err == nil {
			return thread, nil
		}
		if !errors.Is(err, repository.ErrMailThreadNotFound) {
			return nil, err
		}
		mailLog, err := s.logRepository.GetByMessageID(mailbox.MailboxID, reference)
		if err == nil && mailLog.ThreadID > 0 {
			thread, err := s.threadRepository.GetByID(mailLog.ThreadID)
			if err == nil {
				return thread, nil
			}
		}
	}

	// 无线程头时按「主题 + 发件人 + 时间窗口」兜底
	windowDays := setting.GetInt(configKeyMailThreadWindowDays, defaultMailThreadWindowDays)
	if windowDays > 0 && parsed.SubjectKey != "" {
		since := model.Time{Time: time.Now().AddDate(0, 0, -windowDays)}
		thread, err := s.threadRepository.FindLatestBySubject(
			mailbox.ChannelID, visitor.VisitorID, parsed.SubjectKey, since)
		if err == nil {
			return thread, nil
		}
		if !errors.Is(err, repository.ErrMailThreadNotFound) {
			return nil, err
		}
	}

	now := model.Now()
	rootMessageID := parsed.MessageID
	if rootMessageID == "" {
		rootMessageID = fmt.Sprintf("generated_%d_%s", time.Now().UnixNano(), randomHex(4))
	}
	thread := &model.KefuMailThread{
		ChannelID:     mailbox.ChannelID,
		ChannelCode:   channel.ChannelCode,
		VisitorID:     visitor.VisitorID,
		RootMessageID: rootMessageID,
		Subject:       parsed.Subject,
		SubjectKey:    parsed.SubjectKey,
		LastMailTime:  model.Time{Time: parsed.Date},
		Status:        model.KefuMailThreadStatusActive,
		CreateTime:    now,
		UpdateTime:    now,
	}
	if err := s.threadRepository.Create(thread); err != nil {
		return nil, err
	}
	return thread, nil
}

// saveAttachments 保存入站附件，总大小超限时保留正文、跳过附件并标记
func (s *KefuMailInboundService) saveAttachments(
	mailbox *model.KefuMailbox, parsed *mail.Parsed,
) ([]model.KefuMailAttachment, bool) {
	if len(parsed.Attachments) == 0 {
		return nil, false
	}
	limit := int64(mailboxMaxSizeMB(mailbox)) << 20
	list := make([]model.KefuMailAttachment, 0, len(parsed.Attachments))
	skipped := false
	var total int64
	for index, item := range parsed.Attachments {
		if index >= mailMaxAttachmentCount {
			skipped = true
			break
		}
		if !mailAttachmentAllowed(item.FileName) {
			skipped = true
			continue
		}
		size := int64(len(item.Content))
		total += size
		if total > limit {
			skipped = true
			continue
		}
		contentType := item.ContentType
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		record, err := s.fileService.Save(item.FileName, bytes.NewReader(item.Content),
			size, contentType, -1)
		if err != nil {
			log.Logger.Warn("save kefu mail attachment failed",
				zap.String("fileName", item.FileName), zap.Error(err))
			skipped = true
			continue
		}
		list = append(list, model.KefuMailAttachment{
			FileID:      record.ID,
			Name:        record.Name,
			Size:        record.Length,
			ContentType: record.ContentType,
			URL:         record.DownloadURL,
		})
	}
	return list, skipped
}

// skipMail 记录入站邮件被跳过的原因
func (s *KefuMailInboundService) skipMail(record *model.KefuMailLog, reason string) {
	_ = s.logRepository.UpdateFields(record.MailID, map[string]any{
		"status":      model.KefuMailStatusSkipped,
		"skip_reason": reason,
	})
}

// removeMailRaw 删除没有落库的原始邮件，避免重复投递留下孤儿文件
func removeMailRaw(rawPath string) {
	if strings.TrimSpace(rawPath) == "" {
		return
	}
	fullPath := filepath.Join(systemservice.UploadDir, filepath.FromSlash(rawPath))
	if err := os.Remove(fullPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Logger.Warn("remove kefu mail raw failed", zap.String("path", rawPath), zap.Error(err))
	}
}

// markMailFailed 记录入站处理失败原因
func (s *KefuMailInboundService) markMailFailed(record *model.KefuMailLog, err error) {
	_ = s.logRepository.UpdateFields(record.MailID, map[string]any{
		"status": model.KefuMailStatusFailed,
		"error":  trimMailError(err.Error()),
	})
}

// saveMailRaw 把原始邮件存到 uploads/kefu/mail/日期 下，返回相对路径
func saveMailRaw(mailboxID int64, uid uint32, raw []byte) string {
	dir := time.Now().Format("20060102")
	name := fmt.Sprintf("%d-%d-%s.eml", mailboxID, uid, randomHex(4))
	relative := filepath.ToSlash(filepath.Join("kefu", "mail", dir, name))
	fullPath := filepath.Join(systemservice.UploadDir, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		log.Logger.Warn("create kefu mail raw dir failed", zap.Error(err))
		return ""
	}
	if err := os.WriteFile(fullPath, raw, 0o644); err != nil {
		log.Logger.Warn("save kefu mail raw failed", zap.Error(err))
		return ""
	}
	return relative
}

// marshalMailPayload 拼装邮件消息 payload
func marshalMailPayload(
	contentType, content string, sessionID int64, meta kefuMailMessageMeta,
) string {
	role := model.KefuMessageFromVisitor
	payload := kefuMailMessagePayload{
		Version:   1,
		Type:      contentType,
		Role:      role,
		Content:   content,
		SessionID: sessionID,
		Meta:      meta,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		log.Logger.Warn("marshal kefu mail payload failed", zap.Error(err))
		return ""
	}
	return string(raw)
}

// Rehandle 按收信记录重新解析一次，用于修复解析失败的历史邮件
func (s *KefuMailInboundService) Rehandle(ctx context.Context, mailID int64) error {
	record, err := s.logRepository.GetByID(mailID)
	if err != nil {
		return err
	}
	if record.Direction != model.KefuMailDirectionIn {
		return errors.New("只能重新解析入站邮件")
	}
	if strings.TrimSpace(record.RawPath) == "" {
		return errors.New("原始邮件不存在，无法重新解析")
	}
	fullPath := filepath.Join(systemservice.UploadDir, filepath.FromSlash(record.RawPath))
	raw, err := os.ReadFile(fullPath)
	if err != nil {
		return errors.New("原始邮件不存在，无法重新解析")
	}
	mailbox, err := s.mailboxRepository.GetByID(record.MailboxID)
	if err != nil {
		return err
	}
	// 重新解析前把旧的失败记录移出幂等键，避免唯一索引冲突，记录本身保留可追溯
	if err := s.logRepository.UpdateFields(record.MailID, map[string]any{
		"deleted":  1,
		"mail_key": fmt.Sprintf("%s:reparsed:%d", record.MailKey, record.MailID),
	}); err != nil {
		return err
	}
	if _, err := s.Handle(ctx, mailbox, uint32(record.UID), int64(len(raw)), raw); err != nil {
		return err
	}
	return nil
}
