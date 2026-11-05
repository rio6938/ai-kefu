package service

import (
	"context"
	"errors"
	"strings"

	aiclient "admin/server/internal/kefu/ai/client"
	"admin/server/internal/kefu/repository"
	"admin/server/model"
)

// 草稿追加在系统提示词后的动作说明，
// 上下文组装与 AI 自动回复完全一致，只多这段草稿标记
const (
	// aiDraftReplyInstruction 是生成回复草稿的动作说明
	aiDraftReplyInstruction = "现在请以在线客服的身份，直接写出可以发给客户的回复内容；只输出回复本身，不要解释，不要加任何前缀。"
	// aiDraftPolishInstruction 是润色客服草稿的动作说明
	aiDraftPolishInstruction = "请润色下面的客服草稿，保持原意与客服口吻，只输出润色后的内容，不要解释。"
	// aiDraftSummaryInstruction 是总结会话的动作说明
	aiDraftSummaryInstruction = "请用简洁的要点总结本次会话：客户的核心诉求、已经给出的答复与待跟进事项，供客服查看。"
)

// 常见的业务错误
var (
	// ErrAiDraftActionInvalid 是草稿生成的动作不正确
	ErrAiDraftActionInvalid = errors.New("AI 生成动作不正确")
	// ErrAiDraftPromptRequired 是润色时没有提交草稿内容
	ErrAiDraftPromptRequired = errors.New("请先输入需要润色的内容")
	// ErrAiDraftMessageNotFound 是草稿触发的消息不存在或不属于该会话
	ErrAiDraftMessageNotFound = repository.ErrMessageNotFound
)

// KefuAiDraftService 是工作台「右键 AI 生成」的编排：
// 复用 AI 自动回复的上下文组装与提示词渲染，
// 但不建 IM 流、不落 kefu_ai_reply
type KefuAiDraftService struct {
	// AI 自动回复的编排，复用上下文组装与提示词渲染
	aiService *KefuAiService
	// 会话消息的数据访问
	messageRepository *repository.KefuMessageRepository
	// 模型配置的数据访问
	modelRepository *repository.KefuModelRepository
}

// NewKefuAiDraftService 创建工作台 AI 草稿的编排
func NewKefuAiDraftService() *KefuAiDraftService {
	return &KefuAiDraftService{
		aiService:         NewKefuAiService(),
		messageRepository: repository.NewKefuMessageRepository(),
		modelRepository:   repository.NewKefuModelRepository(),
	}
}

// Draft 生成 AI 草稿，开始生成前回调 onBegin 回传模型标识，
// 每收到一段增量回调 onDelta，返回的错误都是中文提示
func (s *KefuAiDraftService) Draft(
	ctx context.Context, userID, sessionID int64, form model.KefuAiDraftForm,
	onBegin func(modelName string) error, onDelta func(delta string) error,
) error {
	action := strings.TrimSpace(form.Action)
	if action == "" {
		action = model.KefuAiDraftActionReply
	}
	if !isDraftAction(action) {
		return ErrAiDraftActionInvalid
	}
	promptText := strings.TrimSpace(form.Prompt)
	if action == model.KefuAiDraftActionPolish && promptText == "" {
		return ErrAiDraftPromptRequired
	}

	session, err := s.aiService.sessionService.Operable(userID, sessionID)
	if err != nil {
		return err
	}

	// 触发消息只用于召回知识与标记回复对象，总结会话时可以不传
	var trigger *model.KefuMessage
	if form.MessageID > 0 {
		trigger, err = s.messageRepository.GetByID(form.MessageID)
		if err != nil {
			return err
		}
		if trigger.SessionID != session.SessionID {
			return ErrAiDraftMessageNotFound
		}
	}

	record, err := s.modelRepository.GetDefault()
	if err != nil {
		return err
	}
	options, err := aiclient.FromModel(record)
	if err != nil {
		return err
	}
	chatClient, err := aiclient.New(options)
	if err != nil {
		return err
	}

	promptCtx := s.aiService.renderContext(session)
	// 只有针对客户消息生成回复时才召回知识，润色与摘要不需要
	if action == model.KefuAiDraftActionReply && trigger != nil {
		promptCtx.Knowledge, _ = s.aiService.retrieveKnowledge(ctx, session, trigger)
		promptCtx.Dialogues = s.aiService.dialogueService.Examples(session.RouteID)
	}
	prompt, err := s.aiService.promptService.Render(promptCtx)
	if err != nil {
		return err
	}

	messages := s.aiService.contextMessages(prompt, session.SessionID)
	messages[0].Content = joinPrompt(messages[0].Content,
		draftInstruction(action, trigger, promptText))

	// 草稿只给客服看，不累加进化提示的注入次数
	req := chatClient.Request(messages)
	if err := onBegin(record.Model); err != nil {
		return err
	}
	_, err = chatClient.ChatStream(ctx, req, onDelta)
	return err
}

// isDraftAction 判断草稿生成的动作是否支持
func isDraftAction(action string) bool {
	switch action {
	case model.KefuAiDraftActionReply,
		model.KefuAiDraftActionPolish,
		model.KefuAiDraftActionSummary:
		return true
	default:
		return false
	}
}

// draftInstruction 返回追加在系统提示词后的草稿动作说明，
// 生成回复时明确要针对的那条客户消息，避免历史消息干扰
func draftInstruction(
	action string, trigger *model.KefuMessage, promptText string,
) string {
	switch action {
	case model.KefuAiDraftActionPolish:
		return aiDraftPolishInstruction + "\n\n客服草稿：" + promptText
	case model.KefuAiDraftActionSummary:
		return aiDraftSummaryInstruction
	default:
		if trigger == nil {
			return aiDraftReplyInstruction
		}
		content := strings.TrimSpace(trigger.Content)
		if content == "" {
			return aiDraftReplyInstruction
		}
		return aiDraftReplyInstruction + "\n\n要回复的客户消息：" + content
	}
}
