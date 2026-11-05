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
	// ErrModelNotFound 是模型配置不存在
	ErrModelNotFound = errors.New("模型配置不存在")
)

// KefuModelRepository 是模型配置的数据访问
type KefuModelRepository struct{}

// NewKefuModelRepository 创建模型配置的数据访问
func NewKefuModelRepository() *KefuModelRepository {
	return &KefuModelRepository{}
}

// db 返回带 kefu_model 条件的查询
func (r *KefuModelRepository) db() *gorm.DB {
	return db.DB.Model(&model.KefuModel{}).Where("deleted = 0")
}

// GetByID 按 id 查询模型配置
func (r *KefuModelRepository) GetByID(modelID int64) (*model.KefuModel, error) {
	record := new(model.KefuModel)
	err := r.db().Where("model_id = ?", modelID).First(record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrModelNotFound
	}
	if err != nil {
		return nil, err
	}
	return record, nil
}

// GetDefault 查询默认模型配置，没有标记默认时取最早启用的一条；
// AI 自动回复与 OpenAI 兼容接口都用它
func (r *KefuModelRepository) GetDefault() (*model.KefuModel, error) {
	record := new(model.KefuModel)
	err := r.db().Where("status = ?", model.KefuModelStatusNormal).
		Order("is_default desc, model_id asc").First(record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrModelNotFound
	}
	if err != nil {
		return nil, err
	}
	return record, nil
}

// GetEnabledByName 按模型标识或模型名称查询启用中的模型配置，
// /v1 接口的请求体里 model 字段两种写法都兼容
func (r *KefuModelRepository) GetEnabledByName(name string) (*model.KefuModel, error) {
	record := new(model.KefuModel)
	err := r.db().Where("status = ? and (model = ? or name = ?)",
		model.KefuModelStatusNormal, name, name).First(record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrModelNotFound
	}
	if err != nil {
		return nil, err
	}
	return record, nil
}

// ListEnabled 查询全部启用中的模型配置，/v1/models 用它返回模型列表
func (r *KefuModelRepository) ListEnabled() ([]model.KefuModel, error) {
	var list []model.KefuModel
	err := r.db().Where("status = ?", model.KefuModelStatusNormal).
		Order("is_default desc, model_id asc").Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

// GetTranslateModel 查询勾选了「用作翻译模型」的启用模型，
// 多条勾选时取 model_id 最小的一条；没有勾选时返回 ErrModelNotFound，
// 由业务层回退默认对话模型
func (r *KefuModelRepository) GetTranslateModel() (*model.KefuModel, error) {
	record := new(model.KefuModel)
	err := r.db().Where("status = ? and translate_enabled = ?",
		model.KefuModelStatusNormal, model.KefuModelIsTranslate).
		Order("model_id asc").First(record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrModelNotFound
	}
	if err != nil {
		return nil, err
	}
	return record, nil
}

// Page 分页查询模型配置
func (r *KefuModelRepository) Page(
	param model.KefuModelParam, page common.PageParam,
) ([]model.KefuModel, int64, error) {
	query := r.db()
	if param.Name != "" {
		query = query.Where("name like ?", "%"+param.Name+"%")
	}
	if param.Provider != "" {
		query = query.Where("provider = ?", param.Provider)
	}
	if param.Status != nil {
		query = query.Where("status = ?", *param.Status)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	var list []model.KefuModel
	query = query.Offset(page.Offset()).Limit(page.PageSize())
	for _, order := range orderClausesWith(page, "is_default desc, model_id asc") {
		query = query.Order(order)
	}
	if err := query.Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, count, nil
}

// Create 新增模型配置
func (r *KefuModelRepository) Create(record *model.KefuModel) error {
	return db.DB.Create(record).Error
}

// Update 修改模型配置
func (r *KefuModelRepository) Update(record *model.KefuModel) error {
	return db.DB.Save(record).Error
}

// UpdateFields 更新模型配置的指定字段
func (r *KefuModelRepository) UpdateFields(modelID int64, fields map[string]any) error {
	return db.DB.Model(&model.KefuModel{}).Where("model_id = ?", modelID).Updates(fields).Error
}

// Remove 软删除模型配置
func (r *KefuModelRepository) Remove(modelID int64) error {
	return db.DB.Model(&model.KefuModel{}).Where("model_id = ?", modelID).
		Updates(map[string]any{"deleted": 1, "update_time": model.Now()}).Error
}

// ClearDefault 取消其它模型的默认标记，保证只有一个默认模型
func (r *KefuModelRepository) ClearDefault(excludeID int64) error {
	query := db.DB.Model(&model.KefuModel{}).
		Where("is_default = ? and deleted = 0", model.KefuModelIsDefault)
	if excludeID > 0 {
		query = query.Where("model_id <> ?", excludeID)
	}
	return query.Updates(map[string]any{
		"is_default":  model.KefuModelNotDefault,
		"update_time": model.Now(),
	}).Error
}
