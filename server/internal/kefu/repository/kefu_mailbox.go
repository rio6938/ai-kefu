package repository

import (
	"errors"
	"strings"

	"gorm.io/gorm"

	"admin/server/model"
	"admin/server/pkg/db"
)

// 常见的业务错误
var (
	// ErrMailboxNotFound 是邮箱配置不存在
	ErrMailboxNotFound = errors.New("邮箱配置不存在")
)

// KefuMailboxRepository 是邮箱配置的数据访问
type KefuMailboxRepository struct{}

// NewKefuMailboxRepository 创建邮箱配置的数据访问
func NewKefuMailboxRepository() *KefuMailboxRepository {
	return &KefuMailboxRepository{}
}

// db 返回带 kefu_mailbox 条件的查询
func (r *KefuMailboxRepository) db() *gorm.DB {
	return db.DB.Model(&model.KefuMailbox{}).Where("deleted = 0")
}

// GetByID 按 id 查询邮箱配置
func (r *KefuMailboxRepository) GetByID(mailboxID int64) (*model.KefuMailbox, error) {
	mailbox := new(model.KefuMailbox)
	err := r.db().Where("mailbox_id = ?", mailboxID).First(mailbox).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrMailboxNotFound
	}
	if err != nil {
		return nil, err
	}
	return mailbox, nil
}

// GetByChannelID 按渠道查询邮箱配置，一个渠道最多一条
func (r *KefuMailboxRepository) GetByChannelID(channelID int64) (*model.KefuMailbox, error) {
	mailbox := new(model.KefuMailbox)
	err := r.db().Where("channel_id = ?", channelID).First(mailbox).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrMailboxNotFound
	}
	if err != nil {
		return nil, err
	}
	return mailbox, nil
}

// ListByChannelIDs 按渠道批量查询邮箱配置，渠道列表回填邮箱地址
func (r *KefuMailboxRepository) ListByChannelIDs(channelIDs []int64) ([]model.KefuMailbox, error) {
	if len(channelIDs) == 0 {
		return nil, nil
	}
	var list []model.KefuMailbox
	if err := r.db().Where("channel_id in ?", channelIDs).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// ListEnabled 查询所有启用且开启同步的邮箱，启动收信 worker 时使用
func (r *KefuMailboxRepository) ListEnabled() ([]model.KefuMailbox, error) {
	var list []model.KefuMailbox
	if err := r.db().Where("status = ?", model.KefuMailboxStatusNormal).
		Order("mailbox_id asc").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// ExistsByEmail 判断邮箱地址是否已被绑定，excludeID 用于修改时排除自身
func (r *KefuMailboxRepository) ExistsByEmail(email string, excludeID int64) (bool, error) {
	query := r.db().Where("lower(email_address) = ?", strings.ToLower(email))
	if excludeID > 0 {
		query = query.Where("mailbox_id <> ?", excludeID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// Create 新增邮箱配置
func (r *KefuMailboxRepository) Create(mailbox *model.KefuMailbox) error {
	return db.DB.Create(mailbox).Error
}

// Update 修改邮箱配置
func (r *KefuMailboxRepository) Update(mailbox *model.KefuMailbox) error {
	return db.DB.Save(mailbox).Error
}

// UpdateFields 按字段更新邮箱配置
func (r *KefuMailboxRepository) UpdateFields(mailboxID int64, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	fields["update_time"] = model.Now()
	return db.DB.Model(&model.KefuMailbox{}).Where("mailbox_id = ?", mailboxID).
		Updates(fields).Error
}

// RemoveByChannelID 软删除渠道绑定的邮箱配置
func (r *KefuMailboxRepository) RemoveByChannelID(channelID int64) error {
	return db.DB.Model(&model.KefuMailbox{}).Where("channel_id = ?", channelID).
		Updates(map[string]any{
			"deleted":     1,
			"status":      model.KefuMailboxStatusDisabled,
			"update_time": model.Now(),
		}).Error
}
