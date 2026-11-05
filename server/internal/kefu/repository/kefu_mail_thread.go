package repository

import (
	"errors"

	"gorm.io/gorm"

	"admin/server/model"
	"admin/server/pkg/db"
)

// 常见的业务错误
var (
	// ErrMailThreadNotFound 是邮件线程不存在
	ErrMailThreadNotFound = errors.New("邮件线程不存在")
)

// KefuMailThreadRepository 是邮件线程的数据访问
type KefuMailThreadRepository struct{}

// NewKefuMailThreadRepository 创建邮件线程的数据访问
func NewKefuMailThreadRepository() *KefuMailThreadRepository {
	return &KefuMailThreadRepository{}
}

// db 返回带 kefu_mail_thread 条件的查询
func (r *KefuMailThreadRepository) db() *gorm.DB {
	return db.DB.Model(&model.KefuMailThread{}).Where("deleted = 0")
}

// GetByID 按 id 查询邮件线程
func (r *KefuMailThreadRepository) GetByID(threadID int64) (*model.KefuMailThread, error) {
	thread := new(model.KefuMailThread)
	err := r.db().Where("thread_id = ?", threadID).First(thread).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrMailThreadNotFound
	}
	if err != nil {
		return nil, err
	}
	return thread, nil
}

// GetByRootMessageID 按渠道与根邮件 Message-ID 查询线程
func (r *KefuMailThreadRepository) GetByRootMessageID(
	channelID int64, rootMessageID string,
) (*model.KefuMailThread, error) {
	thread := new(model.KefuMailThread)
	err := r.db().Where("channel_id = ? and root_message_id = ?", channelID, rootMessageID).
		First(thread).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrMailThreadNotFound
	}
	if err != nil {
		return nil, err
	}
	return thread, nil
}

// GetBySessionID 按会话查询邮件线程
func (r *KefuMailThreadRepository) GetBySessionID(sessionID int64) (*model.KefuMailThread, error) {
	thread := new(model.KefuMailThread)
	err := r.db().Where("session_id = ?", sessionID).Order("thread_id desc").First(thread).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrMailThreadNotFound
	}
	if err != nil {
		return nil, err
	}
	return thread, nil
}

// FindLatestBySubject 按「主题 + 发件人」在时间窗口内匹配最近的线程，无线程头时兜底
func (r *KefuMailThreadRepository) FindLatestBySubject(
	channelID int64, visitorID int64, subjectKey string, since model.Time,
) (*model.KefuMailThread, error) {
	thread := new(model.KefuMailThread)
	err := r.db().
		Where("channel_id = ? and visitor_id = ? and subject_key = ? and last_mail_time >= ?",
			channelID, visitorID, subjectKey, since).
		Order("last_mail_time desc").First(thread).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrMailThreadNotFound
	}
	if err != nil {
		return nil, err
	}
	return thread, nil
}

// CountByVisitor 统计发件人的历史线程数，工作台客户信息展示
func (r *KefuMailThreadRepository) CountByVisitor(visitorID int64) (int64, error) {
	var count int64
	if err := r.db().Where("visitor_id = ?", visitorID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// Create 新增邮件线程
func (r *KefuMailThreadRepository) Create(thread *model.KefuMailThread) error {
	return db.DB.Create(thread).Error
}

// Update 修改邮件线程
func (r *KefuMailThreadRepository) Update(thread *model.KefuMailThread) error {
	return db.DB.Save(thread).Error
}

// UpdateFields 按字段更新邮件线程
func (r *KefuMailThreadRepository) UpdateFields(threadID int64, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	fields["update_time"] = model.Now()
	return db.DB.Model(&model.KefuMailThread{}).Where("thread_id = ?", threadID).
		Updates(fields).Error
}

// ListBySessionIDs 按会话批量查询邮件线程，会话列表回填邮件信息
func (r *KefuMailThreadRepository) ListBySessionIDs(
	sessionIDs []int64,
) ([]model.KefuMailThread, error) {
	if len(sessionIDs) == 0 {
		return nil, nil
	}
	var list []model.KefuMailThread
	if err := r.db().Where("session_id in ?", sessionIDs).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// ListClosedBefore 查询最后邮件时间早于指定时间的活跃线程，用于自动结束邮件会话
func (r *KefuMailThreadRepository) ListClosedBefore(
	before model.Time, limit int,
) ([]model.KefuMailThread, error) {
	var list []model.KefuMailThread
	if err := r.db().
		Where("status = ? and last_mail_time < ?", model.KefuMailThreadStatusActive, before).
		Order("thread_id asc").Limit(limit).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// ListActiveByChannel 查询渠道下所有活跃线程，用于统计与会话维护
func (r *KefuMailThreadRepository) ListActiveByChannel(channelID int64) ([]model.KefuMailThread, error) {
	var list []model.KefuMailThread
	if err := r.db().
		Where("channel_id = ? and status = ?", channelID, model.KefuMailThreadStatusActive).
		Order("thread_id asc").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}
