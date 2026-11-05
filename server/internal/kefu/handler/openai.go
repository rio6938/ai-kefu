package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"admin/server/internal/kefu/openai"
	"admin/server/internal/kefu/service"
)

// kefuOpenAIService 是 OpenAI 兼容接口的业务处理
var kefuOpenAIService = service.NewKefuOpenAIService()

// kefuApiKeyService 是对外 API 密钥的业务处理
var kefuApiKeyService = service.NewKefuApiKeyService()

// KefuOpenAIAuth 校验 /v1 路由的 Authorization: Bearer <kefu_api_key>，
// 校验失败按 OpenAI 规范返回 401
func KefuOpenAIAuth(c *gin.Context) {
	token := openAIBearerToken(c.GetHeader("Authorization"))
	if token == "" {
		openai.WriteError(c, openai.NewAPIError(http.StatusUnauthorized,
			openai.ErrorTypeAuthentication, openai.CodeInvalidAPIKey, "缺少 API 密钥"))
		return
	}
	if err := kefuApiKeyService.Verify(token); err != nil {
		openai.WriteError(c, openai.NewAPIError(http.StatusUnauthorized,
			openai.ErrorTypeAuthentication, openai.CodeInvalidAPIKey, err.Error()))
		return
	}
	c.Next()
}

// KefuOpenAIChatCompletions 兼容 OpenAI 的对话接口，stream 为 true 时走 SSE
func KefuOpenAIChatCompletions(c *gin.Context) {
	var req openai.ChatCompletionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		openai.WriteError(c, openai.NewAPIError(http.StatusBadRequest,
			openai.ErrorTypeInvalidRequest, openai.CodeInvalidBody, "提交数据不正确"))
		return
	}
	if len(req.Messages) == 0 {
		openai.WriteError(c, openai.NewAPIError(http.StatusBadRequest,
			openai.ErrorTypeInvalidRequest, openai.CodeInvalidBody, "messages 不能为空"))
		return
	}
	// 第一版不支持工具调用，明确提示而不是静默忽略
	if len(req.Tools) > 0 || len(req.Functions) > 0 ||
		len(req.FunctionCall) > 0 || len(req.ToolChoice) > 0 {
		openai.WriteError(c, openai.NewAPIError(http.StatusBadRequest,
			openai.ErrorTypeInvalidRequest, openai.CodeInvalidBody, "暂不支持 tools"))
		return
	}

	if req.Stream {
		streamChatCompletions(c, req)
		return
	}

	resp, err := kefuOpenAIService.ChatCompletion(c.Request.Context(), req)
	if err != nil {
		openai.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// streamChatCompletions 按 OpenAI 的 SSE 格式逐块输出，
// 正常结束补发 finish_reason=stop 的分片与 data: [DONE]
func streamChatCompletions(c *gin.Context, req openai.ChatCompletionRequest) {
	started := false
	// 第一个分片产出时才写 SSE 响应头，前置错误还能返回正常的 HTTP 状态码
	start := func() {
		if started {
			return
		}
		started = true
		c.Header("Content-Type", "text/event-stream; charset=utf-8")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no")
		c.Status(http.StatusOK)
		c.Writer.Flush()
	}

	writeData := func(data any) error {
		start()
		raw, err := json.Marshal(data)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(c.Writer, "data: %s\n\n", raw); err != nil {
			return err
		}
		c.Writer.Flush()
		return nil
	}

	// 客户端断开时请求上下文会取消，上游模型请求随之中止
	err := kefuOpenAIService.ChatCompletionStream(c.Request.Context(), req,
		func(chunk openai.ChatCompletionChunk) error {
			return writeData(chunk)
		})
	if err != nil {
		if !started {
			openai.WriteError(c, err)
			return
		}
		// 流已经开始，错误只能通过 SSE 帧下发
		apiErr := openai.AsAPIError(err)
		_ = writeData(openai.ErrorResponse{Error: openai.ErrorBody{
			Message: apiErr.Message,
			Type:    apiErr.Type,
			Code:    apiErr.Code,
		}})
		return
	}
	start()
	_, _ = fmt.Fprint(c.Writer, "data: [DONE]\n\n")
	c.Writer.Flush()
}

// KefuOpenAIModels 返回启用中的模型列表
func KefuOpenAIModels(c *gin.Context) {
	list, err := kefuOpenAIService.Models()
	if err != nil {
		openai.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, openai.ModelList{Object: openai.ObjectList, Data: list})
}

// openAIBearerToken 从 Authorization 请求头里取出密钥
func openAIBearerToken(header string) string {
	header = strings.TrimSpace(header)
	if len(header) > 7 && strings.EqualFold(header[:7], "Bearer ") {
		return strings.TrimSpace(header[7:])
	}
	return header
}
