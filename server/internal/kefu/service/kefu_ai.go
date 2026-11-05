package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	oai "github.com/sashabaranov/go-openai"

	aiclient "admin/server/internal/kefu/ai/client"
	"admin/server/internal/kefu/im"
	"admin/server/internal/kefu/repository"
	"admin/server/model"
	"admin/server/pkg/log"
	rdb "admin/server/pkg/redis"
	"admin/server/pkg/setting"
)

// AI 编排使用到的配置项
const (
	// configKeyAiContextRounds 是组装上下文时带上的最近对话轮数
	configKeyAiContextRounds = "kefu.ai.context_rounds"
	// configKeyAiTimeoutMs 是 AI 生成的总超时, 单位毫秒
	configKeyAiTimeoutMs = "kefu.ai.timeout_ms"
	// configKeyAiFirstTokenTimeoutMs 是 AI 首字超时, 单位毫秒
	configKeyAiFirstTokenTimeoutMs = "kefu.ai.first_token_timeout_ms"
)

// AI 编排用到的其它常量
const (
	// sensitiveWordDictCode 是敏感词字典标识
	sensitiveWordDictCode = "kefu_sensitive_word"
	// aiReplyLockKeyPrefix 是会话级 AI 回复锁的 redis key 前缀
	aiReplyLockKeyPrefix = "kefu:ai:reply:lock:"
	// aiReplyLockTTL 是会话锁的过期时间，避免异常退出后 session 一直被锁住
	aiReplyLockTTL = 5 * time.Minute
	// aiFallbackText 是模型超时或报错时给客户的兜底话术
	aiFallbackText = "抱歉，我暂时无法回答这个问题，客服马上为您处理。"
	// aiCannotAnswerText 是模型上报「知识库没有依据或无法理解问题」时给客户的固定话术
	aiCannotAnswerText = "不好意思，暂时回复不了这个问题，正在为您转接人工客服。"
	// aiHandoffMarker 是模型上报答不上来时要求输出的标记，由系统换成人话术并转人工
	aiHandoffMarker = "[[HANDOFF]]"
	// aiSensitiveText 是命中敏感词后给客户的安全话术
	aiSensitiveText = "抱歉，这个问题我需要请人工客服为您解答。"
	// aiInterruptByAgent 是客服中断生成的来源标识
	aiInterruptByAgent = "agent_interrupt"
	// aiInterruptBySensitive 是敏感词中断生成的来源标识
	aiInterruptBySensitive = "sensitive_word"
	// aiInterruptByHandoff 是客户要求转人工时中断生成的来源标识
	aiInterruptByHandoff = "handoff"
	// aiInterruptByIM 是流式基础消息发送失败的来源标识
	aiInterruptByIM = "im_error"
)

// 常见的业务错误
var (
	// ErrAiDisabled 是会话没有开启 AI 自动回复
	ErrAiDisabled = errors.New("会话没有开启 AI 自动回复")
	// ErrAiModeInvalid 是 AI 开关取值不正确
	ErrAiModeInvalid = errors.New("AI 自动回复开关取值不正确")
	// errAgentInterrupt 是客服中断了本次生成
	errAgentInterrupt = errors.New("AI 生成已被客服中断")
	// errSensitiveWord 是生成内容命中敏感词
	errSensitiveWord = errors.New("AI 生成内容命中敏感词")
)

// KefuAiService 是 AI 自动回复的编排：
// 组装上下文 → 流式生成 → 写 IM 流式事件 → 落库与兜底
type KefuAiService struct {
	// 会话的数据访问
	sessionRepository *repository.KefuSessionRepository
	// 会话消息的数据访问
	messageRepository *repository.KefuMessageRepository
	// AI 回复记录的数据访问
	replyRepository *repository.KefuAiReplyRepository
	// 模型配置的数据访问
	modelRepository *repository.KefuModelRepository
	// 字典的数据访问，读取敏感词
	dictionaryRepository *repository.KefuDictionaryRepository
	// 客服档案的数据访问
	agentRepository *repository.KefuAgentRepository
	// 渠道的数据访问
	channelRepository *repository.KefuChannelRepository
	// 访客的数据访问
	visitorRepository *repository.KefuVisitorRepository
	// 提示词模板的业务处理
	promptService *KefuPromptService
	// 会话的业务处理，复用会话归属校验
	sessionService *KefuSessionService
	// 知识库的业务处理，组装上下文时召回知识
	knowledgeService *KefuKnowledgeService
	// 分配的业务处理，关闭 AI 转待接待时写系统消息
	assignService *KefuAssignService
	// 关联链的业务处理，按会话快照取提示词与检索范围
	routeService *KefuRouteService
	// 对话库的业务处理，取优质对话示例注入提示词
	dialogueService *KefuDialogueService
	// 转人工的业务处理，AI 答不上来时把会话转给人工
	handoffService *KefuHandoffService
}

// NewKefuAiService 创建 AI 自动回复的编排
func NewKefuAiService() *KefuAiService {
	return &KefuAiService{
		sessionRepository:    repository.NewKefuSessionRepository(),
		messageRepository:    repository.NewKefuMessageRepository(),
		replyRepository:      repository.NewKefuAiReplyRepository(),
		modelRepository:      repository.NewKefuModelRepository(),
		dictionaryRepository: repository.NewKefuDictionaryRepository(),
		agentRepository:      repository.NewKefuAgentRepository(),
		channelRepository:    repository.NewKefuChannelRepository(),
		visitorRepository:    repository.NewKefuVisitorRepository(),
		promptService:        NewKefuPromptService(),
		sessionService:       NewKefuSessionService(),
		knowledgeService:     NewKefuKnowledgeService(),
		assignService:        NewKefuAssignService(),
		routeService:         NewKefuRouteService(),
		dialogueService:      NewKefuDialogueService(),
		handoffService:       NewKefuHandoffService(),
	}
}

// Reply 为一条客户消息编排出 AI 自动回复；
// 会话不是 AI 进行中、关闭 AI 或全局总闸关闭时直接跳过，同一会话的回复串行执行
func (s *KefuAiService) Reply(ctx context.Context, sessionID, messageID int64) error {
	session, err := s.sessionRepository.GetByID(sessionID)
	if err != nil {
		return err
	}
	if session.Status != model.KefuSessionStatusAi || session.AiMode != model.KefuSessionAiModeOn {
		return ErrAiDisabled
	}
	if !setting.GetBool(configKeyAiEnabled, true) {
		return ErrAiDisabled
	}

	unlock, ok := s.tryLock(ctx, session.SessionID)
	if !ok {
		return nil
	}
	if unlock != nil {
		defer unlock()
	}

	// 拿到锁后重新读一次会话，避免等锁期间 AI 被客服关掉或已转人工后还在回复
	session, err = s.sessionRepository.GetByID(sessionID)
	if err != nil {
		return err
	}
	if session.Status != model.KefuSessionStatusAi || session.AiMode != model.KefuSessionAiModeOn {
		return ErrAiDisabled
	}
	return s.reply(ctx, session, messageID)
}

// reply 执行一次完整的 AI 回复，任何路径都会以 stream.finish 收尾
func (s *KefuAiService) reply(ctx context.Context, session *model.KefuSession, messageID int64) error {
	trigger, err := s.messageRepository.GetByID(messageID)
	if err != nil {
		return err
	}

	record, chatClient, prompt, ragHits, err := s.prepareGeneration(ctx, session, trigger)
	if err != nil {
		log.Logger.Error("skip kefu ai reply, prepare generation failed",
			zap.Int64("sessionId", session.SessionID), zap.Error(err))
		return err
	}

	now := model.Now()
	reply := &model.KefuAiReply{
		SessionID:        session.SessionID,
		TriggerMessageID: trigger.MessageID,
		ModelID:          record.ModelID,
		Status:           model.KefuAiReplyStatusStreaming,
		PromptSnapshot:   prompt.Content,
		RagHits:          ragHitsJSON(ragHits),
		CreateTime:       now,
		UpdateTime:       now,
	}
	if err := s.replyRepository.Create(reply); err != nil {
		return err
	}
	reply.ClientMsgNo = fmt.Sprintf("reply_%d", reply.ReplyID)
	if err := s.replyRepository.UpdateFields(reply.ReplyID, map[string]any{
		"client_msg_no": reply.ClientMsgNo,
	}); err != nil {
		log.Logger.Warn("update kefu ai reply clientMsgNo failed",
			zap.Int64("replyId", reply.ReplyID), zap.Error(err))
	}

	stream := im.NewStreamReply(ctx, session.SessionID, reply.ReplyID)
	if _, err := stream.Begin(); err != nil {
		log.Logger.Error("begin kefu ai stream failed",
			zap.Int64("sessionId", session.SessionID), zap.Int64("replyId", reply.ReplyID), zap.Error(err))
		if updateErr := s.replyRepository.UpdateFields(reply.ReplyID, map[string]any{
			"status":       model.KefuAiReplyStatusError,
			"interrupt_by": aiInterruptByIM,
		}); updateErr != nil {
			log.Logger.Warn("mark kefu ai reply error failed",
				zap.Int64("replyId", reply.ReplyID), zap.Error(updateErr))
		}
		return err
	}

	started := time.Now()
	// 兜底：任何路径（含 panic）都必须以 stream.finish 收敛，Finish 自身幂等
	var finishText string
	defer func() {
		if err := stream.Finish(finishText); err != nil {
			log.Logger.Error("finish kefu ai stream failed",
				zap.Int64("replyId", reply.ReplyID), zap.Error(err))
		}
	}()

	result := s.generate(ctx, stream, chatClient,
		chatClient.Request(s.contextMessages(prompt, session.SessionID)))

	status := model.KefuAiReplyStatusFinished
	interruptBy := ""
	text := strings.TrimSpace(result.content)
	switch {
	case result.err == nil:
		// 正常结束，不需要额外收尾事件
	case errors.Is(result.err, errAgentInterrupt):
		status = model.KefuAiReplyStatusInterrupted
		interruptBy = aiInterruptByAgent
		if err := stream.Cancel(interruptBy); err != nil {
			log.Logger.Warn("cancel kefu ai stream failed",
				zap.Int64("replyId", reply.ReplyID), zap.Error(err))
		}
		// 客户要求转人工时丢弃已生成的半截回复，由系统消息回复客户
		if im.InterruptReason(ctx, reply.ReplyID) == aiInterruptByHandoff {
			interruptBy = aiInterruptByHandoff
			text = ""
			if err := stream.Discard(); err != nil {
				log.Logger.Warn("discard kefu ai stream failed",
					zap.Int64("replyId", reply.ReplyID), zap.Error(err))
			}
		}
	case errors.Is(result.err, errSensitiveWord):
		status = model.KefuAiReplyStatusError
		interruptBy = aiInterruptBySensitive
		text = aiSensitiveText
		if err := stream.Cancel(interruptBy); err != nil {
			log.Logger.Warn("cancel kefu ai stream failed",
				zap.Int64("replyId", reply.ReplyID), zap.Error(err))
		}
	default:
		status = model.KefuAiReplyStatusError
		text = aiFallbackText
		if err := stream.Error("ai_failed"); err != nil {
			log.Logger.Warn("write kefu ai stream error failed",
				zap.Int64("replyId", reply.ReplyID), zap.Error(err))
		}
		log.Logger.Error("kefu ai generate failed",
			zap.Int64("sessionId", session.SessionID),
			zap.Int64("replyId", reply.ReplyID), zap.Error(result.err))
	}
	// 模型按提示词要求上报「答不上来」时，换固定话术回复，并把会话转给人工；
	// 开关关闭后不做替换，模型输出什么就发什么
	escalated := false
	if setting.GetBool(configKeyAiEscalation, true) && strings.Contains(text, aiHandoffMarker) {
		text = aiCannotAnswerText
		escalated = true
	}
	// 模型没有产出任何文本时也要给客户一句话，避免留下空气泡；
	// 客户要求转人工时由系统消息回复，这里不再补兜底话术
	if text == "" && interruptBy != aiInterruptByHandoff {
		text = aiFallbackText
	}
	finishText = text
	if err := stream.Finish(text); err != nil {
		log.Logger.Error("finish kefu ai stream failed",
			zap.Int64("replyId", reply.ReplyID), zap.Error(err))
	}

	s.saveResult(session, reply, result, status, interruptBy, text, time.Since(started))
	// AI 答不上来时把会话转给人工，与固定话术保持一致
	if escalated {
		s.handoffService.HandoffByAi(ctx, session)
	}
	return nil
}

// aiGenerateResult 是一次流式生成的结果
type aiGenerateResult struct {
	// 已经产出的文本
	content string
	// token 用量, 上游未返回时按字符数估算
	usage *aiclient.Usage
	// 首字耗时, 毫秒
	firstTokenMs int
	// 中断、敏感词或模型错误, nil 表示正常结束
	err error
}

// generate 调用模型流式生成，并在每个增量前检查中断标记与敏感词；
// 首字超时与总超时都会取消上游请求，再按兜底话术收尾
func (s *KefuAiService) generate(
	ctx context.Context, stream *im.StreamReply, chatClient *aiclient.Client,
	req oai.ChatCompletionRequest,
) *aiGenerateResult {
	result := new(aiGenerateResult)

	timeout := time.Duration(setting.GetInt(configKeyAiTimeoutMs, 30000)) * time.Millisecond
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	firstTokenTimeout := time.Duration(setting.GetInt(configKeyAiFirstTokenTimeoutMs, 8000)) * time.Millisecond
	if firstTokenTimeout <= 0 {
		firstTokenTimeout = 8 * time.Second
	}

	genCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 首字超时后主动取消上游请求，生成结束时按超时处理
	var firstTokenTimedOut atomic.Bool
	firstTimer := time.AfterFunc(firstTokenTimeout, func() {
		firstTokenTimedOut.Store(true)
		cancel()
	})
	defer firstTimer.Stop()

	sensitiveWords, err := s.dictionaryRepository.ListCodes(sensitiveWordDictCode)
	if err != nil {
		log.Logger.Warn("load kefu sensitive words failed", zap.Error(err))
	}

	var builder strings.Builder
	started := time.Now()
	usage, genErr := chatClient.ChatStream(genCtx, req, func(delta string) error {
		if result.firstTokenMs == 0 {
			firstTimer.Stop()
			result.firstTokenMs = int(time.Since(started).Milliseconds())
		}
		if stream.Interrupted() {
			cancel()
			return errAgentInterrupt
		}
		if containsSensitiveWord(builder.String()+delta, sensitiveWords) {
			cancel()
			return errSensitiveWord
		}
		if err := stream.Delta(delta); err != nil {
			cancel()
			return err
		}
		builder.WriteString(delta)
		return nil
	})

	result.content = builder.String()
	result.usage = usage
	result.err = genErr
	// 首字超时走的也是上下文取消，这里还原成超时便于区分兜底原因
	if firstTokenTimedOut.Load() && errors.Is(result.err, aiclient.ErrInterrupted) {
		result.err = aiclient.ErrTimeout
	}
	return result
}

// saveResult 把本次生成结果落到 kefu_ai_reply 与 kefu_message；
// 中断与失败都会给会话打上 ai_failed 标记，工作台据此提醒人工接管；
// 客户要求转人工时只记录中断结果，不再写 AI 消息，由系统消息回复客户
func (s *KefuAiService) saveResult(
	session *model.KefuSession, reply *model.KefuAiReply, result *aiGenerateResult,
	status, interruptBy, text string, latency time.Duration,
) {
	fields := map[string]any{
		"status":         status,
		"interrupt_by":   interruptBy,
		"content":        text,
		"first_token_ms": result.firstTokenMs,
		"latency_ms":     int(latency.Milliseconds()),
	}
	if result.usage != nil {
		fields["prompt_tokens"] = result.usage.PromptTokens
		fields["completion_tokens"] = result.usage.CompletionTokens
	}
	if err := s.replyRepository.UpdateFields(reply.ReplyID, fields); err != nil {
		log.Logger.Error("update kefu ai reply failed",
			zap.Int64("replyId", reply.ReplyID), zap.Error(err))
	}
	if interruptBy == aiInterruptByHandoff {
		return
	}

	now := model.Now()
	message := &model.KefuMessage{
		SessionID:   session.SessionID,
		ClientMsgNo: reply.ClientMsgNo,
		FromRole:    model.KefuMessageFromAi,
		FromID:      im.AiUID(),
		ContentType: model.KefuMessageTypeAiAnswer,
		Content:     text,
		AiGenerated: 1,
		AiReplyID:   reply.ReplyID,
		CreateTime:  now,
		UpdateTime:  now,
	}
	payload := &im.Payload{
		Version:   1,
		Type:      im.PayloadTypeAiAnswer,
		Role:      im.PayloadRoleAi,
		Content:   text,
		SessionID: session.SessionID,
		Meta: map[string]any{
			"replyId": reply.ReplyID,
		},
	}
	if raw, err := json.Marshal(payload); err == nil {
		message.Payload = string(raw)
	}

	inserted, err := s.messageRepository.CreateIfAbsent(message)
	if err != nil {
		log.Logger.Error("save kefu ai message failed",
			zap.Int64("replyId", reply.ReplyID), zap.Error(err))
	} else if inserted {
		// AI 回复也计入未读，客服没有开着该会话时靠未读气泡提醒
		if err := s.sessionRepository.UpdateMessageStat(session.SessionID, now, 1); err != nil {
			log.Logger.Warn("update kefu session stat failed",
				zap.Int64("sessionId", session.SessionID), zap.Error(err))
		}
	}

	// 只有超时、上游报错或命中敏感词才算 AI 回复失败，
	// 客服主动中断不算失败，不点亮工作台的待接管提醒
	if status == model.KefuAiReplyStatusError {
		if err := s.sessionRepository.UpdateFields(session.SessionID,
			map[string]any{"ai_failed": 1}); err != nil {
			log.Logger.Warn("mark kefu session ai failed",
				zap.Int64("sessionId", session.SessionID), zap.Error(err))
		}
	}
}

// SetAiMode 修改会话的 AI 自动回复开关，关闭时中断进行中的 AI 回复；
// AI 进行中的会话关掉 AI 后没有人工接待，同时转成待接待等客服领取
func (s *KefuAiService) SetAiMode(
	ctx context.Context, userID int64, form model.KefuSessionAiModeForm,
) (*model.KefuAiInterruptResult, error) {
	if form.AiMode != model.KefuSessionAiModeOn && form.AiMode != model.KefuSessionAiModeOff {
		return nil, ErrAiModeInvalid
	}
	session, err := s.sessionService.Operable(userID, form.SessionID)
	if err != nil {
		return nil, err
	}

	fields := map[string]any{"ai_mode": form.AiMode}
	aiQueueing := form.AiMode == model.KefuSessionAiModeOff &&
		session.Status == model.KefuSessionStatusAi
	if aiQueueing {
		fields["status"] = model.KefuSessionStatusQueueing
		fields["queue_time"] = model.Now()
	}
	if err := s.sessionRepository.UpdateFields(session.SessionID, fields); err != nil {
		return nil, err
	}

	result := &model.KefuAiInterruptResult{}
	if form.AiMode == model.KefuSessionAiModeOff {
		if err := s.replyRepository.CancelPendingBySession(session.SessionID); err != nil {
			log.Logger.Warn("cancel kefu pending mail failed",
				zap.Int64("sessionId", session.SessionID), zap.Error(err))
		}
		replyID, err := s.interrupt(ctx, session.SessionID)
		if err != nil {
			return nil, err
		}
		result.ReplyID = replyID
		if aiQueueing {
			if err := s.assignService.sendSystemMessage(ctx, session.SessionID,
				handoffNoticeText); err != nil {
				log.Logger.Warn("send kefu handoff message failed",
					zap.Int64("sessionId", session.SessionID), zap.Error(err))
			}
		}
	}
	return result, nil
}

// Interrupt 中断会话进行中的 AI 回复，返回被中断的回复id
func (s *KefuAiService) Interrupt(
	ctx context.Context, userID int64, sessionID int64,
) (*model.KefuAiInterruptResult, error) {
	session, err := s.sessionService.Operable(userID, sessionID)
	if err != nil {
		return nil, err
	}
	replyID, err := s.interrupt(ctx, session.SessionID)
	if err != nil {
		return nil, err
	}
	return &model.KefuAiInterruptResult{ReplyID: replyID}, nil
}

// interrupt 给进行中的 AI 回复置中断标记，没有进行中的回复时返回 0
func (s *KefuAiService) interrupt(ctx context.Context, sessionID int64) (int64, error) {
	reply, err := s.replyRepository.GetStreamingBySession(sessionID)
	if err != nil {
		if errors.Is(err, repository.ErrAiReplyNotFound) {
			return 0, nil
		}
		return 0, err
	}
	if err := im.MarkInterrupt(ctx, reply.ReplyID, aiInterruptByAgent); err != nil {
		return reply.ReplyID, err
	}
	return reply.ReplyID, nil
}

// Prompt 返回会话当前渲染后的提示词与最近一次实际使用的快照，供工作台只读查看
func (s *KefuAiService) Prompt(userID int64, sessionID int64) (*model.KefuSessionAiPrompt, error) {
	session, err := s.sessionService.Operable(userID, sessionID)
	if err != nil {
		return nil, err
	}
	promptCtx := s.renderContext(session)
	// 预览与实际回复保持一致，把对话库示例一并渲染进去
	promptCtx.Dialogues = s.dialogueService.Examples(session.RouteID)
	rendered, err := s.promptService.Render(promptCtx)
	if err != nil {
		return nil, err
	}

	result := &model.KefuSessionAiPrompt{
		SessionID:    session.SessionID,
		PromptCustom: session.PromptCustom,
		Prompt:       rendered.Content,
	}
	latest, err := s.replyRepository.GetLatestBySession(session.SessionID)
	if err != nil {
		if errors.Is(err, repository.ErrAiReplyNotFound) {
			return result, nil
		}
		return nil, err
	}
	result.PromptSnapshot = latest.PromptSnapshot
	result.LastReplyID = latest.ReplyID
	return result, nil
}

// SavePrompt 保存会话临时提示词，只对本会话生效
func (s *KefuAiService) SavePrompt(userID int64, form model.KefuSessionAiPromptForm) error {
	session, err := s.sessionService.Operable(userID, form.SessionID)
	if err != nil {
		return err
	}
	return s.sessionRepository.UpdateFields(session.SessionID, map[string]any{
		"prompt_custom": strings.TrimSpace(form.PromptCustom),
	})
}

// contextMessages 组装对话上下文：系统提示词 + 最近 N 轮客户与客服/AI 的对话
func (s *KefuAiService) contextMessages(
	prompt *KefuRenderedPrompt, sessionID int64,
) []aiclient.Message {
	rounds := setting.GetInt(configKeyAiContextRounds, 10)
	if rounds <= 0 {
		rounds = 10
	}

	messages := []aiclient.Message{{
		Role:    aiclient.RoleSystem,
		Content: prompt.Content,
	}}
	history, err := s.messageRepository.ListRecent(sessionID, rounds*2)
	if err != nil {
		log.Logger.Warn("load kefu session history failed",
			zap.Int64("sessionId", sessionID), zap.Error(err))
		return messages
	}
	for _, item := range history {
		content := strings.TrimSpace(item.Content)
		if content == "" || item.ContentType == model.KefuMessageTypeSystem {
			continue
		}
		role := ""
		switch item.FromRole {
		case model.KefuMessageFromVisitor:
			role = aiclient.RoleUser
		case model.KefuMessageFromAi, model.KefuMessageFromAgent:
			role = aiclient.RoleAssistant
		default:
			continue
		}
		messages = append(messages, aiclient.Message{Role: role, Content: content})
	}
	return messages
}

// renderContext 组装渲染提示词需要的上下文，
// session.RouteID 大于 0 时取会话快照的关联链，取不到时留着空走全局默认模板
func (s *KefuAiService) renderContext(session *model.KefuSession) KefuPromptRenderContext {
	ctx := KefuPromptRenderContext{
		Session: session,
	}
	if route, err := s.routeService.RouteOfSession(session); err != nil {
		log.Logger.Warn("load kefu session route failed",
			zap.Int64("sessionId", session.SessionID), zap.Error(err))
	} else if route != nil {
		ctx.Route = route
		ctx.RouteName = route.Name
	}
	if channel, err := s.channelRepository.GetByCode(session.ChannelCode); err == nil {
		ctx.ChannelName = channel.Name
	}
	if visitor, err := s.visitorRepository.GetByID(session.VisitorID); err == nil {
		ctx.VisitorName = visitor.Nickname
	}
	ctx.AgentName = s.agentName(session.AgentID)
	return ctx
}

// retrieveKnowledge 按会话快照的关联链取文档库分类，只在这些分类内召回；
// 关联链没有绑定分类时不检索（D4），失败只记日志
func (s *KefuAiService) retrieveKnowledge(
	ctx context.Context, session *model.KefuSession, trigger *model.KefuMessage,
) (string, []model.KefuKnowledgeHit) {
	query := strings.TrimSpace(trigger.Content)
	if query == "" {
		return "", nil
	}

	route, err := s.routeService.RouteOfSession(session)
	if err != nil {
		log.Logger.Warn("load kefu session route failed",
			zap.Int64("sessionId", session.SessionID), zap.Error(err))
		return "", nil
	}
	if route == nil || len(route.CategoryIDs) == 0 {
		log.Logger.Debug("skip kefu knowledge retrieve, route has no category",
			zap.Int64("sessionId", session.SessionID), zap.Int64("routeId", session.RouteID))
		return "", nil
	}

	hits, err := s.knowledgeService.Search(ctx, model.KefuKnowledgeSearchForm{
		CategoryIDs: route.CategoryIDs,
		Query:       query,
	})
	if err != nil {
		log.Logger.Debug("retrieve kefu knowledge skipped",
			zap.Int64("sessionId", session.SessionID), zap.Error(err))
		return "", nil
	}
	return knowledgeText(hits), hits
}

// ragHitsJSON 把召回命中序列化成 kefu_ai_reply.rag_hits，没有命中时存空数组
func ragHitsJSON(hits []model.KefuKnowledgeHit) string {
	if len(hits) == 0 {
		return "[]"
	}
	raw, err := json.Marshal(hits)
	if err != nil {
		log.Logger.Warn("marshal kefu rag hits failed", zap.Error(err))
		return "[]"
	}
	return string(raw)
}

// agentName 返回接待客服的名称，取不到时返回空串
func (s *KefuAiService) agentName(agentID int64) string {
	if agentID <= 0 {
		return ""
	}
	agent, err := s.agentRepository.GetByID(agentID)
	if err != nil {
		log.Logger.Warn("load kefu agent failed",
			zap.Int64("agentId", agentID), zap.Error(err))
		return ""
	}
	users, err := s.agentRepository.ListUsersByIDs([]int64{agent.UserID})
	if err != nil || len(users) == 0 {
		return ""
	}
	if nickname := strings.TrimSpace(users[0].Nickname); nickname != "" {
		return nickname
	}
	return users[0].Username
}

// tryLock 获取会话级的 AI 回复锁，保证同一会话的回复串行执行
func (s *KefuAiService) tryLock(ctx context.Context, sessionID int64) (func(), bool) {
	if rdb.Client == nil {
		return nil, true
	}
	key := fmt.Sprintf("%s%d", aiReplyLockKeyPrefix, sessionID)
	ok, err := rdb.Client.SetNX(ctx, key, "1", aiReplyLockTTL).Result()
	if err != nil {
		log.Logger.Warn("lock kefu ai reply failed",
			zap.Int64("sessionId", sessionID), zap.Error(err))
		return nil, true
	}
	if !ok {
		log.Logger.Debug("skip kefu ai reply, another reply is running",
			zap.Int64("sessionId", sessionID))
		return nil, false
	}
	return func() {
		if err := rdb.Client.Del(context.Background(), key).Err(); err != nil {
			log.Logger.Warn("unlock kefu ai reply failed",
				zap.Int64("sessionId", sessionID), zap.Error(err))
		}
	}, true
}

// containsSensitiveWord 判断文本是否命中敏感词，v1 只做简单的包含匹配
func containsSensitiveWord(text string, words []string) bool {
	if text == "" {
		return false
	}
	for _, word := range words {
		word = strings.TrimSpace(word)
		if word == "" {
			continue
		}
		if strings.Contains(text, word) {
			return true
		}
	}
	return false
}
