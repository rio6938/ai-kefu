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
	// ErrChannelNotFound 是渠道不存在
	ErrChannelNotFound = errors.New("渠道不存在")
)

// KefuChannelRepository 是渠道的数据访问
type KefuChannelRepository struct{}

// NewKefuChannelRepository 创建渠道的数据访问
func NewKefuChannelRepository() *KefuChannelRepository {
	return &KefuChannelRepository{}
}

// db 返回带 kefu_channel 条件的查询
func (r *KefuChannelRepository) db() *gorm.DB {
	return db.DB.Model(&model.KefuChannel{}).Where("deleted = 0")
}

// GetByID 按 id 查询渠道
func (r *KefuChannelRepository) GetByID(channelID int64) (*model.KefuChannel, error) {
	channel := new(model.KefuChannel)
	err := r.db().Where("channel_id = ?", channelID).First(channel).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrChannelNotFound
	}
	if err != nil {
		return nil, err
	}
	return channel, nil
}

// GetByCode 按渠道标识查询渠道
func (r *KefuChannelRepository) GetByCode(channelCode string) (*model.KefuChannel, error) {
	channel := new(model.KefuChannel)
	err := r.db().Where("channel_code = ?", channelCode).First(channel).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrChannelNotFound
	}
	if err != nil {
		return nil, err
	}
	return channel, nil
}

// Page 分页查询渠道
func (r *KefuChannelRepository) Page(param model.KefuChannelParam, page common.PageParam) ([]model.KefuChannel, int64, error) {
	query := r.db()
	if param.ChannelCode != "" {
		query = query.Where("channel_code like ?", "%"+param.ChannelCode+"%")
	}
	if param.ChannelType != "" {
		query = query.Where("channel_type = ?", param.ChannelType)
	}
	if param.Name != "" {
		query = query.Where("name like ?", "%"+param.Name+"%")
	}
	if param.Status != nil {
		query = query.Where("status = ?", *param.Status)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	var list []model.KefuChannel
	query = query.Offset(page.Offset()).Limit(page.PageSize())
	for _, order := range orderClausesWith(page, "channel_id asc") {
		query = query.Order(order)
	}
	if err := query.Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, count, nil
}

// ExistsByCode 判断渠道标识是否已存在
func (r *KefuChannelRepository) ExistsByCode(channelCode string, excludeID int64) (bool, error) {
	query := r.db().Where("channel_code = ?", channelCode)
	if excludeID > 0 {
		query = query.Where("channel_id <> ?", excludeID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// ListByIDs 按 id 批量查询渠道，回填渠道名称时使用
func (r *KefuChannelRepository) ListByIDs(channelIDs []int64) ([]model.KefuChannel, error) {
	if len(channelIDs) == 0 {
		return nil, nil
	}
	var list []model.KefuChannel
	if err := r.db().Where("channel_id in ?", channelIDs).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// List 查询全部未删除的渠道，关联链编辑与渠道下拉共用
func (r *KefuChannelRepository) List() ([]model.KefuChannel, error) {
	var list []model.KefuChannel
	if err := r.db().Order("channel_id asc").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// Create 新增渠道
func (r *KefuChannelRepository) Create(channel *model.KefuChannel) error {
	return db.DB.Create(channel).Error
}

// Update 修改渠道
func (r *KefuChannelRepository) Update(channel *model.KefuChannel) error {
	return db.DB.Save(channel).Error
}

// UpdateStatus 修改渠道状态
func (r *KefuChannelRepository) UpdateStatus(channelID int64, status int) error {
	return db.DB.Model(&model.KefuChannel{}).Where("channel_id = ?", channelID).
		Updates(map[string]any{"status": status, "update_time": model.Now()}).Error
}
