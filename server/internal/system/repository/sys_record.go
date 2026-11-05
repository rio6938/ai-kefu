package repository

import (
	"gorm.io/gorm"

	"admin/server/internal/common"
	"admin/server/model"
	"admin/server/pkg/db"
)

// SysRecordRepository 是登录日志与操作日志的数据访问
type SysRecordRepository struct{}

// NewSysRecordRepository 创建登录日志与操作日志的数据访问
func NewSysRecordRepository() *SysRecordRepository {
	return &SysRecordRepository{}
}

// ==== 登录日志 ====

// loginDB 返回带 sys_login_record 条件的查询
func (r *SysRecordRepository) loginDB() *gorm.DB {
	return db.DB.Model(&model.SysLoginRecord{})
}

// buildLoginQuery 按查询参数拼接条件
func (r *SysRecordRepository) buildLoginQuery(param model.LoginRecordParam) *gorm.DB {
	query := r.loginDB()
	if param.Username != "" {
		query = query.Where("username like ?", "%"+param.Username+"%")
	}
	if param.Nickname != "" {
		query = query.Where("nickname like ?", "%"+param.Nickname+"%")
	}
	if param.IP != "" {
		query = query.Where("ip like ?", "%"+param.IP+"%")
	}
	if param.LoginType != nil {
		query = query.Where("login_type = ?", *param.LoginType)
	}
	if param.CreateTimeStart != "" {
		query = query.Where("create_time >= ?", param.CreateTimeStart)
	}
	if param.CreateTimeEnd != "" {
		query = query.Where("create_time <= ?", param.CreateTimeEnd)
	}
	return query
}

// PageLoginRecords 分页查询登录日志
func (r *SysRecordRepository) PageLoginRecords(param model.LoginRecordParam, page common.PageParam) ([]model.SysLoginRecord, int64, error) {
	query := r.buildLoginQuery(param)

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	var list []model.SysLoginRecord
	query = query.Offset(page.Offset()).Limit(page.PageSize())
	for _, order := range orderClausesWith(page, "id desc") {
		query = query.Order(order)
	}
	if err := query.Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, count, nil
}

// ListLoginRecords 查询登录日志列表
func (r *SysRecordRepository) ListLoginRecords(param model.LoginRecordParam) ([]model.SysLoginRecord, error) {
	var list []model.SysLoginRecord
	err := r.buildLoginQuery(param).Order("id desc").Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

// CreateLoginRecord 新增登录日志
func (r *SysRecordRepository) CreateLoginRecord(record *model.SysLoginRecord) error {
	return db.DB.Create(record).Error
}

// RemoveLoginRecords 按 id 批量删除登录日志
func (r *SysRecordRepository) RemoveLoginRecords(ids []int64) error {
	return db.DB.Where("id in ?", ids).Delete(&model.SysLoginRecord{}).Error
}

// ==== 操作日志 ====

// operationDB 返回带 sys_operation_record 条件的查询
func (r *SysRecordRepository) operationDB() *gorm.DB {
	return db.DB.Model(&model.SysOperationRecord{})
}

// buildOperationQuery 按查询参数拼接条件
func (r *SysRecordRepository) buildOperationQuery(param model.OperationRecordParam) *gorm.DB {
	query := r.operationDB()
	if param.Username != "" {
		query = query.Where("username like ?", "%"+param.Username+"%")
	}
	if param.Nickname != "" {
		query = query.Where("nickname like ?", "%"+param.Nickname+"%")
	}
	if param.Module != "" {
		query = query.Where("module like ?", "%"+param.Module+"%")
	}
	if param.Description != "" {
		query = query.Where("description like ?", "%"+param.Description+"%")
	}
	if param.Status != nil {
		query = query.Where("status = ?", *param.Status)
	}
	if param.CreateTimeStart != "" {
		query = query.Where("create_time >= ?", param.CreateTimeStart)
	}
	if param.CreateTimeEnd != "" {
		query = query.Where("create_time <= ?", param.CreateTimeEnd)
	}
	return query
}

// PageOperationRecords 分页查询操作日志
func (r *SysRecordRepository) PageOperationRecords(param model.OperationRecordParam, page common.PageParam) ([]model.SysOperationRecord, int64, error) {
	query := r.buildOperationQuery(param)

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	var list []model.SysOperationRecord
	query = query.Offset(page.Offset()).Limit(page.PageSize())
	for _, order := range orderClausesWith(page, "id desc") {
		query = query.Order(order)
	}
	if err := query.Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, count, nil
}

// ListOperationRecords 查询操作日志列表
func (r *SysRecordRepository) ListOperationRecords(param model.OperationRecordParam) ([]model.SysOperationRecord, error) {
	var list []model.SysOperationRecord
	err := r.buildOperationQuery(param).Order("id desc").Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

// CreateOperationRecord 新增操作日志
func (r *SysRecordRepository) CreateOperationRecord(record *model.SysOperationRecord) error {
	return db.DB.Create(record).Error
}

// RemoveOperationRecords 按 id 批量删除操作日志
func (r *SysRecordRepository) RemoveOperationRecords(ids []int64) error {
	return db.DB.Where("id in ?", ids).Delete(&model.SysOperationRecord{}).Error
}
