package service

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"

	oai "github.com/sashabaranov/go-openai"

	aiclient "admin/server/internal/kefu/ai/client"
	"admin/server/internal/kefu/openai"
	"admin/server/internal/kefu/repository"
	"admin/server/model"
	"admin/server/pkg/log"
)

// KefuOpenAIService 是 OpenAI 兼容接口的业务处理，
// 复用内部模型客户端，不通过 HTTP 自请求
type KefuOpenAIService struct {
	// 模型配置的数据访问
	modelRepository *repository.KefuModelRepository
}

// NewKefuOpenAIService 创建 OpenAI 兼容接口的业务处理
func NewKefuOpenAIService() *KefuOpenAIService {
	return &KefuOpenAIService{
		modelRepository: repository.NewKefuModelRepository(),
	}
}

// Models 返回启用中的模型列表，供 GET /v1/models 使用
func (s *KefuOpenAIService) Models() ([]openai.Model, error) {
	records, err := s.modelRepository.ListEnabled()
	if err != nil {
		return nil, err
	}

	list := make([]openai.Model, 0, len(records))
	for _, record := range records {
		list = append(list, openai.Model{
			ID:      modelIdentifier(&record),
			Object:  openai.ObjectModel,
			Created: record.CreateTime.Unix(),
			OwnedBy: record.Provider,
		})
	}
	return list, nil
}

// ChatCompletion 非流式对话，返回按 OpenAI 规范组织的完整响应
func (s *KefuOpenAIService) ChatCompletion(
	ctx context.Context, req openai.ChatCompletionRequest,
) (*openai.ChatCompletionResponse, error) {
	client, record, err := s.newClient(req.Model)
	if err != nil {
		return nil, err
	}

	chatReq := client.Request(convertMessages(req.Messages))
	applyRequestOption(&chatReq, req)
	resp, err := client.ChatOnce(ctx, chatReq)
	if err != nil {
		return nil, mapModelError(err)
	}

	content := ""
	if len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
	}
	return &openai.ChatCompletionResponse{
		ID:      openai.NewCompletionID(),
		Object:  openai.ObjectChatCompletion,
		Created: time.Now().Unix(),
		Model:   record.Model,
		Choices: []openai.Choice{{
			Index:        0,
			Message:      openai.Message{Role: aiclient.RoleAssistant, Content: content},
			FinishReason: "stop",
		}},
		Usage: convertUsage(resp.Usage, convertMessages(req.Messages), content),
	}, nil
}

// ChatCompletionStream 流式对话，每产出一个分片就回调 onChunk；
// 上下文取消后上游请求会在模型客户端内部同步中止
func (s *KefuOpenAIService) ChatCompletionStream(
	ctx context.Context,
	req openai.ChatCompletionRequest,
	onChunk func(chunk openai.ChatCompletionChunk) error,
) error {
	client, record, err := s.newClient(req.Model)
	if err != nil {
		return err
	}

	chatReq := client.Request(convertMessages(req.Messages))
	applyRequestOption(&chatReq, req)

	id := openai.NewCompletionID()
	created := time.Now().Unix()
	// 首个分片只带角色，与 OpenAI 的流式响应保持一致
	if err := onChunk(openai.ChatCompletionChunk{
		ID:      id,
		Object:  openai.ObjectChatCompletionChunk,
		Created: created,
		Model:   record.Model,
		Choices: []openai.ChunkChoice{{
			Index: 0,
			Delta: openai.Delta{Role: aiclient.RoleAssistant, Content: ""},
		}},
	}); err != nil {
		return err
	}

	usage, err := client.ChatStream(ctx, chatReq, func(delta string) error {
		return onChunk(openai.ChatCompletionChunk{
			ID:      id,
			Object:  openai.ObjectChatCompletionChunk,
			Created: created,
			Model:   record.Model,
			Choices: []openai.ChunkChoice{{
				Index: 0,
				Delta: openai.Delta{Content: delta},
			}},
		})
	})
	if err != nil {
		return mapModelError(err)
	}

	// 结束分片带 finish_reason 与 token 用量
	finishReason := "stop"
	return onChunk(openai.ChatCompletionChunk{
		ID:      id,
		Object:  openai.ObjectChatCompletionChunk,
		Created: created,
		Model:   record.Model,
		Choices: []openai.ChunkChoice{{
			Index:        0,
			Delta:        openai.Delta{},
			FinishReason: &finishReason,
		}},
		Usage: &openai.Usage{
			PromptTokens:     usage.PromptTokens,
			CompletionTokens: usage.CompletionTokens,
			TotalTokens:      usage.TotalTokens,
			Estimated:        usage.Estimated,
		},
	})
}

// newClient 按请求里的模型标识取模型配置并创建客户端，
// 未指定或不认识时用默认模型，配置不完整返回 503
func (s *KefuOpenAIService) newClient(modelName string) (*aiclient.Client, *model.KefuModel, error) {
	record, err := s.resolveModel(modelName)
	if err != nil {
		return nil, nil, err
	}

	options, err := aiclient.FromModel(record)
	if err != nil {
		log.Logger.Warn("kefu model config incomplete",
			zap.Int64("modelId", record.ModelID), zap.Error(err))
		return nil, nil, openai.NewAPIError(http.StatusServiceUnavailable,
			openai.ErrorTypeServer, openai.CodeModelNotFound,
			"模型配置不完整，请先在模型配置页补齐接口地址、密钥与模型标识")
	}
	client, err := aiclient.New(options)
	if err != nil {
		return nil, nil, openai.NewAPIError(http.StatusServiceUnavailable,
			openai.ErrorTypeServer, openai.CodeModelNotFound,
			"模型配置不完整，请先在模型配置页补齐接口地址、密钥与模型标识")
	}
	return client, record, nil
}

// resolveModel 按请求的模型标识取配置，匹配不到时退回默认模型
func (s *KefuOpenAIService) resolveModel(modelName string) (*model.KefuModel, error) {
	name := strings.TrimSpace(modelName)
	if name != "" {
		if record, err := s.modelRepository.GetEnabledByName(name); err == nil {
			return record, nil
		} else if !errors.Is(err, repository.ErrModelNotFound) {
			return nil, err
		}
		log.Logger.Debug("kefu openai request model not found, fallback to default",
			zap.String("model", name))
	}

	record, err := s.modelRepository.GetDefault()
	if err != nil {
		if errors.Is(err, repository.ErrModelNotFound) {
			return nil, openai.NewAPIError(http.StatusServiceUnavailable,
				openai.ErrorTypeServer, openai.CodeModelNotFound, "还没有可用的模型配置")
		}
		return nil, err
	}
	return record, nil
}

// modelIdentifier 返回对外暴露的模型标识，模型标识为空时退回模型名称
func modelIdentifier(record *model.KefuModel) string {
	if name := strings.TrimSpace(record.Model); name != "" {
		return name
	}
	return record.Name
}

// convertMessages 把 OpenAI 规范的对话消息转成模型客户端使用的结构，
// 数组形态的 content 只取其中的文本部分
func convertMessages(messages []oai.ChatCompletionMessage) []aiclient.Message {
	list := make([]aiclient.Message, 0, len(messages))
	for _, message := range messages {
		list = append(list, aiclient.Message{
			Role:    message.Role,
			Content: messageText(message),
		})
	}
	return list
}

// messageText 取一条消息的文本内容，兼容 content 为数组的写法
func messageText(message oai.ChatCompletionMessage) string {
	if len(message.MultiContent) == 0 {
		return message.Content
	}
	parts := make([]string, 0, len(message.MultiContent))
	for _, part := range message.MultiContent {
		if part.Type == oai.ChatMessagePartTypeText && part.Text != "" {
			parts = append(parts, part.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// applyRequestOption 用请求里的采样参数覆盖模型配置的默认值，
// 请求未提交的字段保持模型配置里的值
func applyRequestOption(chatReq *oai.ChatCompletionRequest, req openai.ChatCompletionRequest) {
	if req.Temperature != nil {
		chatReq.Temperature = *req.Temperature
	}
	if req.TopP != nil {
		chatReq.TopP = *req.TopP
	}
	if req.MaxTokens > 0 {
		chatReq.MaxTokens = req.MaxTokens
	}
	if req.PresencePenalty != nil {
		chatReq.PresencePenalty = *req.PresencePenalty
	}
	if req.FrequencyPenalty != nil {
		chatReq.FrequencyPenalty = *req.FrequencyPenalty
	}
	if len(req.Stop) > 0 {
		chatReq.Stop = req.Stop
	}
	if req.User != "" {
		chatReq.User = req.User
	}
}

// convertUsage 把上游返回的 token 用量转成响应里的用量，
// 上游没有返回时按字符数估算并标记 estimated
func convertUsage(usage oai.Usage, messages []aiclient.Message, reply string) openai.Usage {
	if usage.TotalTokens == 0 {
		estimated := aiclient.EstimateUsage(messages, reply)
		return openai.Usage{
			PromptTokens:     estimated.PromptTokens,
			CompletionTokens: estimated.CompletionTokens,
			TotalTokens:      estimated.TotalTokens,
			Estimated:        true,
		}
	}
	return openai.Usage{
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
	}
}

// mapModelError 把模型客户端的错误转换成 OpenAI 规范错误体
func mapModelError(err error) error {
	switch {
	case errors.Is(err, aiclient.ErrEmptyMessages):
		return openai.NewAPIError(http.StatusBadRequest, openai.ErrorTypeInvalidRequest,
			openai.CodeInvalidBody, aiclient.ErrEmptyMessages.Error())
	case errors.Is(err, aiclient.ErrInterrupted):
		return openai.NewAPIError(http.StatusRequestTimeout, openai.ErrorTypeServer,
			openai.CodeInternalError, aiclient.ErrInterrupted.Error())
	case errors.Is(err, aiclient.ErrTimeout):
		return openai.NewAPIError(http.StatusGatewayTimeout, openai.ErrorTypeServer,
			openai.CodeInternalError, aiclient.ErrTimeout.Error())
	default:
		return openai.NewAPIError(http.StatusBadGateway, openai.ErrorTypeServer,
			openai.CodeInternalError, err.Error())
	}
}
