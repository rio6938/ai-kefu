package handler

import (
	"errors"

	"github.com/gin-gonic/gin"

	"admin/server/internal/common"
	aiclient "admin/server/internal/kefu/ai/client"
	"admin/server/internal/kefu/service"
	"admin/server/model"
)

// kefuTranslateService 是翻译的业务处理
var kefuTranslateService = service.NewKefuTranslateService()

// handleTranslateError 按错误类型输出翻译相关的响应
func handleTranslateError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, service.ErrTranslateModelMissing):
		common.BadRequest(c, "未配置可用的模型，请先在模型配置中启用")
	case errors.Is(err, service.ErrTranslateDisabled),
		errors.Is(err, service.ErrTranslateTextRequired),
		errors.Is(err, service.ErrTranslateTextTooLong),
		errors.Is(err, service.ErrTranslateFailed),
		errors.Is(err, aiclient.ErrNotConfigured):
		common.BadRequest(c, err.Error())
	case errors.Is(err, aiclient.ErrTimeout),
		errors.Is(err, aiclient.ErrInterrupted):
		common.BadRequest(c, "翻译超时，请稍后重试")
	default:
		common.ServerError(c, fallback)
	}
}

// KefuTranslate 把文本翻译成目标语言，客户消息自动翻译与输入框翻译共用
func KefuTranslate(c *gin.Context) {
	var form model.KefuTranslateForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}

	result, err := kefuTranslateService.Translate(c.Request.Context(), form)
	if err != nil {
		handleTranslateError(c, err, "翻译失败")
		return
	}
	common.Success(c, result)
}
