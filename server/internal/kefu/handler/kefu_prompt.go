package handler

import (
	"errors"

	"github.com/gin-gonic/gin"

	"admin/server/internal/common"
	"admin/server/internal/kefu/service"
	"admin/server/model"
)

// kefuPromptService 是提示词模板的业务处理
var kefuPromptService = service.NewKefuPromptService()

// handlePromptError 按错误类型输出提示词相关的响应
func handlePromptError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, service.ErrPromptNotFound):
		common.NotFound(c, err.Error())
	case errors.Is(err, service.ErrPromptNameRequired),
		errors.Is(err, service.ErrPromptStatusInvalid),
		errors.Is(err, service.ErrPromptDefaultInvalid):
		common.BadRequest(c, err.Error())
	default:
		common.ServerError(c, fallback)
	}
}

// KefuPromptPage 分页查询提示词模板
func KefuPromptPage(c *gin.Context) {
	var param model.KefuPromptParam
	if err := c.ShouldBindQuery(&param); err != nil {
		common.BadRequest(c, "查询参数不正确")
		return
	}

	list, count, err := kefuPromptService.Page(param)
	if err != nil {
		common.ServerError(c, "查询提示词失败")
		return
	}
	common.Page(c, list, count)
}

// KefuPromptList 查询启用中的提示词模板
func KefuPromptList(c *gin.Context) {
	list, err := kefuPromptService.List()
	if err != nil {
		common.ServerError(c, "查询提示词失败")
		return
	}
	common.Success(c, list)
}

// KefuPromptAdd 新增提示词模板
func KefuPromptAdd(c *gin.Context) {
	var form model.KefuPromptForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}

	if err := kefuPromptService.Add(form); err != nil {
		handlePromptError(c, err, "添加提示词失败")
		return
	}
	common.SuccessWithMessage(c, "添加成功", nil)
}

// KefuPromptUpdate 修改提示词模板
func KefuPromptUpdate(c *gin.Context) {
	var form model.KefuPromptForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}
	if form.PromptID == 0 {
		common.BadRequest(c, "提示词id不能为空")
		return
	}

	if err := kefuPromptService.Update(form); err != nil {
		handlePromptError(c, err, "修改提示词失败")
		return
	}
	common.SuccessWithMessage(c, "修改成功", nil)
}

// KefuPromptRemove 删除提示词模板
func KefuPromptRemove(c *gin.Context) {
	promptID, err := parseID(c.Param("id"))
	if err != nil {
		common.BadRequest(c, "提示词id不正确")
		return
	}

	if err := kefuPromptService.Remove(promptID); err != nil {
		handlePromptError(c, err, "删除提示词失败")
		return
	}
	common.SuccessWithMessage(c, "删除成功", nil)
}
