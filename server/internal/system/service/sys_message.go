package service

import (
	"errors"
	"strings"

	"admin/server/internal/system/repository"
	"admin/server/model"
)

// 常见的业务错误
var (
	// ErrMessageTypeInvalid 是消息类型不正确
	ErrMessageTypeInvalid = errors.New("消息类型不正确")
	// ErrMessageIDsRequired 是消息 id 不能为空
	ErrMessageIDsRequired = errors.New("请选择要操作的消息")
	// ErrMessageStatusInvalid 是消息状态不正确
	ErrMessageStatusInvalid = errors.New("消息状态不正确")
)

// SysMessageService 是我的消息的业务处理
type SysMessageService struct {
	repo *repository.SysMessageRepository
}

// NewSysMessageService 创建我的消息的业务处理
func NewSysMessageService() *SysMessageService {
	return &SysMessageService{repo: repository.NewSysMessageRepository()}
}

// Page 分页查询我的消息
func (s *SysMessageService) Page(userID int64, param model.MessageParam) ([]model.SysMessage, int64, error) {
	if param.Type != nil &&
		*param.Type != model.MessageTypeNotice &&
		*param.Type != model.MessageTypeTodo {
		return nil, 0, ErrMessageTypeInvalid
	}
	param.Title = strings.TrimSpace(param.Title)
	return s.repo.Page(userID, param, param.PageParam)
}

// UnreadNum 查询我的消息未读数量
func (s *SysMessageService) UnreadNum(userID int64) (model.MessageUnreadNum, error) {
	noticeCount, todoCount, err := s.repo.CountUnread(userID)
	if err != nil {
		return model.MessageUnreadNum{}, err
	}
	return model.MessageUnreadNum{Notice: noticeCount, Todo: todoCount}, nil
}

// UpdateStatus 修改我的消息状态，通知用于标记已读，待办用于标记已完成
func (s *SysMessageService) UpdateStatus(userID int64, form model.MessageStatusForm) error {
	if len(form.IDs) == 0 {
		return ErrMessageIDsRequired
	}
	if form.Status == nil ||
		(*form.Status != model.MessageStatusUnread && *form.Status != model.MessageStatusRead) {
		return ErrMessageStatusInvalid
	}
	return s.repo.UpdateStatus(userID, form.IDs, *form.Status)
}

// Remove 按 id 批量删除我的消息
func (s *SysMessageService) Remove(userID int64, ids []int64) error {
	if len(ids) == 0 {
		return ErrMessageIDsRequired
	}
	return s.repo.Remove(userID, ids)
}
