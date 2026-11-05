package handler

import (
	"errors"

	"github.com/gin-gonic/gin"

	"admin/server/internal/common"
	"admin/server/internal/kefu/service"
	"admin/server/model"
)

// handleApiKeyError 按错误类型输出 API 密钥相关的响应
func handleApiKeyError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, service.ErrApiKeyNotFound):
		common.NotFound(c, err.Error())
	case errors.Is(err, service.ErrApiKeyNameRequired),
		errors.Is(err, service.ErrApiKeyStatusInvalid),
		errors.Is(err, service.ErrApiKeyExpireInvalid):
		common.BadRequest(c, err.Error())
	default:
		common.ServerError(c, fallback)
	}
}

// KefuApiKeyPage 分页查询对外 API 密钥
func KefuApiKeyPage(c *gin.Context) {
	var param model.KefuApiKeyParam
	if err := c.ShouldBindQuery(&param); err != nil {
		common.BadRequest(c, "查询参数不正确")
		return
	}

	list, count, err := kefuApiKeyService.Page(param)
	if err != nil {
		common.ServerError(c, "查询密钥失败")
		return
	}
	common.Page(c, list, count)
}

// KefuApiKeyIssue 签发对外 API 密钥，明文只返回一次
func KefuApiKeyIssue(c *gin.Context) {
	var form model.KefuApiKeyForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}

	plain, err := kefuApiKeyService.Issue(form)
	if err != nil {
		handleApiKeyError(c, err, "签发密钥失败")
		return
	}
	common.SuccessWithMessage(c, "签发成功，请妥善保存密钥，它只显示这一次", plain)
}

// KefuApiKeyUpdateStatus 修改对外 API 密钥状态
func KefuApiKeyUpdateStatus(c *gin.Context) {
	var form model.KefuApiKeyStatusForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}
	if form.KeyID == 0 {
		common.BadRequest(c, "密钥id不能为空")
		return
	}

	if err := kefuApiKeyService.UpdateStatus(form.KeyID, form.Status); err != nil {
		handleApiKeyError(c, err, "修改密钥状态失败")
		return
	}
	common.SuccessWithMessage(c, "修改成功", nil)
}
