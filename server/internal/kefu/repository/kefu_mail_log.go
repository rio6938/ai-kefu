package repository

import (
	"errors"

	"gorm.io/gorm"

	"admin/server/internal/common"
	"admin/server/model"
	"admin/server/pkg/db"
)

// 常见的业务错误
var (
	// ErrMailLogNotFound 是邮件收发记录不存在
	ErrMailLogNotFound = errors.New("邮件收发记录不存在")
)

// KefuMailLogRepository 是邮件收发记录的数据访问
type KefuMailLogRepository struct{}

// NewKefuMailLogRepository 创建邮件收发记录的数据访问
func NewKefuMailLogRepository() *KefuMailLogRepository {
	return &KefuMailLogRepository{}
}

// db 返回带 kefu_mail_log 条件的查询
func (r *KefuMailLogRepository) db() *gorm.DB {
	return db.DB.Model(&model.KefuMailLog{}).Where("deleted = 0")
}

// GetByID 按 id 查询邮件收发记录
func (r *KefuMailLogRepository) GetByID(mailID int64) (*model.KefuMailLog, error) {
	record := new(model.KefuMailLog)
	err := r.db().Where("mail_id = ?", mailID).First(record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrMailLogNotFound
	}
	if err != nil {
		return nil, err
	}
	return record, nil
}

// GetByMailKey 按幂等键查询邮件收发记录，重复投递与重复发送都靠它判断
func (r *KefuMailLogRepository) GetByMailKey(
	mailboxID int64, mailKey string,
) (*model.KefuMailLog, error) {
	record := new(model.KefuMailLog)
	err := r.db().Where("mailbox_id = ? and mail_key = ?", mailboxID, mailKey).
		First(record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrMailLogNotFound
	}
	if err != nil {
		return nil, err
	}
	return record, nil
}

// GetByMessageID 按邮件 Message-ID 查询收发记录，线程匹配时使用
func (r *KefuMailLogRepository) GetByMessageID(
	mailboxID int64, messageID string,
) (*model.KefuMailLog, error) {
	record := new(model.KefuMailLog)
	err := r.db().Where("mailbox_id = ? and message_id = ?", mailboxID, messageID).
		Order("mail_id desc").First(record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrMailLogNotFound
	}
	if err != nil {
		return nil, err
	}
	return record, nil
}

// GetByKefuMessageID 按会话消息查询出站记录，保证同一条消息只发一次
func (r *KefuMailLogRepository) GetByKefuMessageID(
	kefuMessageID int64,
) (*model.KefuMailLog, error) {
	record := new(model.KefuMailLog)
	err := r.db().Where("kefu_message_id = ? and direction = ?",
		kefuMessageID, model.KefuMailDirectionOut).First(record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrMailLogNotFound
	}
	if err != nil {
		return nil, err
	}
	return record, nil
}

// ListBySession 查询会话内的邮件记录，工作台展示邮件信息
func (r *KefuMailLogRepository) ListBySession(sessionID int64) ([]model.KefuMailLog, error) {
	var list []model.KefuMailLog
	if err := r.db().Where("session_id = ?", sessionID).
		Order("sent_time asc, mail_id asc").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// Create 新增邮件收发记录
func (r *KefuMailLogRepository) Create(record *model.KefuMailLog) error {
	return db.DB.Create(record).Error
}

// UpdateFields 按字段更新邮件收发记录
func (r *KefuMailLogRepository) UpdateFields(
	mailID int64, fields map[string]any,
) error {
	if len(fields) == 0 {
		return nil
	}
	fields["update_time"] = model.Now()
	return db.DB.Model(&model.KefuMailLog{}).Where("mail_id = ?", mailID).
		Updates(fields).Error
}

// Page 分页查询邮件收发记录
func (r *KefuMailLogRepository) Page(
	param model.KefuMailLogParam, page common.PageParam,
) ([]model.KefuMailLog, int64, error) {
	query := r.scope(param)
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	var list []model.KefuMailLog
	query = query.Offset(page.Offset()).Limit(page.PageSize())
	for _, order := range orderClausesWith(page, "sent_time desc") {
		query = query.Order(order)
	}
	if err := query.Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, count, nil
}

// scope 追加邮件收发记录的筛选条件
func (r *KefuMailLogRepository) scope(param model.KefuMailLogParam) *gorm.DB {
	query := r.db()
	if param.MailboxID != nil {
		query = query.Where("mailbox_id = ?", *param.MailboxID)
	}
	if param.ChannelID != nil {
		query = query.Where("channel_id = ?", *param.ChannelID)
	}
	if param.SessionID != nil {
		query = query.Where("session_id = ?", *param.SessionID)
	}
	if param.Direction != "" {
		query = query.Where("direction = ?", param.Direction)
	}
	if param.Status != "" {
		query = query.Where("status = ?", param.Status)
	}
	if param.StartTime != "" {
		query = query.Where("sent_time >= ?", param.StartTime)
	}
	if param.EndTime != "" {
		query = query.Where("sent_time <= ?", param.EndTime)
	}
	if param.Keyword != "" {
		like := "%" + param.Keyword + "%"
		query = query.Where("subject like ? or from_addr like ? or to_addr like ?", like, like, like)
	}
	return query
}

// ListFailedBefore 查询指定时间前的失败记录，用于失败重发与排查
func (r *KefuMailLogRepository) ListFailedBefore(
	before model.Time, limit int,
) ([]model.KefuMailLog, error) {
	var list []model.KefuMailLog
	if err := r.db().
		Where("direction = ? and status = ? and sent_time < ?",
			model.KefuMailDirectionOut, model.KefuMailStatusFailed, before).
		Order("mail_id asc").Limit(limit).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// ListExpiredRaw 查询超过保留期的入站记录，清理原始邮件与附件时使用
func (r *KefuMailLogRepository) ListExpiredRaw(
	before model.Time, limit int,
) ([]model.KefuMailLog, error) {
	var list []model.KefuMailLog
	if err := r.db().
		Where("direction = ? and raw_path <> '' and sent_time < ?",
			model.KefuMailDirectionIn, before).
		Order("mail_id asc").Limit(limit).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// ListExpiredInbound 查询超过保留期的入站邮件，清理附件与原文时使用
func (r *KefuMailLogRepository) ListExpiredInbound(
	before model.Time, limit int,
) ([]model.KefuMailLog, error) {
	var list []model.KefuMailLog
	if err := r.db().
		Where("direction = ? and sent_time < ?", model.KefuMailDirectionIn, before).
		Order("mail_id asc").Limit(limit).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}
