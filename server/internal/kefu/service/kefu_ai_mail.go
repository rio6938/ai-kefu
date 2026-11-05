package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"go.uber.org/zap"

	aiclient "admin/server/internal/kefu/ai/client"
	"admin/server/model"
	"admin/server/pkg/log"
	"admin/server/pkg/setting"
)

// mailAiInstruction 是邮件生成追加在系统提示词后的要求
const mailAiInstruction = "请用与最近一封客户来信相同的语言回复，只支持中文和英文；" +
	"邮件正文只用纯文本，不要输出 Markdown 语法；不要编造订单、物流、价格与赔付信息；" +
	"回复要一次把问题答完，信息不足时一次问清。"

// prepareGeneration 组装网页与邮件共用的生成要素：模型配置、模型客户端、
// 提示词（含 RAG 命中与对话库示例）与召回结果，网页流式与邮件非流式都走这里
func (s *KefuAiService) prepareGeneration(
	ctx context.Context, session *model.KefuSession, trigger *model.KefuMessage,
) (*model.KefuModel, *aiclient.Client, *KefuRenderedPrompt, []model.KefuKnowledgeHit, error) {
	record, err := s.modelRepository.GetDefault()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	options, err := aiclient.FromModel(record)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	chatClient, err := aiclient.New(options)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	promptCtx := s.renderContext(session)
	var ragHits []model.KefuKnowledgeHit
	if trigger != nil {
		// 知识库召回失败不影响回复，只是没有知识依据
		ragText, hits := s.retrieveKnowledge(ctx, session, trigger)
		promptCtx.Knowledge = ragText
		ragHits = hits
	}
	promptCtx.Dialogues = s.dialogueService.Examples(session.RouteID)
	prompt, err := s.promptService.Render(promptCtx)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return record, chatClient, prompt, ragHits, nil
}

// GenerateMailReply 非流式生成一封邮件回复，复用提示词、RAG、关联链、对话库、
// 敏感词与模型选择，不创建任何 IM 流；生成结果写入 kefu_ai_reply 并返回
func (s *KefuAiService) GenerateMailReply(
	ctx context.Context, session *model.KefuSession, trigger *model.KefuMessage,
	thread *model.KefuMailThread, history []aiclient.Message, query string, language string,
) (*model.KefuAiReply, error) {
	record, chatClient, prompt, ragHits, err := s.prepareGeneration(ctx, session, trigger)
	if err != nil {
		return nil, err
	}

	instruction := mailAiInstruction
	if language == model.KefuMailLanguageEn {
		instruction += " Reply in English."
	} else if language == model.KefuMailLanguageZh {
		instruction += " 用中文回复。"
	}
	messages := make([]aiclient.Message, 0, len(history)+2)
	messages = append(messages, aiclient.Message{
		Role:    aiclient.RoleSystem,
		Content: prompt.Content + "\n\n" + instruction,
	})
	messages = append(messages, history...)
	if strings.TrimSpace(query) != "" {
		messages = append(messages, aiclient.Message{
			Role:    aiclient.RoleUser,
			Content: query,
		})
	}

	timeout := time.Duration(setting.GetInt(configKeyAiTimeoutMs, 30000)) * time.Millisecond
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	genCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	started := time.Now()
	response, err := chatClient.ChatOnce(genCtx, chatClient.Request(messages))
	latency := time.Since(started)
	if err != nil {
		return nil, err
	}

	content := ""
	promptTokens, completionTokens := 0, 0
	if len(response.Choices) > 0 {
		content = strings.TrimSpace(response.Choices[0].Message.Content)
	}
	if response.Usage.TotalTokens > 0 {
		promptTokens = response.Usage.PromptTokens
		completionTokens = response.Usage.CompletionTokens
	}

	sensitiveWords, err := s.dictionaryRepository.ListCodes(sensitiveWordDictCode)
	if err != nil {
		log.Logger.Warn("load kefu sensitive words failed", zap.Error(err))
	}
	if containsSensitiveWord(content, sensitiveWords) {
		return nil, errSensitiveWord
	}
	if content == "" {
		return nil, errors.New("模型没有返回任何回复内容")
	}

	now := model.Now()
	reply := &model.KefuAiReply{
		SessionID:        session.SessionID,
		TriggerMessageID: messageIDOf(trigger),
		ModelID:          record.ModelID,
		Status:           model.KefuAiReplyStatusFinished,
		Content:          content,
		PromptSnapshot:   prompt.Content,
		RagHits:          ragHitsJSON(ragHits),
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		LatencyMs:        int(latency.Milliseconds()),
		Channel:          model.KefuAiReplyChannelMail,
		MailThreadID:     thread.ThreadID,
		Subject:          normalizeMailSubject(thread.Subject, thread.Subject),
		SendStatus:       model.KefuAiReplySendDraft,
		CreateTime:       now,
		UpdateTime:       now,
	}
	if err := s.replyRepository.Create(reply); err != nil {
		return nil, err
	}
	return reply, nil
}

// messageIDOf 返回触发消息的 id，没有触发消息时为 0
func messageIDOf(message *model.KefuMessage) int64 {
	if message == nil {
		return 0
	}
	return message.MessageID
}
