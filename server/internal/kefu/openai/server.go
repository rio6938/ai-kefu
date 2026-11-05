// Package openai 是 OpenAI 兼容接口的协议定义与响应输出；
// 这一组路由不使用统一的 common.Response，直接按 OpenAI 规范返回数据与错误，
// 以便第三方的 OpenAI SDK 可以直接指向本服务
package openai

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	oai "github.com/sashabaranov/go-openai"
)

// 响应的对象类型，与 OpenAI 规范保持一致
const (
	// ObjectChatCompletion 是非流式对话的响应对象
	ObjectChatCompletion = "chat.completion"
	// ObjectChatCompletionChunk 是流式对话的分片对象
	ObjectChatCompletionChunk = "chat.completion.chunk"
	// ObjectList 是列表类响应的对象
	ObjectList = "list"
	// ObjectModel 是模型对象
	ObjectModel = "model"
)

// 错误类型，与 OpenAI 规范保持一致
const (
	// ErrorTypeInvalidRequest 是请求参数错误
	ErrorTypeInvalidRequest = "invalid_request_error"
	// ErrorTypeAuthentication 是密钥错误
	ErrorTypeAuthentication = "authentication_error"
	// ErrorTypeServer 是服务端错误
	ErrorTypeServer = "server_error"
)

// 常用错误码
const (
	// CodeInvalidAPIKey 是密钥不正确或已失效
	CodeInvalidAPIKey = "invalid_api_key"
	// CodeModelNotFound 是没有可用的模型配置
	CodeModelNotFound = "model_not_found"
	// CodeInvalidBody 是请求体不正确
	CodeInvalidBody = "invalid_body"
	// CodeUnauthorized 是未登录或登录已过期
	CodeUnauthorized = "unauthorized"
	// CodeForbidden 是登录用户没有权限
	CodeForbidden = "forbidden"
	// CodeInternalError 是服务端内部错误
	CodeInternalError = "internal_error"
)

// APIError 是 /v1 接口的业务错误，携带 HTTP 状态码与 OpenAI 错误字段
type APIError struct {
	// HTTP 状态码
	Status int
	// 错误类型
	Type string
	// 错误码
	Code string
	// 中文错误提示
	Message string
}

// Error 返回中文错误提示
func (e *APIError) Error() string {
	return e.Message
}

// NewAPIError 创建 /v1 接口的业务错误
func NewAPIError(status int, errType, code, message string) *APIError {
	return &APIError{Status: status, Type: errType, Code: code, Message: message}
}

// AsAPIError 把任意错误转换成 OpenAI 错误体，未知错误按服务端错误处理
func AsAPIError(err error) *APIError {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr
	}
	message := "服务内部错误"
	if err != nil {
		message = err.Error()
	}
	return NewAPIError(http.StatusInternalServerError, ErrorTypeServer, CodeInternalError, message)
}

// WriteError 按 OpenAI 规范输出错误响应，HTTP 状态码同样使用对应语义
func WriteError(c *gin.Context, err error) {
	apiErr := AsAPIError(err)
	c.AbortWithStatusJSON(apiErr.Status, ErrorResponse{
		Error: ErrorBody{
			Message: apiErr.Message,
			Type:    apiErr.Type,
			Code:    apiErr.Code,
		},
	})
}

// ErrorResponse 是错误响应体
type ErrorResponse struct {
	// 错误详情
	Error ErrorBody `json:"error"`
}

// ErrorBody 是错误响应的详细字段
type ErrorBody struct {
	// 中文错误提示
	Message string `json:"message"`
	// 错误类型
	Type string `json:"type"`
	// 错误码
	Code string `json:"code"`
}

// Message 是对话消息
type Message struct {
	// 角色, system/user/assistant
	Role string `json:"role"`
	// 消息内容
	Content string `json:"content"`
}

// ChatCompletionRequest 是对话请求，只声明 OpenAI 规范里第一版支持的字段；
// messages 直接复用 go-openai 的类型，content 支持字符串与数组两种写法
// （引用类型原名会与本包冲突，这里单独声明 Message 用于响应）
type ChatCompletionRequest struct {
	// 模型标识, 为空时用默认模型
	Model string `json:"model"`
	// 对话消息
	Messages []oai.ChatCompletionMessage `json:"messages"`
	// 是否流式返回
	Stream bool `json:"stream"`
	// 采样温度, 未提交时用模型配置的值
	Temperature *float32 `json:"temperature"`
	// 核采样, 未提交时用模型配置的值
	TopP *float32 `json:"top_p"`
	// 最大回复 token 数, 未提交时用模型配置的值
	MaxTokens int `json:"max_tokens"`
	// 停止词
	Stop []string `json:"stop"`
	// 存在惩罚
	PresencePenalty *float32 `json:"presence_penalty"`
	// 频率惩罚
	FrequencyPenalty *float32 `json:"frequency_penalty"`
	// 调用方标识
	User string `json:"user"`
	// 工具调用, 第一版不支持
	Tools []json.RawMessage `json:"tools"`
	// 函数调用, 第一版不支持
	Functions []json.RawMessage `json:"functions"`
	// 指定函数, 第一版不支持
	FunctionCall json.RawMessage `json:"function_call"`
	// 工具选择, 第一版不支持
	ToolChoice json.RawMessage `json:"tool_choice"`
}

// Usage 是 token 用量，上游未返回时按字符数估算并标记 estimated
type Usage struct {
	// 提示词 token 数
	PromptTokens int `json:"prompt_tokens"`
	// 回复 token 数
	CompletionTokens int `json:"completion_tokens"`
	// 总 token 数
	TotalTokens int `json:"total_tokens"`
	// 是否为估算值
	Estimated bool `json:"estimated,omitempty"`
}

// ChatCompletionResponse 是非流式对话的响应，字段与 OpenAI 规范一致
type ChatCompletionResponse struct {
	// 响应id
	ID string `json:"id"`
	// 对象类型
	Object string `json:"object"`
	// 创建时间, 秒
	Created int64 `json:"created"`
	// 模型标识
	Model string `json:"model"`
	// 回复候选
	Choices []Choice `json:"choices"`
	// token 用量
	Usage Usage `json:"usage"`
}

// Choice 是回复候选
type Choice struct {
	// 候选下标
	Index int `json:"index"`
	// 回复消息
	Message Message `json:"message"`
	// 结束原因
	FinishReason string `json:"finish_reason"`
}

// ChatCompletionChunk 是流式对话的一个分片
type ChatCompletionChunk struct {
	// 响应id
	ID string `json:"id"`
	// 对象类型
	Object string `json:"object"`
	// 创建时间, 秒
	Created int64 `json:"created"`
	// 模型标识
	Model string `json:"model"`
	// 分片候选
	Choices []ChunkChoice `json:"choices"`
	// token 用量, 只有结束分片才带
	Usage *Usage `json:"usage,omitempty"`
}

// ChunkChoice 是分片候选
type ChunkChoice struct {
	// 候选下标
	Index int `json:"index"`
	// 本次增量
	Delta Delta `json:"delta"`
	// 结束原因, 未结束时为 null
	FinishReason *string `json:"finish_reason"`
}

// Delta 是流式增量
type Delta struct {
	// 角色, 只有首个分片带
	Role string `json:"role,omitempty"`
	// 本次增量文本
	Content string `json:"content,omitempty"`
}

// ModelList 是模型列表的响应
type ModelList struct {
	// 对象类型
	Object string `json:"object"`
	// 模型列表
	Data []Model `json:"data"`
}

// Model 是模型对象
type Model struct {
	// 模型标识
	ID string `json:"id"`
	// 对象类型
	Object string `json:"object"`
	// 创建时间, 秒
	Created int64 `json:"created"`
	// 归属方
	OwnedBy string `json:"owned_by"`
}

// NewCompletionID 生成对话响应的id
func NewCompletionID() string {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return "chatcmpl-unknown"
	}
	return fmt.Sprintf("chatcmpl-%s", hex.EncodeToString(buf))
}
