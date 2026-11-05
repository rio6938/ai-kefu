package repository

import (
	"errors"

	"gorm.io/gorm"

	"admin/server/model"
	"admin/server/pkg/db"
)

// ErrConfigNotFound 是配置项不存在
var ErrConfigNotFound = errors.New("配置项不存在")

// SysConfigRepository 是配置项的数据访问
type SysConfigRepository struct{}

// NewSysConfigRepository 创建配置项的数据访问
func NewSysConfigRepository() *SysConfigRepository {
	return &SysConfigRepository{}
}

// db 返回带 sys_config 条件的查询
func (r *SysConfigRepository) db() *gorm.DB {
	return db.DB.Model(&model.Config{}).Where("deleted = 0")
}

// List 查询配置项列表，groupCode 为空时返回全部
func (r *SysConfigRepository) List(groupCode string) ([]model.Config, error) {
	var list []model.Config
	query := r.db()
	if groupCode != "" {
		query = query.Where("group_code = ?", groupCode)
	}
	err := query.Order("group_code asc").Order("sort_number asc").
		Order("config_id asc").Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

// GetByKey 按配置键查询配置项
func (r *SysConfigRepository) GetByKey(configKey string) (*model.Config, error) {
	config := new(model.Config)
	err := r.db().Where("config_key = ?", configKey).First(config).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrConfigNotFound
	}
	if err != nil {
		return nil, err
	}
	return config, nil
}

// UpdateValues 在一个事务里按配置键批量修改配置值
func (r *SysConfigRepository) UpdateValues(
	values map[string]string,
	updateUserID int64,
	updateTime model.Time,
) error {
	return db.DB.Transaction(func(tx *gorm.DB) error {
		for configKey, value := range values {
			err := tx.Model(&model.Config{}).
				Where("config_key = ? and deleted = 0", configKey).
				Updates(map[string]any{
					"config_value":   value,
					"update_user_id": updateUserID,
					"update_time":    updateTime,
				}).Error
			if err != nil {
				return err
			}
		}
		return nil
	})
}
