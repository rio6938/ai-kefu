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
	// ErrPromptNotFound 是提示词模板不存在
	ErrPromptNotFound = errors.New("提示词模板不存在")
)

// KefuPromptRepository 是提示词模板的数据访问
type KefuPromptRepository struct{}

// NewKefuPromptRepository 创建提示词模板的数据访问
func NewKefuPromptRepository() *KefuPromptRepository {
	return &KefuPromptRepository{}
}

// db 返回带 kefu_prompt 条件的查询
func (r *KefuPromptRepository) db() *gorm.DB {
	return db.DB.Model(&model.KefuPrompt{}).Where("deleted = 0")
}

// GetByID 按 id 查询提示词模板
func (r *KefuPromptRepository) GetByID(promptID int64) (*model.KefuPrompt, error) {
	record := new(model.KefuPrompt)
	err := r.db().Where("prompt_id = ?", promptID).First(record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrPromptNotFound
	}
	if err != nil {
		return nil, err
	}
	return record, nil
}

// Page 分页查询提示词模板
func (r *KefuPromptRepository) Page(
	param model.KefuPromptParam, page common.PageParam,
) ([]model.KefuPrompt, int64, error) {
	query := r.db()
	if param.Name != "" {
		query = query.Where("name like ?", "%"+param.Name+"%")
	}
	if param.Status != nil {
		query = query.Where("status = ?", *param.Status)
	}
	if param.IsDefault != nil {
		query = query.Where("is_default = ?", *param.IsDefault)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	var list []model.KefuPrompt
	query = query.Offset(page.Offset()).Limit(page.PageSize())
	for _, order := range orderClausesWith(page, "prompt_id asc") {
		query = query.Order(order)
	}
	if err := query.Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, count, nil
}

// List 查询启用中的提示词模板
func (r *KefuPromptRepository) List() ([]model.KefuPrompt, error) {
	query := r.db().Where("status = ?", model.KefuPromptStatusNormal)
	var list []model.KefuPrompt
	if err := query.Order("prompt_id asc").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// GetDefault 查询全局默认模板
func (r *KefuPromptRepository) GetDefault() (*model.KefuPrompt, error) {
	record := new(model.KefuPrompt)
	err := r.db().
		Where("is_default = ? and status = ?",
			model.KefuPromptIsDefault, model.KefuPromptStatusNormal).
		Order("prompt_id asc").First(record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrPromptNotFound
	}
	if err != nil {
		return nil, err
	}
	return record, nil
}

// Create 新增提示词模板
func (r *KefuPromptRepository) Create(record *model.KefuPrompt) error {
	return db.DB.Create(record).Error
}

// Update 修改提示词模板
func (r *KefuPromptRepository) Update(record *model.KefuPrompt) error {
	return db.DB.Save(record).Error
}

// Remove 软删除提示词模板
func (r *KefuPromptRepository) Remove(promptID int64) error {
	return db.DB.Model(&model.KefuPrompt{}).Where("prompt_id = ?", promptID).
		Updates(map[string]any{"deleted": 1, "update_time": model.Now()}).Error
}

// ClearDefault 取消其它模板的默认标记，保证全系统只有一个默认模板
func (r *KefuPromptRepository) ClearDefault(excludeID int64) error {
	query := db.DB.Model(&model.KefuPrompt{}).
		Where("is_default = ? and deleted = 0", model.KefuPromptIsDefault)
	if excludeID > 0 {
		query = query.Where("prompt_id <> ?", excludeID)
	}
	return query.Updates(map[string]any{
		"is_default":  model.KefuPromptNotDefault,
		"update_time": model.Now(),
	}).Error
}
