package service

import (
	"errors"
	"fmt"
	"strings"

	"admin/server/internal/kefu/repository"
	"admin/server/model"
)

// 常见的业务错误
var (
	// ErrRouteNotFound 是关联链不存在
	ErrRouteNotFound = repository.ErrRouteNotFound
	// ErrRouteNameRequired 是关联链名称不能为空
	ErrRouteNameRequired = errors.New("关联链名称不能为空")
	// ErrRouteNameExists 是关联链名称已存在
	ErrRouteNameExists = errors.New("关联链名称已经存在")
	// ErrRoutePromptRequired 是未选择提示词模板
	ErrRoutePromptRequired = errors.New("请选择提示词模板")
	// ErrRoutePromptNotFound 是提示词模板不存在或已停用
	ErrRoutePromptNotFound = errors.New("提示词模板已停用，请重新选择")
	// ErrRouteCategoryNotFound 是文档库分类不存在或已停用
	ErrRouteCategoryNotFound = errors.New("文档库分类不存在或已停用")
	// ErrRouteChannelBound 是渠道已被其它关联链绑定
	ErrRouteChannelBound = errors.New("渠道已绑定关联链")
	// ErrRouteChannelNotFound 是渠道不存在
	ErrRouteChannelNotFound = errors.New("渠道不存在")
	// ErrRouteStatusInvalid 是关联链状态不正确
	ErrRouteStatusInvalid = errors.New("关联链状态不正确")
)

// KefuRouteService 是关联链的业务处理
type KefuRouteService struct {
	// 关联链的数据访问
	routeRepository *repository.KefuRouteRepository
	// 提示词模板的数据访问，校验绑定的模板可用
	promptRepository *repository.KefuPromptRepository
	// 文档库分类的数据访问，校验绑定的分类可用
	categoryRepository *repository.KefuKnowledgeCategoryRepository
	// 渠道的数据访问，校验绑定的渠道可用
	channelRepository *repository.KefuChannelRepository
}

// NewKefuRouteService 创建关联链的业务处理
func NewKefuRouteService() *KefuRouteService {
	return &KefuRouteService{
		routeRepository:    repository.NewKefuRouteRepository(),
		promptRepository:   repository.NewKefuPromptRepository(),
		categoryRepository: repository.NewKefuKnowledgeCategoryRepository(),
		channelRepository:  repository.NewKefuChannelRepository(),
	}
}

// Page 分页查询关联链，并回填分类与渠道名称
func (s *KefuRouteService) Page(
	param model.KefuRouteParam,
) ([]model.KefuRoute, int64, error) {
	param.Name = strings.TrimSpace(param.Name)
	list, count, err := s.routeRepository.Page(param, param.PageParam)
	if err != nil {
		return nil, 0, err
	}
	s.fillNames(list)
	return list, count, nil
}

// List 查询启用中的关联链，下拉选择时使用
func (s *KefuRouteService) List() ([]model.KefuRoute, error) {
	list, err := s.routeRepository.List()
	if err != nil {
		return nil, err
	}
	s.fillNames(list)
	return list, nil
}

// Detail 查询关联链详情，回填提示词模板、分类与渠道名称供前端展示
func (s *KefuRouteService) Detail(routeID int64) (*model.KefuRoute, error) {
	record, err := s.routeRepository.GetByID(routeID)
	if err != nil {
		return nil, err
	}
	list := []model.KefuRoute{*record}
	s.fillNames(list)
	return &list[0], nil
}

// Add 新增关联链
func (s *KefuRouteService) Add(form model.KefuRouteForm) error {
	if err := s.validateForm(&form, 0); err != nil {
		return err
	}

	now := model.Now()
	return s.routeRepository.Create(&model.KefuRoute{
		Name:           form.Name,
		PromptID:       form.PromptID,
		PromptOverride: form.PromptOverride,
		Status:         form.Status,
		CreateTime:     now,
		UpdateTime:     now,
	}, form.CategoryIDs, form.ChannelIDs)
}

// Update 修改关联链，分类与渠道关联整体替换
func (s *KefuRouteService) Update(form model.KefuRouteForm) error {
	record, err := s.routeRepository.GetByID(form.RouteID)
	if err != nil {
		return err
	}
	if err := s.validateForm(&form, form.RouteID); err != nil {
		return err
	}

	record.Name = form.Name
	record.PromptID = form.PromptID
	record.PromptOverride = form.PromptOverride
	record.Status = form.Status
	record.UpdateTime = model.Now()
	return s.routeRepository.Update(record, form.CategoryIDs, form.ChannelIDs)
}

// UpdateStatus 修改关联链状态
func (s *KefuRouteService) UpdateStatus(routeID int64, status int) error {
	if _, err := s.routeRepository.GetByID(routeID); err != nil {
		return err
	}
	if status != model.KefuRouteStatusNormal && status != model.KefuRouteStatusDisabled {
		return ErrRouteStatusInvalid
	}
	return s.routeRepository.UpdateStatus(routeID, status)
}

// Remove 删除关联链
func (s *KefuRouteService) Remove(routeID int64) error {
	if _, err := s.routeRepository.GetByID(routeID); err != nil {
		return err
	}
	return s.routeRepository.Remove(routeID)
}

// ByChannelID 按渠道取关联链，给会话快照复用；取不到时返回 (nil, nil)
func (s *KefuRouteService) ByChannelID(channelID int64) (*model.KefuRoute, error) {
	record, err := s.routeRepository.GetByChannelID(channelID)
	if err != nil {
		if errors.Is(err, repository.ErrRouteNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return record, nil
}

// RouteOfSession 取会话快照的关联链，给 AI 编排复用；取不到时返回 (nil, nil)
func (s *KefuRouteService) RouteOfSession(
	session *model.KefuSession,
) (*model.KefuRoute, error) {
	if session == nil || session.RouteID <= 0 {
		return nil, nil
	}
	record, err := s.routeRepository.GetByID(session.RouteID)
	if err != nil {
		if errors.Is(err, repository.ErrRouteNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return record, nil
}

// NamesByIDs 按 id 批量查询关联链名称，会话列表回填 routeName 时使用
func (s *KefuRouteService) NamesByIDs(routeIDs []int64) (map[int64]string, error) {
	return s.routeRepository.ListNamesByIDs(routeIDs)
}

// RouteMapByChannelIDs 批量查询渠道已绑定的关联链，返回 渠道id -> 关联链id
func (s *KefuRouteService) RouteMapByChannelIDs(
	channelIDs []int64,
) (map[int64]int64, error) {
	return s.routeRepository.RouteMapByChannelIDs(channelIDs)
}

// validateForm 校验并归一化关联链的提交数据，routeID 为修改时的自身 id
func (s *KefuRouteService) validateForm(form *model.KefuRouteForm, routeID int64) error {
	form.Name = strings.TrimSpace(form.Name)
	form.PromptOverride = strings.TrimSpace(form.PromptOverride)

	if form.Name == "" {
		return ErrRouteNameRequired
	}
	if form.Status != model.KefuRouteStatusNormal && form.Status != model.KefuRouteStatusDisabled {
		return ErrRouteStatusInvalid
	}

	exists, err := s.routeRepository.ExistsByName(form.Name, routeID)
	if err != nil {
		return err
	}
	if exists {
		return ErrRouteNameExists
	}

	if err := s.checkPrompt(form.PromptID); err != nil {
		return err
	}

	form.CategoryIDs, err = s.checkCategories(form.CategoryIDs)
	if err != nil {
		return err
	}
	form.ChannelIDs, err = s.checkChannels(form.ChannelIDs, routeID)
	return err
}

// checkPrompt 校验绑定的提示词模板存在且启用
func (s *KefuRouteService) checkPrompt(promptID int64) error {
	if promptID <= 0 {
		return ErrRoutePromptRequired
	}
	record, err := s.promptRepository.GetByID(promptID)
	if err != nil {
		if errors.Is(err, repository.ErrPromptNotFound) {
			return ErrRoutePromptNotFound
		}
		return err
	}
	if record.Status != model.KefuPromptStatusNormal {
		return ErrRoutePromptNotFound
	}
	return nil
}

// checkCategories 去重并校验文档库分类存在且启用
func (s *KefuRouteService) checkCategories(categoryIDs []int64) ([]int64, error) {
	list := uniqueIDs(categoryIDs)
	for _, categoryID := range list {
		record, err := s.categoryRepository.GetByID(categoryID)
		if err != nil {
			if errors.Is(err, repository.ErrKnowledgeCategoryNotFound) {
				return nil, ErrRouteCategoryNotFound
			}
			return nil, err
		}
		if record.Status != model.KefuKnowledgeCategoryStatusNormal {
			return nil, ErrRouteCategoryNotFound
		}
	}
	return list, nil
}

// checkChannels 去重并校验渠道存在、未被其它关联链占用
func (s *KefuRouteService) checkChannels(channelIDs []int64, routeID int64) ([]int64, error) {
	list := uniqueIDs(channelIDs)
	for _, channelID := range list {
		channel, err := s.channelRepository.GetByID(channelID)
		if err != nil {
			if errors.Is(err, repository.ErrChannelNotFound) {
				return nil, ErrRouteChannelNotFound
			}
			return nil, err
		}

		boundRouteID, err := s.routeRepository.ChannelBoundRouteID(channelID)
		if err != nil {
			return nil, err
		}
		if boundRouteID != 0 && boundRouteID != routeID {
			return nil, fmt.Errorf("%w: 渠道【%s】已绑定关联链【%s】",
				ErrRouteChannelBound, channel.Name, s.routeName(boundRouteID))
		}
	}
	return list, nil
}

// routeName 返回关联链名称，取不到时返回未知提示
func (s *KefuRouteService) routeName(routeID int64) string {
	record, err := s.routeRepository.GetByID(routeID)
	if err != nil || strings.TrimSpace(record.Name) == "" {
		return "其它关联链"
	}
	return record.Name
}

// fillNames 批量回填提示词模板名称、渠道名称与分类名称
func (s *KefuRouteService) fillNames(list []model.KefuRoute) {
	if len(list) == 0 {
		return
	}

	channelIDs := make([]int64, 0)
	categoryIDs := make([]int64, 0)
	for _, item := range list {
		channelIDs = append(channelIDs, item.ChannelIDs...)
		categoryIDs = append(categoryIDs, item.CategoryIDs...)
	}

	channelNames := make(map[int64]string, len(channelIDs))
	channels, err := s.channelRepository.ListByIDs(uniqueIDs(channelIDs))
	if err != nil {
		channelNames = make(map[int64]string)
	}
	for _, channel := range channels {
		channelNames[channel.ChannelID] = channel.Name
	}

	categoryNames := make(map[int64]string, len(categoryIDs))
	categories, err := s.categoryRepository.ListByIDs(uniqueIDs(categoryIDs))
	if err != nil {
		categoryNames = make(map[int64]string)
	}
	for _, category := range categories {
		categoryNames[category.CategoryID] = category.Name
	}

	for i := range list {
		list[i].PromptName = s.promptName(list[i].PromptID)
		list[i].ChannelNames = namesOf(list[i].ChannelIDs, channelNames)
		list[i].CategoryNames = namesOf(list[i].CategoryIDs, categoryNames)
	}
}

// promptName 返回提示词模板名称，取不到时返回空串
func (s *KefuRouteService) promptName(promptID int64) string {
	if promptID <= 0 {
		return ""
	}
	record, err := s.promptRepository.GetByID(promptID)
	if err != nil {
		return ""
	}
	return record.Name
}

// namesOf 按 id 顺序取出名称，查不到的直接跳过
func namesOf(ids []int64, names map[int64]string) []string {
	list := make([]string, 0, len(ids))
	for _, id := range ids {
		if name, ok := names[id]; ok {
			list = append(list, name)
		}
	}
	return list
}

// uniqueIDs 去掉重复与非法 id，保持原有顺序
func uniqueIDs(ids []int64) []int64 {
	list := make([]int64, 0, len(ids))
	seen := make(map[int64]bool, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		list = append(list, id)
	}
	return list
}
