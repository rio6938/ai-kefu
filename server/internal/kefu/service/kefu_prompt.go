package service

import (
	"errors"
	"strings"
	"time"

	"admin/server/internal/kefu/repository"
	"admin/server/model"
	"admin/server/pkg/setting"
)

// 提示词模板支持的变量名，未知变量渲染时原样保留
var promptVariableKeys = []string{
	"channelName", "visitorName",
	"agentName", "knowledge", "dialogues", "nowTime", "routeName",
}

// 没有任何提示词模板时使用的兜底提示词
const defaultSystemPrompt = "你是{channelName}的在线客服助手，请用简洁、礼貌、准确的中文回答客户的问题。"

// knowledgeHeader 是知识内容注入提示词时的引导语，
// 模板里没有 {knowledge} 占位符时靠它把召回内容补进系统提示词
const knowledgeHeader = "以下是本次回答可用的【知识内容】，请优先依据它作答："

// dialogueHeader 是对话库示例注入提示词时的引导语，
// 模板里没有 {dialogues} 占位符时靠它把示例补进系统提示词
const dialogueHeader = "以下是同类问题的优质对话示例，请参考其中的语气与处理方式作答："

// configKeyAiEscalation 是「答不上来上报转人工」的总开关，关闭后不再注入上报规则
const configKeyAiEscalation = "kefu.ai.escalation_enabled"

// aiEscalationRule 是强制追加的答不上来上报规则：
// 寒暄与追问不作上报，只有确实没有知识依据或无法理解问题才输出标记，
// 由系统换成固定话术并转人工
const aiEscalationRule = "【回答要求】\n" +
	"1. 寒暄、感谢、告别、确认、追问上文的短句（如「你好」「在吗」「谢谢」）直接自然回复，不要上报。\n" +
	"2. 涉及功能、价格、政策、流程、版本等事实性问题，只能依据上面的知识内容作答；" +
	"知识内容里没有依据时禁止猜测、禁止编造、不要给出笼统的通用建议。\n" +
	"3. 只有确实找不到依据、或完全无法理解客户意图时，才只输出 " + aiHandoffMarker +
	" 这个标记，不要输出其它任何文字。"

// 常见的业务错误
var (
	// ErrPromptNotFound 是提示词模板不存在
	ErrPromptNotFound = repository.ErrPromptNotFound
	// ErrPromptNameRequired 是模板名称不能为空
	ErrPromptNameRequired = errors.New("模板名称不能为空")
	// ErrPromptStatusInvalid 是模板状态不正确
	ErrPromptStatusInvalid = errors.New("模板状态不正确")
	// ErrPromptDefaultInvalid 是默认标记不正确
	ErrPromptDefaultInvalid = errors.New("默认标记不正确")
)

// KefuPromptRenderContext 是渲染提示词需要的上下文，
// 知识库与对话库示例由后续能力回填，v1 先传空串
type KefuPromptRenderContext struct {
	// 会话快照的关联链, 提示词模板与覆盖文本从它读取, 可以为空
	Route *model.KefuRoute
	// 关联链名称
	RouteName string
	// 会话, 会话临时提示词从它读取, 可以为空
	Session *model.KefuSession
	// 渠道名称
	ChannelName string
	// 访客名称
	VisitorName string
	// 接待客服名称
	AgentName string
	// 知识库召回内容
	Knowledge string
	// 对话库示例内容
	Dialogues string
}

// KefuRenderedPrompt 是提示词渲染结果
type KefuRenderedPrompt struct {
	// 实际生效的模板id, 0 表示用的是兜底提示词
	PromptID int64
	// 渲染后的系统提示词
	SystemPrompt string
	// 渲染后的角色提示词
	RolePrompt string
	// 完整提示词快照, 落到 kefu_ai_reply.prompt_snapshot
	Content string
}

// KefuPromptService 是提示词模板的业务处理
type KefuPromptService struct {
	// 提示词模板的数据访问
	promptRepository *repository.KefuPromptRepository
}

// NewKefuPromptService 创建提示词模板的业务处理
func NewKefuPromptService() *KefuPromptService {
	return &KefuPromptService{
		promptRepository: repository.NewKefuPromptRepository(),
	}
}

// Page 分页查询提示词模板
func (s *KefuPromptService) Page(
	param model.KefuPromptParam,
) ([]model.KefuPrompt, int64, error) {
	return s.promptRepository.Page(param, param.PageParam)
}

// List 查询启用中的提示词模板
func (s *KefuPromptService) List() ([]model.KefuPrompt, error) {
	return s.promptRepository.List()
}

// Add 新增提示词模板，标记为默认时先取消其它默认模板
func (s *KefuPromptService) Add(form model.KefuPromptForm) error {
	if err := normalizePromptForm(&form); err != nil {
		return err
	}

	if form.IsDefault == model.KefuPromptIsDefault {
		if err := s.promptRepository.ClearDefault(0); err != nil {
			return err
		}
	}

	now := model.Now()
	return s.promptRepository.Create(&model.KefuPrompt{
		Name:         form.Name,
		SystemPrompt: form.SystemPrompt,
		RolePrompt:   form.RolePrompt,
		Variables:    renderPromptVariables(form.SystemPrompt + form.RolePrompt),
		IsDefault:    form.IsDefault,
		Version:      1,
		Status:       form.Status,
		CreateTime:   now,
		UpdateTime:   now,
	})
}

// Update 修改提示词模板，标记为默认时先取消其它默认模板
func (s *KefuPromptService) Update(form model.KefuPromptForm) error {
	record, err := s.promptRepository.GetByID(form.PromptID)
	if err != nil {
		return err
	}
	if err := normalizePromptForm(&form); err != nil {
		return err
	}

	if form.IsDefault == model.KefuPromptIsDefault {
		if err := s.promptRepository.ClearDefault(form.PromptID); err != nil {
			return err
		}
	}

	record.Name = form.Name
	record.SystemPrompt = form.SystemPrompt
	record.RolePrompt = form.RolePrompt
	record.Variables = renderPromptVariables(form.SystemPrompt + form.RolePrompt)
	record.IsDefault = form.IsDefault
	record.Status = form.Status
	record.Version++
	record.UpdateTime = model.Now()
	return s.promptRepository.Update(record)
}

// Remove 删除提示词模板
func (s *KefuPromptService) Remove(promptID int64) error {
	if _, err := s.promptRepository.GetByID(promptID); err != nil {
		return err
	}
	return s.promptRepository.Remove(promptID)
}

// Render 按四层合并渲染提示词：全局默认模板 → 关联链绑定模板 → 关联链覆盖文本 → 会话临时提示词，
// 变量替换后返回完整提示词快照
func (s *KefuPromptService) Render(ctx KefuPromptRenderContext) (*KefuRenderedPrompt, error) {
	global, err := s.defaultPrompt()
	if err != nil {
		return nil, err
	}
	route, err := s.routePrompt(ctx.Route)
	if err != nil {
		return nil, err
	}

	result := &KefuRenderedPrompt{}
	parts := make([]string, 0, 4)
	if global != nil {
		result.PromptID = global.PromptID
		if text := strings.TrimSpace(global.SystemPrompt); text != "" {
			parts = append(parts, text)
		}
	}
	if route != nil {
		result.PromptID = route.PromptID
		if text := strings.TrimSpace(route.SystemPrompt); text != "" {
			parts = append(parts, text)
		}
	}
	if ctx.Route != nil {
		if text := strings.TrimSpace(ctx.Route.PromptOverride); text != "" {
			parts = append(parts, text)
		}
	}
	if ctx.Session != nil {
		if text := strings.TrimSpace(ctx.Session.PromptCustom); text != "" {
			parts = append(parts, text)
		}
	}

	rolePrompt := ""
	if route != nil {
		rolePrompt = route.RolePrompt
	} else if global != nil {
		rolePrompt = global.RolePrompt
	}

	// 对话库示例统一补上引导语，保证 {dialogues} 占位符与自动追加两种写法一致
	if ctx.Dialogues != "" {
		ctx.Dialogues = dialogueHeader + "\n\n" + ctx.Dialogues
	}

	values := promptValues(ctx)
	template := strings.Join(parts, "\n\n")
	systemPrompt := replacePromptVariables(template, values)
	// 没有配置任何模板时用兜底提示词，避免模型没有系统约束
	if systemPrompt == "" {
		systemPrompt = replacePromptVariables(defaultSystemPrompt, values)
	}
	// 系统提示词与角色提示词都没有 {knowledge} 占位符时也要把召回内容补进去，
	// 否则知识库对提示词无效，AI 只能报「没有依据」
	if ctx.Knowledge != "" && !strings.Contains(template+rolePrompt, "{knowledge}") {
		systemPrompt = joinPrompt(systemPrompt, knowledgeHeader+"\n"+ctx.Knowledge)
	}
	// 对话库示例同理，模板没有 {dialogues} 占位符时追加到系统提示词
	if ctx.Dialogues != "" && !strings.Contains(template+rolePrompt, "{dialogues}") {
		systemPrompt = joinPrompt(systemPrompt, ctx.Dialogues)
	}
	// 上报规则与模板无关，开关打开时固定追加在最后，保证优先级最高
	if setting.GetBool(configKeyAiEscalation, true) {
		systemPrompt = joinPrompt(systemPrompt, aiEscalationRule)
	}
	result.SystemPrompt = systemPrompt
	result.RolePrompt = replacePromptVariables(strings.TrimSpace(rolePrompt), values)
	result.Content = joinPrompt(result.SystemPrompt, result.RolePrompt)
	return result, nil
}

// defaultPrompt 查询默认模板，模板不存在时返回 nil
func (s *KefuPromptService) defaultPrompt() (*model.KefuPrompt, error) {
	record, err := s.promptRepository.GetDefault()
	if err != nil {
		if errors.Is(err, repository.ErrPromptNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return record, nil
}

// routePrompt 查询关联链绑定的提示词模板，未绑定或模板已被删除时返回 nil
func (s *KefuPromptService) routePrompt(route *model.KefuRoute) (*model.KefuPrompt, error) {
	if route == nil || route.PromptID <= 0 {
		return nil, nil
	}
	record, err := s.promptRepository.GetByID(route.PromptID)
	if err != nil {
		if errors.Is(err, repository.ErrPromptNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return record, nil
}

// promptValues 返回变量替换表
func promptValues(ctx KefuPromptRenderContext) map[string]string {
	return map[string]string{
		"channelName": ctx.ChannelName,
		"routeName":   ctx.RouteName,
		"visitorName": ctx.VisitorName,
		"agentName":   ctx.AgentName,
		"knowledge":   ctx.Knowledge,
		"dialogues":   ctx.Dialogues,
		"nowTime":     time.Now().Format(model.TimeLayout),
	}
}

// replacePromptVariables 替换模板里的变量，未知变量原样保留
func replacePromptVariables(text string, values map[string]string) string {
	if text == "" {
		return ""
	}
	pairs := make([]string, 0, len(promptVariableKeys)*2)
	for _, key := range promptVariableKeys {
		pairs = append(pairs, "{"+key+"}", values[key])
	}
	return strings.NewReplacer(pairs...).Replace(text)
}

// renderPromptVariables 列出模板里实际用到的变量，存进 variables 字段
func renderPromptVariables(text string) string {
	used := make([]string, 0, len(promptVariableKeys))
	for _, key := range promptVariableKeys {
		if strings.Contains(text, "{"+key+"}") {
			used = append(used, key)
		}
	}
	if len(used) == 0 {
		return "[]"
	}
	return `["` + strings.Join(used, `","`) + `"]`
}

// joinPrompt 拼接系统提示词与角色提示词，其中一段为空时直接返回另一段
func joinPrompt(systemPrompt, rolePrompt string) string {
	parts := make([]string, 0, 2)
	if text := strings.TrimSpace(systemPrompt); text != "" {
		parts = append(parts, text)
	}
	if text := strings.TrimSpace(rolePrompt); text != "" {
		parts = append(parts, text)
	}
	return strings.Join(parts, "\n\n")
}

// normalizePromptForm 校验并归一化提示词模板的提交数据
func normalizePromptForm(form *model.KefuPromptForm) error {
	form.Name = strings.TrimSpace(form.Name)
	form.SystemPrompt = strings.TrimSpace(form.SystemPrompt)
	form.RolePrompt = strings.TrimSpace(form.RolePrompt)

	if form.Name == "" {
		return ErrPromptNameRequired
	}
	if form.Status != model.KefuPromptStatusNormal && form.Status != model.KefuPromptStatusDisabled {
		return ErrPromptStatusInvalid
	}
	if form.IsDefault != model.KefuPromptNotDefault && form.IsDefault != model.KefuPromptIsDefault {
		return ErrPromptDefaultInvalid
	}
	return nil
}
