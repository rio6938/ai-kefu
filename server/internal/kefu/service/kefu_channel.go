package service

import (
	"encoding/json"
	"errors"
	"strings"

	"admin/server/internal/kefu/repository"
	"admin/server/model"
	"admin/server/pkg/setting"
)

// configKeyChatURL 是聊天窗部署地址的配置项，公开配置里下发给悬浮脚本
const configKeyChatURL = "kefu.web.chat_url"

// configKeyHandoffVisitorEnabled 是访客端「转人工」按钮的开关
const configKeyHandoffVisitorEnabled = "kefu.handoff.visitor_enabled"

// configKeyRateEnabled 是会话评价的开关
const configKeyRateEnabled = "kefu.rate.enabled"

// 常见的业务错误
var (
	// ErrChannelNotFound 是渠道不存在
	ErrChannelNotFound = repository.ErrChannelNotFound
	// ErrChannelDisabled 是渠道已停用
	ErrChannelDisabled = errors.New("渠道已停用")
	// ErrChannelCodeRequired 是渠道标识不能为空
	ErrChannelCodeRequired = errors.New("渠道标识不能为空")
	// ErrChannelNameRequired 是渠道名称不能为空
	ErrChannelNameRequired = errors.New("渠道名称不能为空")
	// ErrChannelCodeExists 是渠道标识已存在
	ErrChannelCodeExists = errors.New("渠道标识已经存在")
	// ErrChannelConfigInvalid 是扩展配置不是合法的 json
	ErrChannelConfigInvalid = errors.New("扩展配置必须是合法的 JSON")
	// ErrChannelStatusInvalid 是渠道状态不正确
	ErrChannelStatusInvalid = errors.New("渠道状态不正确")
)

// KefuChannelService 是渠道的业务处理
type KefuChannelService struct {
	// 渠道的数据访问
	channelRepository *repository.KefuChannelRepository
	// 关联链的业务处理，列表回填渠道已绑定的关联链
	routeService *KefuRouteService
	// 邮箱配置的数据访问，渠道列表回填邮箱地址
	mailboxRepository *repository.KefuMailboxRepository
	// 邮箱配置的业务处理，渠道改类型时停用邮箱
	mailboxService *KefuMailboxService
}

// NewKefuChannelService 创建渠道的业务处理
func NewKefuChannelService() *KefuChannelService {
	return &KefuChannelService{
		channelRepository: repository.NewKefuChannelRepository(),
		routeService:      NewKefuRouteService(),
		mailboxRepository: repository.NewKefuMailboxRepository(),
		mailboxService:    NewKefuMailboxService(),
	}
}

// Page 分页查询渠道，并回填关联链 id 与名称
func (s *KefuChannelService) Page(param model.KefuChannelParam) ([]model.KefuChannel, int64, error) {
	list, count, err := s.channelRepository.Page(param, param.PageParam)
	if err != nil {
		return nil, 0, err
	}
	s.fillRouteInfo(list)
	s.fillMailboxInfo(list)
	return list, count, nil
}

// List 查询全部渠道，供关联链编辑弹窗多选渠道
func (s *KefuChannelService) List() ([]model.KefuChannelItem, error) {
	list, err := s.channelRepository.List()
	if err != nil {
		return nil, err
	}
	s.fillRouteInfo(list)
	return toChannelItems(list), nil
}

// toChannelItems 裁剪关联链编辑弹窗需要的渠道字段
func toChannelItems(list []model.KefuChannel) []model.KefuChannelItem {
	items := make([]model.KefuChannelItem, 0, len(list))
	for _, item := range list {
		items = append(items, model.KefuChannelItem{
			ChannelID:   item.ChannelID,
			ChannelCode: item.ChannelCode,
			ChannelType: item.ChannelType,
			Name:        item.Name,
			RouteID:     item.RouteID,
		})
	}
	return items
}

// fillRouteInfo 批量回填渠道已绑定的关联链 id 与名称，未绑定时为 0 与空串
func (s *KefuChannelService) fillRouteInfo(list []model.KefuChannel) {
	if len(list) == 0 {
		return
	}

	channelIDs := make([]int64, 0, len(list))
	for _, channel := range list {
		channelIDs = append(channelIDs, channel.ChannelID)
	}
	routeMap, err := s.routeService.RouteMapByChannelIDs(channelIDs)
	if err != nil {
		return
	}
	routeIDs := make([]int64, 0, len(routeMap))
	for _, routeID := range routeMap {
		routeIDs = append(routeIDs, routeID)
	}
	names, err := s.routeService.NamesByIDs(routeIDs)
	if err != nil {
		return
	}
	for i := range list {
		routeID := routeMap[list[i].ChannelID]
		list[i].RouteID = routeID
		if routeID > 0 {
			list[i].RouteName = names[routeID]
		}
	}
}

// Add 新增渠道
func (s *KefuChannelService) Add(form model.KefuChannelForm) error {
	if err := normalizeChannelForm(&form); err != nil {
		return err
	}

	exists, err := s.channelRepository.ExistsByCode(form.ChannelCode, 0)
	if err != nil {
		return err
	}
	if exists {
		return ErrChannelCodeExists
	}

	now := model.Now()
	return s.channelRepository.Create(&model.KefuChannel{
		ChannelCode: form.ChannelCode,
		ChannelType: form.ChannelType,
		Name:        form.Name,
		WelcomeText: form.WelcomeText,
		ThemeColor:  form.ThemeColor,
		NeedLogin:   form.NeedLogin,
		Config:      form.Config,
		Status:      model.KefuChannelStatusNormal,
		CreateTime:  now,
		UpdateTime:  now,
	})
}

// Update 修改渠道
func (s *KefuChannelService) Update(form model.KefuChannelForm) error {
	channel, err := s.channelRepository.GetByID(form.ChannelID)
	if err != nil {
		return err
	}
	if err := normalizeChannelForm(&form); err != nil {
		return err
	}

	exists, err := s.channelRepository.ExistsByCode(form.ChannelCode, form.ChannelID)
	if err != nil {
		return err
	}
	if exists {
		return ErrChannelCodeExists
	}

	channel.ChannelCode = form.ChannelCode
	channel.ChannelType = form.ChannelType
	channel.Name = form.Name
	channel.WelcomeText = form.WelcomeText
	channel.ThemeColor = form.ThemeColor
	channel.NeedLogin = form.NeedLogin
	channel.Config = form.Config
	channel.UpdateTime = model.Now()
	if err := s.channelRepository.Update(channel); err != nil {
		return err
	}
	// 改成其它类型时保留邮箱配置但置停用，避免误删凭据与历史
	if channel.ChannelType != model.KefuChannelTypeEmail {
		s.mailboxService.DisableByChannelID(channel.ChannelID)
	}
	return nil
}

// UpdateStatus 修改渠道状态
func (s *KefuChannelService) UpdateStatus(channelID int64, status int) error {
	if _, err := s.channelRepository.GetByID(channelID); err != nil {
		return err
	}
	if status != model.KefuChannelStatusNormal && status != model.KefuChannelStatusDisabled {
		return ErrChannelStatusInvalid
	}
	return s.channelRepository.UpdateStatus(channelID, status)
}

// Config 返回渠道对外的公开配置，只下发聊天窗需要的字段；
// 附件限额一并下发，便于聊天窗在选文件前就做前端校验
func (s *KefuChannelService) Config(channelCode string) (*model.KefuChannelConfig, error) {
	channel, err := s.getEnabled(channelCode)
	if err != nil {
		return nil, err
	}
	return &model.KefuChannelConfig{
		ChannelCode:    channel.ChannelCode,
		ChannelType:    channel.ChannelType,
		Name:           channel.Name,
		WelcomeText:    channel.WelcomeText,
		ThemeColor:     channel.ThemeColor,
		NeedLogin:      channel.NeedLogin,
		ChatURL:        strings.TrimSpace(setting.GetString(configKeyChatURL, "")),
		FileEnabled:    fileEnabled(),
		FileMaxSizeMB:  fileMaxSizeMB(),
		FileAllowedExt: fileAllowedExt(),
		HandoffEnabled: handoffVisitorEnabled(),
		RateEnabled:    rateEnabled(),
	}, nil
}

// fillMailboxInfo 批量回填邮箱渠道的邮箱地址，非邮箱渠道为空
func (s *KefuChannelService) fillMailboxInfo(list []model.KefuChannel) {
	if len(list) == 0 {
		return
	}
	channelIDs := make([]int64, 0, len(list))
	for _, channel := range list {
		if channel.ChannelType == model.KefuChannelTypeEmail {
			channelIDs = append(channelIDs, channel.ChannelID)
		}
	}
	if len(channelIDs) == 0 {
		return
	}
	mailboxes, err := s.mailboxRepository.ListByChannelIDs(channelIDs)
	if err != nil {
		return
	}
	mailboxMap := make(map[int64]string, len(mailboxes))
	for _, mailbox := range mailboxes {
		mailboxMap[mailbox.ChannelID] = mailbox.EmailAddress
	}
	for i := range list {
		list[i].MailboxEmail = mailboxMap[list[i].ChannelID]
	}
}

// handoffVisitorEnabled 返回访客端「转人工」按钮的开关
func handoffVisitorEnabled() bool {
	return setting.GetBool(configKeyHandoffVisitorEnabled, true)
}

// rateEnabled 返回会话评价的开关
func rateEnabled() bool {
	return setting.GetBool(configKeyRateEnabled, true)
}

// getEnabled 查询未停用的渠道，访客接入与公开配置都要求渠道可用
func (s *KefuChannelService) getEnabled(channelCode string) (*model.KefuChannel, error) {
	channel, err := s.channelRepository.GetByCode(strings.TrimSpace(channelCode))
	if err != nil {
		return nil, err
	}
	if channel.Status != model.KefuChannelStatusNormal {
		return nil, ErrChannelDisabled
	}
	return channel, nil
}

// normalizeChannelForm 校验并归一化渠道的提交数据
func normalizeChannelForm(form *model.KefuChannelForm) error {
	form.ChannelCode = strings.TrimSpace(form.ChannelCode)
	form.ChannelType = strings.TrimSpace(form.ChannelType)
	form.Name = strings.TrimSpace(form.Name)
	form.WelcomeText = strings.TrimSpace(form.WelcomeText)
	form.ThemeColor = strings.TrimSpace(form.ThemeColor)
	form.Config = strings.TrimSpace(form.Config)

	if form.ChannelCode == "" {
		return ErrChannelCodeRequired
	}
	if form.Name == "" {
		return ErrChannelNameRequired
	}
	// v1 只实现网页渠道，未指定类型时按 web 处理
	if form.ChannelType == "" {
		form.ChannelType = model.KefuChannelTypeWeb
	}
	if form.Config == "" {
		form.Config = "{}"
	}
	if !json.Valid([]byte(form.Config)) {
		return ErrChannelConfigInvalid
	}
	return nil
}
