package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"admin/server/internal/common"
	aiclient "admin/server/internal/kefu/ai/client"
	"admin/server/internal/kefu/openai"
	"admin/server/internal/kefu/repository"
	"admin/server/internal/kefu/service"
	"admin/server/model"
)

// kefuAiDraftService 是工作台 AI 草稿的业务处理
var kefuAiDraftService = service.NewKefuAiDraftService()

// KefuSessionAiDraft 为工作台的「右键 AI 生成」输出草稿；
// 路由故意与 useAiSdk 的 /chat/completions 对齐，所以请求体忽略 model 与 messages，
// 响应按 OpenAI SSE 输出，错误体也按 OpenAI 规范，便于前端零改动复用 useAiSdk
func KefuSessionAiDraft(c *gin.Context) {
	userID, ok := common.CurrentUserID(c)
	if !ok {
		openai.WriteError(c, openai.NewAPIError(http.StatusUnauthorized,
			openai.ErrorTypeAuthentication, openai.CodeUnauthorized, "请先登录"))
		return
	}
	sessionID, err := parseID(c.Param("id"))
	if err != nil {
		openai.WriteError(c, openai.NewAPIError(http.StatusBadRequest,
			openai.ErrorTypeInvalidRequest, openai.CodeInvalidBody, "会话id不正确"))
		return
	}

	var form model.KefuAiDraftForm
	if err := c.ShouldBindJSON(&form); err != nil {
		openai.WriteError(c, openai.NewAPIError(http.StatusBadRequest,
			openai.ErrorTypeInvalidRequest, openai.CodeInvalidBody, "提交数据不正确"))
		return
	}

	id := openai.NewCompletionID()
	created := time.Now().Unix()
	modelName := ""
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
	writeChunk := func(delta openai.Delta, finishReason *string) error {
		return writeData(openai.ChatCompletionChunk{
			ID:      id,
			Object:  openai.ObjectChatCompletionChunk,
			Created: created,
			Model:   modelName,
			Choices: []openai.ChunkChoice{{
				Index:        0,
				Delta:        delta,
				FinishReason: finishReason,
			}},
		})
	}

	err = kefuAiDraftService.Draft(c.Request.Context(), userID, sessionID, form,
		func(model string) error {
			modelName = model
			// 首个分片只带角色，与 OpenAI 的流式响应保持一致
			return writeChunk(openai.Delta{Role: aiclient.RoleAssistant, Content: ""}, nil)
		},
		func(delta string) error {
			return writeChunk(openai.Delta{Content: delta}, nil)
		})
	if err != nil {
		// 客服点了停止或页面关闭时请求上下文已取消，不再回写
		if c.Request.Context().Err() != nil {
			return
		}
		if !started {
			openai.WriteError(c, draftAPIError(err))
			return
		}
		// 流已经开始，错误只能通过 SSE 帧下发
		apiErr := draftAPIError(err)
		_ = writeData(openai.ErrorResponse{Error: openai.ErrorBody{
			Message: apiErr.Message,
			Type:    apiErr.Type,
			Code:    apiErr.Code,
		}})
		return
	}

	// 正常结束补发 finish_reason=stop 的分片与 data: [DONE]
	finishReason := "stop"
	_ = writeChunk(openai.Delta{}, &finishReason)
	_, _ = fmt.Fprint(c.Writer, "data: [DONE]\n\n")
	c.Writer.Flush()
}

// draftAPIError 把草稿接口的错误转换成 OpenAI 错误体
func draftAPIError(err error) *openai.APIError {
	switch {
	case errors.Is(err, service.ErrSessionNotFound),
		errors.Is(err, service.ErrAiDraftMessageNotFound),
		errors.Is(err, service.ErrVisitorNotFound):
		return openai.NewAPIError(http.StatusNotFound,
			openai.ErrorTypeInvalidRequest, openai.CodeInvalidBody, err.Error())
	case errors.Is(err, service.ErrSessionNoPermission),
		errors.Is(err, service.ErrNotAgent):
		return openai.NewAPIError(http.StatusForbidden,
			openai.ErrorTypeInvalidRequest, openai.CodeForbidden, err.Error())
	case errors.Is(err, service.ErrAiDraftActionInvalid),
		errors.Is(err, service.ErrAiDraftPromptRequired),
		errors.Is(err, aiclient.ErrNotConfigured):
		return openai.NewAPIError(http.StatusBadRequest,
			openai.ErrorTypeInvalidRequest, openai.CodeInvalidBody, err.Error())
	case errors.Is(err, repository.ErrModelNotFound):
		return openai.NewAPIError(http.StatusBadRequest,
			openai.ErrorTypeInvalidRequest, openai.CodeModelNotFound,
			"没有可用的模型配置，请先在模型配置页启用一个模型")
	case errors.Is(err, aiclient.ErrTimeout):
		return openai.NewAPIError(http.StatusGatewayTimeout,
			openai.ErrorTypeServer, openai.CodeInternalError, aiclient.ErrTimeout.Error())
	default:
		return openai.NewAPIError(http.StatusInternalServerError,
			openai.ErrorTypeServer, openai.CodeInternalError, "AI 生成失败")
	}
}
