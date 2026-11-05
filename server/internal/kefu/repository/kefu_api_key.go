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
	// ErrApiKeyNotFound 是 API 密钥不存在
	ErrApiKeyNotFound = errors.New("API 密钥不存在")
)

// KefuApiKeyRepository 是对外 API 密钥的数据访问
type KefuApiKeyRepository struct{}

// NewKefuApiKeyRepository 创建对外 API 密钥的数据访问
func NewKefuApiKeyRepository() *KefuApiKeyRepository {
	return &KefuApiKeyRepository{}
}

// db 返回带 kefu_api_key 条件的查询
func (r *KefuApiKeyRepository) db() *gorm.DB {
	return db.DB.Model(&model.KefuApiKey{}).Where("deleted = 0")
}

// GetByID 按 id 查询 API 密钥
func (r *KefuApiKeyRepository) GetByID(keyID int64) (*model.KefuApiKey, error) {
	record := new(model.KefuApiKey)
	err := r.db().Where("key_id = ?", keyID).First(record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrApiKeyNotFound
	}
	if err != nil {
		return nil, err
	}
	return record, nil
}

// GetByHash 按密钥哈希查询，/v1 接口鉴权时使用
func (r *KefuApiKeyRepository) GetByHash(keyHash string) (*model.KefuApiKey, error) {
	record := new(model.KefuApiKey)
	err := r.db().Where("key_hash = ?", keyHash).First(record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrApiKeyNotFound
	}
	if err != nil {
		return nil, err
	}
	return record, nil
}

// Page 分页查询 API 密钥
func (r *KefuApiKeyRepository) Page(
	param model.KefuApiKeyParam, page common.PageParam,
) ([]model.KefuApiKey, int64, error) {
	query := r.db()
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

	var list []model.KefuApiKey
	query = query.Offset(page.Offset()).Limit(page.PageSize())
	for _, order := range orderClausesWith(page, "key_id asc") {
		query = query.Order(order)
	}
	if err := query.Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, count, nil
}

// Create 新增 API 密钥
func (r *KefuApiKeyRepository) Create(record *model.KefuApiKey) error {
	return db.DB.Create(record).Error
}

// UpdateStatus 修改 API 密钥状态
func (r *KefuApiKeyRepository) UpdateStatus(keyID int64, status int) error {
	return db.DB.Model(&model.KefuApiKey{}).Where("key_id = ?", keyID).
		Updates(map[string]any{"status": status, "update_time": model.Now()}).Error
}

// UpdateLastUsed 刷新 API 密钥的最后使用时间
func (r *KefuApiKeyRepository) UpdateLastUsed(keyID int64) error {
	now := model.Now()
	return db.DB.Model(&model.KefuApiKey{}).Where("key_id = ?", keyID).
		Updates(map[string]any{"last_used_time": now, "update_time": now}).Error
}
