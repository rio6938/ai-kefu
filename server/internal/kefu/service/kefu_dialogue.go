package service

import (
	"errors"
	"strings"

	"go.uber.org/zap"

	"admin/server/internal/common"
	"admin/server/internal/kefu/repository"
	"admin/server/model"
	"admin/server/pkg/log"
	"admin/server/pkg/setting"
)

// 对话库使用到的配置项
const (
	// configKeyDialogueLimit 是每次喂给 AI 的对话示例条数上限
	configKeyDialogueLimit = "kefu.dialogue.limit"
)

// 对话库使用到的常量
const (
	// dialogueVisitorPrefix 是示例里客户一方的标识
	dialogueVisitorPrefix = "客户："
	// dialogueAgentPrefix 是示例里客服一方的标识
	dialogueAgentPrefix = "客服："
	// dialogueDraftMessageLimit 是一次导入最多读取的会话消息条数
	dialogueDraftMessageLimit = 1000
)

// 常见的业务错误
var (
	// ErrDialogueNotFound 是对话库条目不存在
	ErrDialogueNotFound = repository.ErrDialogueNotFound
	// ErrDialogueTitleRequired 是对话标题不能为空
	ErrDialogueTitleRequired = errors.New("对话标题不能为空")
	// ErrDialogueTitleExists 是对话标题已经存在
	ErrDialogueTitleExists = errors.New("对话标题已经存在")
	// ErrDialogueMessagesRequired 是未录入对话内容
	ErrDialogueMessagesRequired = errors.New("请至少录入一句对话")
	// ErrDialogueMessageRoleInvalid 是对话角色不正确
	ErrDialogueMessageRoleInvalid = errors.New("对话角色不正确")
	// ErrDialogueMessageContentRequired 是对话内容不能为空
	ErrDialogueMessageContentRequired = errors.New("对话内容不能为空")
	// ErrDialogueRouteNotFound 是关联链不存在或已停用
	ErrDialogueRouteNotFound = errors.New("关联链不存在或已停用")
	// ErrDialogueStatusInvalid 是对话库条目状态不正确
	ErrDialogueStatusInvalid = errors.New("对话状态不正确")
)

// KefuDialogueService 是对话库的业务处理
type KefuDialogueService struct {
	// 对话库的数据访问
	dialogueRepository *repository.KefuDialogueRepository
	// 关联链的数据访问，校验绑定的关联链可用
	routeRepository *repository.KefuRouteRepository
	// 会话的业务处理，从会话记录导入时复用会话归属校验
	sessionService *KefuSessionService
	// 会话消息的数据访问，导入时读取会话消息
	messageRepository *repository.KefuMessageRepository
}

// NewKefuDialogueService 创建对话库的业务处理
func NewKefuDialogueService() *KefuDialogueService {
	return &KefuDialogueService{
		dialogueRepository: repository.NewKefuDialogueRepository(),
		routeRepository:    repository.NewKefuRouteRepository(),
		sessionService:     NewKefuSessionService(),
		messageRepository:  repository.NewKefuMessageRepository(),
	}
}

// Page 分页查询对话库，并回填关联链名称
func (s *KefuDialogueService) Page(
	param model.KefuDialogueParam,
) ([]model.KefuDialogue, int64, error) {
	param.Title = strings.TrimSpace(param.Title)
	param.Source = strings.TrimSpace(param.Source)
	list, count, err := s.dialogueRepository.Page(param, param.PageParam)
	if err != nil {
		return nil, 0, err
	}
	s.fillRouteNames(list)
	return list, count, nil
}

// Detail 查询对话库条目详情，含完整对话内容
func (s *KefuDialogueService) Detail(dialogueID int64) (*model.KefuDialogue, error) {
	record, err := s.dialogueRepository.GetByID(dialogueID)
	if err != nil {
		return nil, err
	}
	list := []model.KefuDialogue{*record}
	s.fillRouteNames(list)
	return &list[0], nil
}

// Add 新增对话库条目
func (s *KefuDialogueService) Add(form model.KefuDialogueForm) error {
	messages, err := s.validateForm(&form, 0)
	if err != nil {
		return err
	}

	now := model.Now()
	record := &model.KefuDialogue{
		Title:           form.Title,
		Question:        form.Question,
		RouteID:         form.RouteID,
		Source:          model.KefuDialogueSourceManual,
		SourceSessionID: form.SourceSessionID,
		Status:          form.Status,
		CreateTime:      now,
		UpdateTime:      now,
	}
	if form.SourceSessionID > 0 {
		record.Source = model.KefuDialogueSourceSession
	}
	return s.dialogueRepository.Create(record, messages)
}

// Update 修改对话库条目，对话内容整体替换
func (s *KefuDialogueService) Update(form model.KefuDialogueForm) error {
	record, err := s.dialogueRepository.GetByID(form.DialogueID)
	if err != nil {
		return err
	}
	messages, err := s.validateForm(&form, form.DialogueID)
	if err != nil {
		return err
	}

	record.Title = form.Title
	record.Question = form.Question
	record.RouteID = form.RouteID
	record.Status = form.Status
	record.UpdateTime = model.Now()
	return s.dialogueRepository.Update(record, messages)
}

// UpdateStatus 修改对话库条目的状态
func (s *KefuDialogueService) UpdateStatus(dialogueID int64, status int) error {
	if _, err := s.dialogueRepository.GetByID(dialogueID); err != nil {
		return err
	}
	if status != model.KefuDialogueStatusNormal &&
		status != model.KefuDialogueStatusDisabled {
		return ErrDialogueStatusInvalid
	}
	return s.dialogueRepository.UpdateStatus(dialogueID, status)
}

// Remove 删除对话库条目
func (s *KefuDialogueService) Remove(dialogueID int64) error {
	if _, err := s.dialogueRepository.GetByID(dialogueID); err != nil {
		return err
	}
	return s.dialogueRepository.Remove(dialogueID)
}

// SessionDraft 把一次会话的消息整理成对话库草稿：
// 只保留文本消息，角色统一成客户与客服，导入后由运营在原对话上修改再保存
func (s *KefuDialogueService) SessionDraft(
	userID int64, sessionID int64,
) (*model.KefuDialogueForm, error) {
	session, err := s.sessionService.ReadableSession(userID, sessionID)
	if err != nil {
		return nil, err
	}

	messages, _, err := s.messageRepository.ListBySession(sessionID,
		model.KefuMessageParam{PageParam: common.PageParam{Page: 1, Limit: dialogueDraftMessageLimit}})
	if err != nil {
		return nil, err
	}

	draft := &model.KefuDialogueForm{
		Title:           session.SessionNo,
		RouteID:         session.RouteID,
		SourceSessionID: session.SessionID,
		Status:          model.KefuDialogueStatusNormal,
		Messages:        make([]model.KefuDialogueMessageForm, 0, len(messages)),
	}
	for _, item := range messages {
		content := strings.TrimSpace(item.Content)
		if content == "" || item.ContentType != model.KefuMessageTypeText {
			continue
		}
		role := ""
		switch item.FromRole {
		case model.KefuMessageFromVisitor:
			role = model.KefuDialogueRoleVisitor
		case model.KefuMessageFromAgent, model.KefuMessageFromAi:
			role = model.KefuDialogueRoleAgent
		default:
			continue
		}
		if draft.Question == "" && role == model.KefuDialogueRoleVisitor {
			draft.Question = content
		}
		draft.Messages = append(draft.Messages, model.KefuDialogueMessageForm{
			Role:    role,
			Content: content,
		})
	}
	if len(draft.Messages) == 0 {
		return nil, ErrDialogueMessagesRequired
	}
	return draft, nil
}

// Examples 返回可以喂给 AI 的对话示例文本；
// 绑定该关联链的示例优先，其次是通用示例，没有示例时返回空串
func (s *KefuDialogueService) Examples(routeID int64) string {
	limit := setting.GetInt(configKeyDialogueLimit, 2)
	if limit <= 0 {
		return ""
	}

	list, err := s.dialogueRepository.ListExamples(routeID, limit)
	if err != nil {
		log.Logger.Warn("load kefu dialogue examples failed",
			zap.Int64("routeId", routeID), zap.Error(err))
		return ""
	}

	blocks := make([]string, 0, len(list))
	for _, item := range list {
		lines := make([]string, 0, len(item.Messages))
		for _, message := range item.Messages {
			content := strings.TrimSpace(message.Content)
			if content == "" {
				continue
			}
			prefix := dialogueVisitorPrefix
			if message.Role == model.KefuDialogueRoleAgent {
				prefix = dialogueAgentPrefix
			}
			lines = append(lines, prefix+content)
		}
		if len(lines) == 0 {
			continue
		}
		blocks = append(blocks, strings.Join(lines, "\n"))
	}
	if len(blocks) == 0 {
		return ""
	}
	return strings.Join(blocks, "\n\n")
}

// validateForm 校验并归一化对话库的提交数据，返回待写入的对话内容
func (s *KefuDialogueService) validateForm(
	form *model.KefuDialogueForm, dialogueID int64,
) ([]model.KefuDialogueMessage, error) {
	form.Title = strings.TrimSpace(form.Title)
	form.Question = strings.TrimSpace(form.Question)

	if form.Title == "" {
		return nil, ErrDialogueTitleRequired
	}
	if form.Status != model.KefuDialogueStatusNormal &&
		form.Status != model.KefuDialogueStatusDisabled {
		return nil, ErrDialogueStatusInvalid
	}

	exists, err := s.dialogueRepository.ExistsByTitle(form.Title, dialogueID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrDialogueTitleExists
	}

	if err := s.checkRoute(form.RouteID); err != nil {
		return nil, err
	}

	list := make([]model.KefuDialogueMessage, 0, len(form.Messages))
	for _, item := range form.Messages {
		role := strings.TrimSpace(item.Role)
		content := strings.TrimSpace(item.Content)
		if role != model.KefuDialogueRoleVisitor &&
			role != model.KefuDialogueRoleAgent {
			return nil, ErrDialogueMessageRoleInvalid
		}
		if content == "" {
			return nil, ErrDialogueMessageContentRequired
		}
		list = append(list, model.KefuDialogueMessage{
			Role:    role,
			Content: content,
		})
	}
	if len(list) == 0 {
		return nil, ErrDialogueMessagesRequired
	}
	return list, nil
}

// checkRoute 校验绑定的关联链存在且启用，0 表示不绑定
func (s *KefuDialogueService) checkRoute(routeID int64) error {
	if routeID <= 0 {
		return nil
	}
	record, err := s.routeRepository.GetByID(routeID)
	if err != nil {
		if errors.Is(err, repository.ErrRouteNotFound) {
			return ErrDialogueRouteNotFound
		}
		return err
	}
	if record.Status != model.KefuRouteStatusNormal {
		return ErrDialogueRouteNotFound
	}
	return nil
}

// fillRouteNames 批量回填绑定的关联链名称
func (s *KefuDialogueService) fillRouteNames(list []model.KefuDialogue) {
	if len(list) == 0 {
		return
	}

	routeIDs := make([]int64, 0, len(list))
	for _, item := range list {
		if item.RouteID > 0 {
			routeIDs = append(routeIDs, item.RouteID)
		}
	}
	names, err := s.routeRepository.ListNamesByIDs(uniqueIDs(routeIDs))
	if err != nil {
		names = make(map[int64]string)
	}
	for i := range list {
		if name, ok := names[list[i].RouteID]; ok {
			list[i].RouteName = name
		}
	}
}
