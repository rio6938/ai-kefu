package repository

import (
	"errors"

	"gorm.io/gorm"

	"admin/server/model"
	"admin/server/pkg/db"
)

// 常见的业务错误
var (
	// ErrAiReplyNotFound 是 AI 回复记录不存在
	ErrAiReplyNotFound = errors.New("AI 回复记录不存在")
)

// KefuAiReplyRepository 是 AI 回复记录的数据访问
type KefuAiReplyRepository struct{}

// NewKefuAiReplyRepository 创建 AI 回复记录的数据访问
func NewKefuAiReplyRepository() *KefuAiReplyRepository {
	return &KefuAiReplyRepository{}
}

// db 返回带 kefu_ai_reply 条件的查询
func (r *KefuAiReplyRepository) db() *gorm.DB {
	return db.DB.Model(&model.KefuAiReply{}).Where("deleted = 0")
}

// GetByID 按 id 查询 AI 回复记录
func (r *KefuAiReplyRepository) GetByID(replyID int64) (*model.KefuAiReply, error) {
	record := new(model.KefuAiReply)
	err := r.db().Where("reply_id = ?", replyID).First(record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrAiReplyNotFound
	}
	if err != nil {
		return nil, err
	}
	return record, nil
}

// GetStreamingBySession 查询会话进行中的 AI 回复，人工干预时用它定位要中断的回复
func (r *KefuAiReplyRepository) GetStreamingBySession(sessionID int64) (*model.KefuAiReply, error) {
	record := new(model.KefuAiReply)
	err := r.db().
		Where("session_id = ? and status = ?", sessionID, model.KefuAiReplyStatusStreaming).
		Order("reply_id desc").First(record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrAiReplyNotFound
	}
	if err != nil {
		return nil, err
	}
	return record, nil
}

// GetLatestBySession 查询会话最近一次 AI 回复，工作台查看本次实际使用的提示词
func (r *KefuAiReplyRepository) GetLatestBySession(sessionID int64) (*model.KefuAiReply, error) {
	record := new(model.KefuAiReply)
	err := r.db().Where("session_id = ?", sessionID).
		Order("reply_id desc").First(record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrAiReplyNotFound
	}
	if err != nil {
		return nil, err
	}
	return record, nil
}

// Create 新增 AI 回复记录
func (r *KefuAiReplyRepository) Create(record *model.KefuAiReply) error {
	return db.DB.Create(record).Error
}

// UpdateFields 按字段更新 AI 回复记录，只更新传入的字段
func (r *KefuAiReplyRepository) UpdateFields(replyID int64, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	fields["update_time"] = model.Now()
	return db.DB.Model(&model.KefuAiReply{}).Where("reply_id = ?", replyID).
		Updates(fields).Error
}

// GetPendingMailBySession 查询会话当前未发送的邮件草稿或待发送记录
func (r *KefuAiReplyRepository) GetPendingMailBySession(
	sessionID int64,
) (*model.KefuAiReply, error) {
	record := new(model.KefuAiReply)
	err := r.db().
		Where("session_id = ? and channel = ? and send_status in ?",
			sessionID, model.KefuAiReplyChannelMail,
			[]string{model.KefuAiReplySendDraft, model.KefuAiReplySendScheduled}).
		Order("reply_id desc").First(record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrAiReplyNotFound
	}
	if err != nil {
		return nil, err
	}
	return record, nil
}

// ListScheduledBefore 查询已到期的待发送邮件，延时发送 worker 每 30 秒扫描一次
func (r *KefuAiReplyRepository) ListScheduledBefore(
	before model.Time, limit int,
) ([]model.KefuAiReply, error) {
	var list []model.KefuAiReply
	if err := r.db().
		Where("channel = ? and send_status = ? and scheduled_time <= ?",
			model.KefuAiReplyChannelMail, model.KefuAiReplySendScheduled, before).
		Order("scheduled_time asc").Limit(limit).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// CancelScheduledBySession 取消会话内所有待发送记录，客服接管后不再自动发出
func (r *KefuAiReplyRepository) CancelScheduledBySession(sessionID int64) error {
	return db.DB.Model(&model.KefuAiReply{}).
		Where("session_id = ? and channel = ? and send_status = ?",
			sessionID, model.KefuAiReplyChannelMail, model.KefuAiReplySendScheduled).
		Updates(map[string]any{
			"send_status": model.KefuAiReplySendCancelled,
			"update_time": model.Now(),
		}).Error
}

// CountMailSentByThread 统计线程内已自动发出的 AI 邮件数，用于同线程限流
func (r *KefuAiReplyRepository) CountMailSentByThread(threadID int64) (int64, error) {
	var count int64
	if err := r.db().
		Where("channel = ? and mail_thread_id = ? and send_status = ?",
			model.KefuAiReplyChannelMail, threadID, model.KefuAiReplySendSent).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// CountMailSentByVisitor 统计发件人当天已自动发出的 AI 邮件数，用于每日限流
func (r *KefuAiReplyRepository) CountMailSentByVisitor(
	visitorID int64, since model.Time,
) (int64, error) {
	var count int64
	if err := db.DB.Model(&model.KefuAiReply{}).
		Joins("join kefu_mail_thread on kefu_mail_thread.thread_id = kefu_ai_reply.mail_thread_id").
		Where("kefu_ai_reply.deleted = 0 and kefu_ai_reply.channel = ? and kefu_ai_reply.send_status = ?",
			model.KefuAiReplyChannelMail, model.KefuAiReplySendSent).
		Where("kefu_mail_thread.deleted = 0 and kefu_mail_thread.visitor_id = ?", visitorID).
		Where("kefu_ai_reply.update_time >= ?", since).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// LatestMailSentTime 返回线程最近一次自动回复的发送时间，用于最小间隔限流
func (r *KefuAiReplyRepository) LatestMailSentTime(threadID int64) (model.Time, error) {
	record := new(model.KefuAiReply)
	err := r.db().
		Where("channel = ? and mail_thread_id = ? and send_status = ?",
			model.KefuAiReplyChannelMail, threadID, model.KefuAiReplySendSent).
		Order("update_time desc").First(record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Time{}, nil
	}
	if err != nil {
		return model.Time{}, err
	}
	return record.UpdateTime, nil
}

// CancelPendingBySession 取消会话内所有未发送的草稿与待发送记录
func (r *KefuAiReplyRepository) CancelPendingBySession(sessionID int64) error {
	return db.DB.Model(&model.KefuAiReply{}).
		Where("session_id = ? and channel = ? and send_status in ?",
			sessionID, model.KefuAiReplyChannelMail,
			[]string{model.KefuAiReplySendDraft, model.KefuAiReplySendScheduled}).
		Updates(map[string]any{
			"send_status": model.KefuAiReplySendCancelled,
			"update_time": model.Now(),
		}).Error
}
