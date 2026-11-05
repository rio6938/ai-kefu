package service

import (
	"admin/server/internal/kefu/repository"
	"admin/server/model"
)

// KefuMailLogService 是发信记录的业务处理
type KefuMailLogService struct {
	// 邮件收发记录的数据访问
	logRepository *repository.KefuMailLogRepository
	// 邮件出站编排，重发失败邮件
	sendService *KefuMailSendService
}

// NewKefuMailLogService 创建发信记录的业务处理
func NewKefuMailLogService() *KefuMailLogService {
	return &KefuMailLogService{
		logRepository: repository.NewKefuMailLogRepository(),
		sendService:   NewKefuMailSendService(),
	}
}

// Page 分页查询邮件收发记录
func (s *KefuMailLogService) Page(
	param model.KefuMailLogParam,
) ([]model.KefuMailLog, int64, error) {
	return s.logRepository.Page(param, param.PageParam)
}
