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
	// ErrQuickReplyNotFound 是快捷回复不存在
	ErrQuickReplyNotFound = errors.New("快捷回复不存在")
)

// KefuQuickReplyRepository 是快捷回复的数据访问
type KefuQuickReplyRepository struct{}

// NewKefuQuickReplyRepository 创建快捷回复的数据访问
func NewKefuQuickReplyRepository() *KefuQuickReplyRepository {
	return &KefuQuickReplyRepository{}
}

// db 返回带 kefu_quick_reply 条件的查询
func (r *KefuQuickReplyRepository) db() *gorm.DB {
	return db.DB.Model(&model.KefuQuickReply{}).Where("deleted = 0")
}

// GetByID 按 id 查询快捷回复
func (r *KefuQuickReplyRepository) GetByID(quickReplyID int64) (*model.KefuQuickReply, error) {
	record := new(model.KefuQuickReply)
	err := r.db().Where("quick_reply_id = ?", quickReplyID).First(record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrQuickReplyNotFound
	}
	if err != nil {
		return nil, err
	}
	return record, nil
}

// Page 分页查询快捷回复
func (r *KefuQuickReplyRepository) Page(
	param model.KefuQuickReplyParam, page common.PageParam,
) ([]model.KefuQuickReply, int64, error) {
	query := r.db()
	if param.Scope != "" {
		query = query.Where("scope = ?", param.Scope)
	}
	if param.Status != nil {
		query = query.Where("status = ?", *param.Status)
	}
	if param.GroupCode != "" {
		query = query.Where("group_code = ?", param.GroupCode)
	}
	if param.Keyword != "" {
		keyword := "%" + param.Keyword + "%"
		query = query.Where("title like ? or content like ?", keyword, keyword)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	var list []model.KefuQuickReply
	query = query.Offset(page.Offset()).Limit(page.PageSize())
	for _, order := range orderClausesWith(page, "sort_number asc, quick_reply_id asc") {
		query = query.Order(order)
	}
	if err := query.Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, count, nil
}

// ListEnabled 查询启用中的快捷回复，按适用范围过滤
func (r *KefuQuickReplyRepository) ListEnabled(
	scope string,
) ([]model.KefuQuickReply, error) {
	query := r.db().
		Where("scope = ? and status = ?", scope, model.KefuQuickReplyStatusNormal)
	var list []model.KefuQuickReply
	if err := query.Order("sort_number asc, quick_reply_id asc").
		Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// ExistsTitle 判断同一适用范围下标题是否已存在，excludeID 为修改时的自身 id
func (r *KefuQuickReplyRepository) ExistsTitle(
	scope, title string, excludeID int64,
) (bool, error) {
	query := r.db().Where("scope = ? and title = ?", scope, title)
	if excludeID > 0 {
		query = query.Where("quick_reply_id <> ?", excludeID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// Create 新增快捷回复
func (r *KefuQuickReplyRepository) Create(record *model.KefuQuickReply) error {
	return db.DB.Create(record).Error
}

// Update 修改快捷回复
func (r *KefuQuickReplyRepository) Update(record *model.KefuQuickReply) error {
	return db.DB.Save(record).Error
}

// UpdateStatus 修改快捷回复状态
func (r *KefuQuickReplyRepository) UpdateStatus(quickReplyID int64, status int) error {
	return db.DB.Model(&model.KefuQuickReply{}).
		Where("quick_reply_id = ?", quickReplyID).
		Updates(map[string]any{"status": status, "update_time": model.Now()}).Error
}

// IncrementUseCount 累加快捷回复的使用次数
func (r *KefuQuickReplyRepository) IncrementUseCount(quickReplyID int64) error {
	return db.DB.Model(&model.KefuQuickReply{}).
		Where("quick_reply_id = ?", quickReplyID).
		UpdateColumn("use_count", gorm.Expr("use_count + 1")).Error
}

// Remove 软删除快捷回复
func (r *KefuQuickReplyRepository) Remove(quickReplyID int64) error {
	return db.DB.Model(&model.KefuQuickReply{}).
		Where("quick_reply_id = ?", quickReplyID).
		Updates(map[string]any{"deleted": 1, "update_time": model.Now()}).Error
}
