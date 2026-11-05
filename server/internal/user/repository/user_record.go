package repository

import (
	"gorm.io/gorm"

	"admin/server/internal/common"
	"admin/server/model"
	"admin/server/pkg/db"
)

// UserRecordRepository 是前台用户登录日志与操作日志的数据访问
type UserRecordRepository struct{}

// NewUserRecordRepository 创建前台用户日志的数据访问
func NewUserRecordRepository() *UserRecordRepository {
	return &UserRecordRepository{}
}

// ==== 登录日志 ====

// loginDB 返回带 user_login_record 条件的查询
func (r *UserRecordRepository) loginDB() *gorm.DB {
	return db.DB.Model(&model.UserLoginRecord{})
}

// buildLoginQuery 按查询参数拼接条件
func (r *UserRecordRepository) buildLoginQuery(param model.UserLoginRecordParam) *gorm.DB {
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
func (r *UserRecordRepository) PageLoginRecords(param model.UserLoginRecordParam, page common.PageParam) ([]model.UserLoginRecord, int64, error) {
	query := r.buildLoginQuery(param)

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	var list []model.UserLoginRecord
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
func (r *UserRecordRepository) ListLoginRecords(param model.UserLoginRecordParam) ([]model.UserLoginRecord, error) {
	var list []model.UserLoginRecord
	err := r.buildLoginQuery(param).Order("id desc").Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

// CreateLoginRecord 新增登录日志
func (r *UserRecordRepository) CreateLoginRecord(record *model.UserLoginRecord) error {
	return db.DB.Create(record).Error
}

// RemoveLoginRecords 按 id 批量删除登录日志
func (r *UserRecordRepository) RemoveLoginRecords(ids []int64) error {
	return db.DB.Where("id in ?", ids).Delete(&model.UserLoginRecord{}).Error
}

// ==== 操作日志 ====

// operationDB 返回带 user_operation_record 条件的查询
func (r *UserRecordRepository) operationDB() *gorm.DB {
	return db.DB.Model(&model.UserOperationRecord{})
}

// buildOperationQuery 按查询参数拼接条件
func (r *UserRecordRepository) buildOperationQuery(param model.UserOperationRecordParam) *gorm.DB {
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
func (r *UserRecordRepository) PageOperationRecords(param model.UserOperationRecordParam, page common.PageParam) ([]model.UserOperationRecord, int64, error) {
	query := r.buildOperationQuery(param)

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	var list []model.UserOperationRecord
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
func (r *UserRecordRepository) ListOperationRecords(param model.UserOperationRecordParam) ([]model.UserOperationRecord, error) {
	var list []model.UserOperationRecord
	err := r.buildOperationQuery(param).Order("id desc").Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

// CreateOperationRecord 新增操作日志
func (r *UserRecordRepository) CreateOperationRecord(record *model.UserOperationRecord) error {
	return db.DB.Create(record).Error
}

// RemoveOperationRecords 按 id 批量删除操作日志
func (r *UserRecordRepository) RemoveOperationRecords(ids []int64) error {
	return db.DB.Where("id in ?", ids).Delete(&model.UserOperationRecord{}).Error
}
