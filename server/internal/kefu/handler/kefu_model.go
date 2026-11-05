package handler

import (
	"errors"

	"github.com/gin-gonic/gin"

	"admin/server/internal/common"
	aiclient "admin/server/internal/kefu/ai/client"
	"admin/server/internal/kefu/service"
	"admin/server/model"
)

// kefuModelService 是模型配置的业务处理
var kefuModelService = service.NewKefuModelService()

// handleModelError 按错误类型输出模型配置相关的响应
func handleModelError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, service.ErrModelNotFound):
		common.NotFound(c, err.Error())
	case errors.Is(err, service.ErrModelNameRequired),
		errors.Is(err, service.ErrModelBaseURLRequired),
		errors.Is(err, service.ErrModelIdentifierRequired),
		errors.Is(err, service.ErrModelApiKeyRequired),
		errors.Is(err, service.ErrModelStatusInvalid),
		errors.Is(err, service.ErrModelDefaultInvalid),
		errors.Is(err, service.ErrModelTranslateInvalid),
		errors.Is(err, service.ErrModelTestFailed),
		errors.Is(err, aiclient.ErrNotConfigured):
		common.BadRequest(c, err.Error())
	default:
		common.ServerError(c, fallback)
	}
}

// KefuModelPage 分页查询模型配置
func KefuModelPage(c *gin.Context) {
	var param model.KefuModelParam
	if err := c.ShouldBindQuery(&param); err != nil {
		common.BadRequest(c, "查询参数不正确")
		return
	}

	list, count, err := kefuModelService.Page(param)
	if err != nil {
		handleModelError(c, err, "查询模型失败")
		return
	}
	common.Page(c, list, count)
}

// KefuModelList 查询启用中的模型配置，下拉选择时使用
func KefuModelList(c *gin.Context) {
	list, err := kefuModelService.List()
	if err != nil {
		handleModelError(c, err, "查询模型失败")
		return
	}
	common.Success(c, list)
}

// KefuModelAdd 新增模型配置
func KefuModelAdd(c *gin.Context) {
	var form model.KefuModelForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}

	if err := kefuModelService.Add(form); err != nil {
		handleModelError(c, err, "添加模型失败")
		return
	}
	common.SuccessWithMessage(c, "添加成功", nil)
}

// KefuModelUpdate 修改模型配置，接口密钥留空表示不修改
func KefuModelUpdate(c *gin.Context) {
	var form model.KefuModelForm
	if err := c.ShouldBindJSON(&form); err != nil || form.ModelID <= 0 {
		common.BadRequest(c, "提交数据不正确")
		return
	}

	if err := kefuModelService.Update(form); err != nil {
		handleModelError(c, err, "修改模型失败")
		return
	}
	common.SuccessWithMessage(c, "修改成功", nil)
}

// KefuModelUpdateStatus 修改模型配置的状态
func KefuModelUpdateStatus(c *gin.Context) {
	var form model.KefuModelStatusForm
	if err := c.ShouldBindJSON(&form); err != nil || form.ModelID <= 0 {
		common.BadRequest(c, "提交数据不正确")
		return
	}

	if err := kefuModelService.UpdateStatus(form); err != nil {
		handleModelError(c, err, "修改模型状态失败")
		return
	}
	common.SuccessWithMessage(c, "修改成功", nil)
}

// KefuModelRemove 删除模型配置
func KefuModelRemove(c *gin.Context) {
	modelID, err := parseID(c.Param("id"))
	if err != nil {
		common.BadRequest(c, "模型id不正确")
		return
	}

	if err := kefuModelService.Remove(modelID); err != nil {
		handleModelError(c, err, "删除模型失败")
		return
	}
	common.SuccessWithMessage(c, "删除成功", nil)
}

// KefuModelTest 用已保存的配置做一次连通性测试
func KefuModelTest(c *gin.Context) {
	modelID, err := parseID(c.Param("id"))
	if err != nil {
		common.BadRequest(c, "模型id不正确")
		return
	}

	if err := kefuModelService.Test(c.Request.Context(), modelID); err != nil {
		handleModelError(c, err, "测试模型连接失败")
		return
	}
	common.SuccessWithMessage(c, "连接成功", nil)
}
