package service

import (
	"admin/server/internal/common"
	"admin/server/internal/system/repository"
	"admin/server/model"
)

// SysRecordService 是登录日志与操作日志的业务处理
type SysRecordService struct {
	repo *repository.SysRecordRepository
}

// NewSysRecordService 创建登录日志与操作日志的业务处理
func NewSysRecordService() *SysRecordService {
	return &SysRecordService{repo: repository.NewSysRecordRepository()}
}

// PageLoginRecords 分页查询登录日志
func (s *SysRecordService) PageLoginRecords(param model.LoginRecordParam, page common.PageParam) ([]model.SysLoginRecord, int64, error) {
	return s.repo.PageLoginRecords(param, page)
}

// ListLoginRecords 查询登录日志列表
func (s *SysRecordService) ListLoginRecords(param model.LoginRecordParam) ([]model.SysLoginRecord, error) {
	return s.repo.ListLoginRecords(param)
}

// AddLoginRecord 写入登录日志
func (s *SysRecordService) AddLoginRecord(record *model.SysLoginRecord) error {
	now := model.Now()
	record.CreateTime = now
	record.UpdateTime = now
	return s.repo.CreateLoginRecord(record)
}

// RemoveLoginRecords 按 id 批量删除登录日志
func (s *SysRecordService) RemoveLoginRecords(ids []int64) error {
	return s.repo.RemoveLoginRecords(ids)
}

// PageOperationRecords 分页查询操作日志
func (s *SysRecordService) PageOperationRecords(param model.OperationRecordParam, page common.PageParam) ([]model.SysOperationRecord, int64, error) {
	return s.repo.PageOperationRecords(param, page)
}

// ListOperationRecords 查询操作日志列表
func (s *SysRecordService) ListOperationRecords(param model.OperationRecordParam) ([]model.SysOperationRecord, error) {
	return s.repo.ListOperationRecords(param)
}

// AddOperationRecord 写入操作日志
func (s *SysRecordService) AddOperationRecord(record *model.SysOperationRecord) error {
	if record.CreateTime.IsZero() {
		record.CreateTime = model.Now()
	}
	record.UpdateTime = model.Now()
	return s.repo.CreateOperationRecord(record)
}

// RemoveOperationRecords 按 id 批量删除操作日志
func (s *SysRecordService) RemoveOperationRecords(ids []int64) error {
	return s.repo.RemoveOperationRecords(ids)
}
