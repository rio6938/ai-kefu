package service

import (
	"admin/server/internal/common"
	"admin/server/internal/user/repository"
	"admin/server/model"
)

// UserRecordService 是前台用户登录日志与操作日志的业务处理
type UserRecordService struct {
	repo *repository.UserRecordRepository
	// userRepo 用于补充日志里的昵称
	userRepo *repository.UserRepository
}

// NewUserRecordService 创建前台用户日志的业务处理
func NewUserRecordService() *UserRecordService {
	return &UserRecordService{
		repo:     repository.NewUserRecordRepository(),
		userRepo: repository.NewUserRepository(),
	}
}

// PageLoginRecords 分页查询登录日志
func (s *UserRecordService) PageLoginRecords(param model.UserLoginRecordParam, page common.PageParam) ([]model.UserLoginRecord, int64, error) {
	return s.repo.PageLoginRecords(param, page)
}

// ListLoginRecords 查询登录日志列表
func (s *UserRecordService) ListLoginRecords(param model.UserLoginRecordParam) ([]model.UserLoginRecord, error) {
	return s.repo.ListLoginRecords(param)
}

// AddLoginRecord 写入登录日志
func (s *UserRecordService) AddLoginRecord(record *model.UserLoginRecord) error {
	now := model.Now()
	record.CreateTime = now
	record.UpdateTime = now
	return s.repo.CreateLoginRecord(record)
}

// RemoveLoginRecords 按 id 批量删除登录日志
func (s *UserRecordService) RemoveLoginRecords(ids []int64) error {
	return s.repo.RemoveLoginRecords(ids)
}

// PageOperationRecords 分页查询操作日志
func (s *UserRecordService) PageOperationRecords(param model.UserOperationRecordParam, page common.PageParam) ([]model.UserOperationRecord, int64, error) {
	return s.repo.PageOperationRecords(param, page)
}

// ListOperationRecords 查询操作日志列表
func (s *UserRecordService) ListOperationRecords(param model.UserOperationRecordParam) ([]model.UserOperationRecord, error) {
	return s.repo.ListOperationRecords(param)
}

// AddOperationRecord 写入操作日志，昵称为空时按用户id回查补充
func (s *UserRecordService) AddOperationRecord(record *model.UserOperationRecord) error {
	if record.Nickname == "" && record.UserID > 0 {
		if user, err := s.userRepo.GetByID(record.UserID); err == nil {
			record.Nickname = user.Nickname
		}
	}
	now := model.Now()
	if record.CreateTime.IsZero() {
		record.CreateTime = now
	}
	record.UpdateTime = now
	return s.repo.CreateOperationRecord(record)
}

// RemoveOperationRecords 按 id 批量删除操作日志
func (s *UserRecordService) RemoveOperationRecords(ids []int64) error {
	return s.repo.RemoveOperationRecords(ids)
}
