// Package client 是 OpenAI 兼容模型客户端的封装，
// 屏蔽具体服务商的差异，模型参数统一从 kefu_model 记录构造
package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"

	"admin/server/model"
	"admin/server/pkg/encrypt"
)

// 默认请求超时，AI 编排另有自己的超时控制，这里只兜底
const requestTimeout = 5 * time.Minute

// 上游未返回 token 用量时，按字符数估算的除数
const charsPerToken = 1.5

// httpClient 是调用模型接口使用的 HTTP 客户端，只做超时兜底
var httpClient = http.Client{Timeout: requestTimeout}

// 对话角色，与 OpenAI 保持一致
const (
	// RoleSystem 是系统提示词
	RoleSystem = "system"
	// RoleUser 是用户消息
	RoleUser = "user"
	// RoleAssistant 是模型回复
	RoleAssistant = "assistant"
)

// 常见的业务错误
var (
	// ErrNotConfigured 是模型配置不完整
	ErrNotConfigured = errors.New("模型配置不完整，请先在模型配置页补齐接口地址、密钥与模型标识")
	// ErrInterrupted 是生成被中断，映射为 IM 流的 stream.cancel
	ErrInterrupted = errors.New("AI 生成已中断")
	// ErrTimeout 是生成超时，映射为客户侧的兜底话术
	ErrTimeout = errors.New("AI 生成超时")
	// ErrEmptyMessages 是对话消息为空
	ErrEmptyMessages = errors.New("对话消息不能为空")
)

// Options 是一次请求使用的模型参数
type Options struct {
	// 接口地址, 如 https://api.deepseek.com/v1
	BaseURL string
	// 接口密钥明文
	ApiKey string
	// 模型标识
	Model string
	// embedding 接口地址, 为空且未单独配置密钥时沿用 BaseURL
	EmbeddingBaseURL string
	// embedding 接口密钥明文, 为空且未单独配置地址时沿用 ApiKey
	EmbeddingApiKey string
	// embedding 模型标识, 为空时沿用 Model
	EmbeddingModel string
	// 采样温度
	Temperature float64
	// 核采样
	TopP float64
	// 最大回复 token 数
	MaxTokens int
}

// Message 是一条对话消息
type Message struct {
	// 角色, system/user/assistant
	Role string
	// 消息内容
	Content string
}

// Usage 是一次请求的 token 用量，上游未返回时按字符数估算
type Usage struct {
	// 提示词 token 数
	PromptTokens int
	// 回复 token 数
	CompletionTokens int
	// 总 token 数
	TotalTokens int
	// 是否为估算值
	Estimated bool
}

// Client 是一次请求使用的模型客户端，创建后配置只读
type Client struct {
	// 模型参数
	options Options
	// 对话用的 openai 客户端
	client *openai.Client
	// 向量化用的 openai 客户端，embedding 单独配置地址或密钥时不与对话共用
	embedClient *openai.Client
}

// FromModel 按 kefu_model 记录构造模型参数，api_key 在这里解密；
// embedding 的地址与密钥允许独立配置，留空时回退到对话模型的配置
func FromModel(record *model.KefuModel) (Options, error) {
	if record == nil {
		return Options{}, ErrNotConfigured
	}

	baseURL := strings.TrimSpace(record.BaseURL)
	apiKey := strings.TrimSpace(record.ApiKey)
	modelName := strings.TrimSpace(record.Model)
	if baseURL == "" || apiKey == "" || modelName == "" {
		return Options{}, ErrNotConfigured
	}

	plain, err := encrypt.Decrypt(apiKey)
	if err != nil {
		return Options{}, fmt.Errorf("模型密钥解密失败: %w", err)
	}

	embeddingApiKey := ""
	if strings.TrimSpace(record.EmbeddingApiKey) != "" {
		decrypted, err := encrypt.Decrypt(record.EmbeddingApiKey)
		if err != nil {
			return Options{}, fmt.Errorf("embedding 模型密钥解密失败: %w", err)
		}
		embeddingApiKey = strings.TrimSpace(decrypted)
	}

	return Options{
		BaseURL:          baseURL,
		ApiKey:           strings.TrimSpace(plain),
		Model:            modelName,
		EmbeddingBaseURL: strings.TrimSpace(record.EmbeddingBaseURL),
		EmbeddingApiKey:  embeddingApiKey,
		EmbeddingModel:   strings.TrimSpace(record.EmbeddingModel),
		Temperature:      record.Temperature,
		TopP:             record.TopP,
		MaxTokens:        record.MaxTokens,
	}, nil
}

// New 创建模型客户端，对话参数不完整时返回 ErrNotConfigured；
// embedding 的地址与密钥都没单独配置时沿用对话模型的配置
func New(options Options) (*Client, error) {
	if strings.TrimSpace(options.BaseURL) == "" ||
		strings.TrimSpace(options.ApiKey) == "" ||
		strings.TrimSpace(options.Model) == "" {
		return nil, ErrNotConfigured
	}

	baseURL := strings.TrimRight(strings.TrimSpace(options.BaseURL), "/")
	apiKey := strings.TrimSpace(options.ApiKey)
	chatClient := newOpenAIClient(baseURL, apiKey)

	// embedding 地址与密钥都没单独配置时完全沿用对话配置；
	// 单独配了地址说明是另一个服务(如本地部署的向量模型), 此时密钥不跟随对话配置，
	// 避免把云服务的密钥发给本地服务
	embedBaseURL := strings.TrimRight(strings.TrimSpace(options.EmbeddingBaseURL), "/")
	embedApiKey := strings.TrimSpace(options.EmbeddingApiKey)
	embedClient := chatClient
	if embedBaseURL != "" || embedApiKey != "" {
		if embedBaseURL == "" {
			embedBaseURL = baseURL
		}
		embedClient = newOpenAIClient(embedBaseURL, embedApiKey)
	}

	return &Client{options: options, client: chatClient, embedClient: embedClient}, nil
}

// newOpenAIClient 按地址与密钥创建一个 openai 客户端
func newOpenAIClient(baseURL, apiKey string) *openai.Client {
	cfg := openai.DefaultConfig(apiKey)
	cfg.BaseURL = baseURL
	cfg.HTTPClient = &httpClient
	return openai.NewClientWithConfig(cfg)
}

// Request 按模型参数组装一次对话请求，调用方可以继续改其它字段（如 stop）
func (c *Client) Request(messages []Message) openai.ChatCompletionRequest {
	list := make([]openai.ChatCompletionMessage, 0, len(messages))
	for _, message := range messages {
		list = append(list, openai.ChatCompletionMessage{
			Role:    message.Role,
			Content: message.Content,
		})
	}

	return openai.ChatCompletionRequest{
		Model:       c.options.Model,
		Messages:    list,
		Temperature: float32(c.options.Temperature),
		TopP:        float32(c.options.TopP),
		MaxTokens:   c.options.MaxTokens,
	}
}

// ChatStream 流式对话，每收到一段增量就回调 onDelta；返回本次 token 用量。
// 上下文取消映射为 ErrInterrupted, 超时映射为 ErrTimeout
func (c *Client) ChatStream(
	ctx context.Context,
	req openai.ChatCompletionRequest,
	onDelta func(delta string) error,
) (*Usage, error) {
	if len(req.Messages) == 0 {
		return nil, ErrEmptyMessages
	}

	// 让上游在最后一个 chunk 里带上用量，不支持的模型会忽略该参数
	req.Stream = true
	req.StreamOptions = &openai.StreamOptions{IncludeUsage: true}

	stream, err := c.client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return nil, wrapError(ctx, err)
	}
	defer func() {
		_ = stream.Close()
	}()

	var builder strings.Builder
	var usage *Usage
	for {
		resp, recvErr := stream.Recv()
		if errors.Is(recvErr, io.EOF) {
			break
		}
		if recvErr != nil {
			return nil, wrapError(ctx, recvErr)
		}

		if resp.Usage != nil && resp.Usage.TotalTokens > 0 {
			usage = &Usage{
				PromptTokens:     resp.Usage.PromptTokens,
				CompletionTokens: resp.Usage.CompletionTokens,
				TotalTokens:      resp.Usage.TotalTokens,
			}
		}
		if len(resp.Choices) == 0 {
			continue
		}

		delta := resp.Choices[0].Delta.Content
		if delta == "" {
			continue
		}
		builder.WriteString(delta)
		if onDelta != nil {
			if err := onDelta(delta); err != nil {
				return usageOrEstimated(usage, req, builder.String()), err
			}
		}
	}

	return usageOrEstimated(usage, req, builder.String()), nil
}

// ChatOnce 非流式对话，返回完整响应
func (c *Client) ChatOnce(ctx context.Context, req openai.ChatCompletionRequest) (*openai.ChatCompletionResponse, error) {
	if len(req.Messages) == 0 {
		return nil, ErrEmptyMessages
	}

	req.Stream = false
	resp, err := c.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, wrapError(ctx, err)
	}
	return &resp, nil
}

// Embeddings 批量向量化，返回与入参一一对应的向量；
// 配置了 embedding 模型时用它，否则沿用对话模型；
// embedding 单独配置了地址或密钥时用自己的客户端，与对话模型互不影响
func (c *Client) Embeddings(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	embeddingModel := strings.TrimSpace(c.options.EmbeddingModel)
	if embeddingModel == "" {
		embeddingModel = strings.TrimSpace(c.options.Model)
	}
	if embeddingModel == "" {
		return nil, ErrNotConfigured
	}

	resp, err := c.embedClient.CreateEmbeddings(ctx, openai.EmbeddingRequestStrings{
		Input: texts,
		Model: openai.EmbeddingModel(embeddingModel),
	})
	if err != nil {
		return nil, wrapError(ctx, err)
	}
	if len(resp.Data) != len(texts) {
		return nil, fmt.Errorf("向量化结果数量不正确: 期望 %d 条, 实际 %d 条", len(texts), len(resp.Data))
	}

	list := make([][]float32, 0, len(resp.Data))
	for _, item := range resp.Data {
		list = append(list, item.Embedding)
	}
	return list, nil
}

// wrapError 把上下文取消与超时映射成本包的业务错误
func wrapError(ctx context.Context, err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
		return ErrInterrupted
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return ErrTimeout
	}
	return fmt.Errorf("调用模型接口失败: %w", err)
}

// usageOrEstimated 优先用上游返回的用量，没有时按字符数估算
func usageOrEstimated(usage *Usage, req openai.ChatCompletionRequest, reply string) *Usage {
	if usage != nil {
		return usage
	}

	messages := make([]Message, 0, len(req.Messages))
	for _, message := range req.Messages {
		messages = append(messages, Message{Role: message.Role, Content: message.Content})
	}
	return EstimateUsage(messages, reply)
}

// EstimateUsage 按字符数估算 token 用量，上游没有返回用量时使用
func EstimateUsage(messages []Message, reply string) *Usage {
	promptChars := 0
	for _, message := range messages {
		promptChars += len([]rune(message.Content))
	}
	promptTokens := estimateTokens(promptChars)
	completionTokens := estimateTokens(len([]rune(reply)))
	return &Usage{
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      promptTokens + completionTokens,
		Estimated:        true,
	}
}

// estimateTokens 按字符数粗略估算 token 数
func estimateTokens(chars int) int {
	if chars <= 0 {
		return 0
	}
	tokens := int(float64(chars) / charsPerToken)
	if tokens <= 0 {
		return 1
	}
	return tokens
}
