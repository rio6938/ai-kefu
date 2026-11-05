package service

import (
	"errors"
	"strings"

	"admin/server/internal/kefu/repository"
	"admin/server/model"
)

// 常见的业务错误
var (
	// ErrQuickReplyNotFound 是快捷回复不存在
	ErrQuickReplyNotFound = repository.ErrQuickReplyNotFound
	// ErrQuickReplyScopeInvalid 是适用范围不正确
	ErrQuickReplyScopeInvalid = errors.New("适用范围不正确")
	// ErrQuickReplyTitleRequired 是标题不能为空
	ErrQuickReplyTitleRequired = errors.New("标题不能为空")
	// ErrQuickReplyContentRequired 是内容不能为空
	ErrQuickReplyContentRequired = errors.New("内容不能为空")
	// ErrQuickReplyTitleExists 是同一适用范围下标题已存在
	ErrQuickReplyTitleExists = errors.New("该适用范围下标题已经存在")
	// ErrQuickReplyStatusInvalid 是状态不正确
	ErrQuickReplyStatusInvalid = errors.New("快捷回复状态不正确")
)

// KefuQuickReplyService 是快捷回复的业务处理
type KefuQuickReplyService struct {
	// 快捷回复的数据访问
	quickReplyRepository *repository.KefuQuickReplyRepository
	// 渠道的业务处理，访客快捷问题先校验收到的渠道可用
	channelService *KefuChannelService
}

// NewKefuQuickReplyService 创建快捷回复的业务处理
func NewKefuQuickReplyService() *KefuQuickReplyService {
	return &KefuQuickReplyService{
		quickReplyRepository: repository.NewKefuQuickReplyRepository(),
		channelService:       NewKefuChannelService(),
	}
}

// Page 分页查询快捷回复
func (s *KefuQuickReplyService) Page(
	param model.KefuQuickReplyParam,
) ([]model.KefuQuickReply, int64, error) {
	param.Scope = strings.TrimSpace(param.Scope)
	param.Keyword = strings.TrimSpace(param.Keyword)
	return s.quickReplyRepository.Page(param, param.PageParam)
}

// Add 新增快捷回复
func (s *KefuQuickReplyService) Add(form model.KefuQuickReplyForm) error {
	if err := normalizeQuickReplyForm(&form); err != nil {
		return err
	}

	exists, err := s.quickReplyRepository.ExistsTitle(form.Scope, form.Title, 0)
	if err != nil {
		return err
	}
	if exists {
		return ErrQuickReplyTitleExists
	}

	now := model.Now()
	return s.quickReplyRepository.Create(&model.KefuQuickReply{
		Scope:      form.Scope,
		Title:      form.Title,
		Content:    form.Content,
		GroupCode:  form.GroupCode,
		SortNumber: form.SortNumber,
		Status:     form.Status,
		CreateTime: now,
		UpdateTime: now,
	})
}

// Update 修改快捷回复
func (s *KefuQuickReplyService) Update(form model.KefuQuickReplyForm) error {
	record, err := s.quickReplyRepository.GetByID(form.QuickReplyID)
	if err != nil {
		return err
	}
	if err := normalizeQuickReplyForm(&form); err != nil {
		return err
	}

	exists, err := s.quickReplyRepository.ExistsTitle(
		form.Scope, form.Title, form.QuickReplyID)
	if err != nil {
		return err
	}
	if exists {
		return ErrQuickReplyTitleExists
	}

	record.Scope = form.Scope
	record.Title = form.Title
	record.Content = form.Content
	record.GroupCode = form.GroupCode
	record.SortNumber = form.SortNumber
	record.Status = form.Status
	record.UpdateTime = model.Now()
	return s.quickReplyRepository.Update(record)
}

// UpdateStatus 修改快捷回复状态
func (s *KefuQuickReplyService) UpdateStatus(quickReplyID int64, status int) error {
	if _, err := s.quickReplyRepository.GetByID(quickReplyID); err != nil {
		return err
	}
	if status != model.KefuQuickReplyStatusNormal &&
		status != model.KefuQuickReplyStatusDisabled {
		return ErrQuickReplyStatusInvalid
	}
	return s.quickReplyRepository.UpdateStatus(quickReplyID, status)
}

// Remove 删除快捷回复
func (s *KefuQuickReplyService) Remove(quickReplyID int64) error {
	if _, err := s.quickReplyRepository.GetByID(quickReplyID); err != nil {
		return err
	}
	return s.quickReplyRepository.Remove(quickReplyID)
}

// MarkUsed 累加快捷回复的使用次数，客服插入话术后调用
func (s *KefuQuickReplyService) MarkUsed(quickReplyID int64) error {
	if _, err := s.quickReplyRepository.GetByID(quickReplyID); err != nil {
		return err
	}
	return s.quickReplyRepository.IncrementUseCount(quickReplyID)
}

// VisitorList 返回渠道可用的访客快捷问题，渠道不可用时返回错误、停用的不返回
func (s *KefuQuickReplyService) VisitorList(
	channelCode string,
) ([]model.KefuQuickReplyItem, error) {
	if _, err := s.channelService.getEnabled(channelCode); err != nil {
		return nil, err
	}
	list, err := s.quickReplyRepository.ListEnabled(model.KefuQuickReplyScopeVisitor)
	if err != nil {
		return nil, err
	}
	return toQuickReplyItems(list), nil
}

// AgentList 返回客服可用的快捷话术
func (s *KefuQuickReplyService) AgentList() ([]model.KefuQuickReply, error) {
	return s.quickReplyRepository.ListEnabled(model.KefuQuickReplyScopeAgent)
}

// toQuickReplyItems 裁剪聊天窗只需要展示的字段
func toQuickReplyItems(list []model.KefuQuickReply) []model.KefuQuickReplyItem {
	items := make([]model.KefuQuickReplyItem, 0, len(list))
	for _, item := range list {
		items = append(items, model.KefuQuickReplyItem{
			QuickReplyID: item.QuickReplyID,
			Title:        item.Title,
			Content:      item.Content,
			GroupCode:    item.GroupCode,
		})
	}
	return items
}

// normalizeQuickReplyForm 校验并归一化快捷回复的提交数据
func normalizeQuickReplyForm(form *model.KefuQuickReplyForm) error {
	form.Scope = strings.TrimSpace(form.Scope)
	form.Title = strings.TrimSpace(form.Title)
	form.Content = strings.TrimSpace(form.Content)
	form.GroupCode = strings.TrimSpace(form.GroupCode)
	if runes := []rune(form.GroupCode); len(runes) > 64 {
		form.GroupCode = string(runes[:64])
	}

	if form.Scope != model.KefuQuickReplyScopeVisitor &&
		form.Scope != model.KefuQuickReplyScopeAgent {
		return ErrQuickReplyScopeInvalid
	}
	if form.Title == "" {
		return ErrQuickReplyTitleRequired
	}
	if form.Content == "" {
		return ErrQuickReplyContentRequired
	}
	return nil
}
