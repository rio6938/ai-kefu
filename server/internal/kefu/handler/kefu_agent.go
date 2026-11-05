package handler

import (
	"errors"

	"github.com/gin-gonic/gin"

	"admin/server/internal/common"
	"admin/server/internal/kefu/service"
	"admin/server/model"
)

// kefuAgentService 是客服档案的业务处理
var kefuAgentService = service.NewKefuAgentService()

// handleAgentError 按错误类型输出客服档案相关的响应
func handleAgentError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, service.ErrAgentNotFound):
		common.NotFound(c, err.Error())
	case errors.Is(err, service.ErrAgentUserNotFound):
		common.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrAgentUserRequired),
		errors.Is(err, service.ErrAgentUserExists),
		errors.Is(err, service.ErrAgentStatusInvalid),
		errors.Is(err, service.ErrAgentWorkStatusInvalid),
		errors.Is(err, service.ErrAgentSessionActive),
		errors.Is(err, service.ErrAgentAcceptStatusInvalid):
		common.BadRequest(c, err.Error())
	default:
		common.ServerError(c, fallback)
	}
}

// KefuAgentPage 分页查询客服档案
func KefuAgentPage(c *gin.Context) {
	var param model.KefuAgentParam
	if err := c.ShouldBindQuery(&param); err != nil {
		common.BadRequest(c, "查询参数不正确")
		return
	}

	list, count, err := kefuAgentService.Page(c.Request.Context(), param)
	if err != nil {
		handleAgentError(c, err, "查询客服失败")
		return
	}
	common.Page(c, list, count)
}

// KefuAgentList 查询启用中的客服档案，下拉选择时使用
func KefuAgentList(c *gin.Context) {
	list, err := kefuAgentService.List(c.Request.Context())
	if err != nil {
		handleAgentError(c, err, "查询客服失败")
		return
	}
	common.Success(c, list)
}

// KefuAgentAdd 新增客服档案
func KefuAgentAdd(c *gin.Context) {
	var form model.KefuAgentForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}

	if err := kefuAgentService.Add(form); err != nil {
		handleAgentError(c, err, "添加客服失败")
		return
	}
	common.SuccessWithMessage(c, "添加成功", nil)
}

// KefuAgentUpdate 修改客服档案
func KefuAgentUpdate(c *gin.Context) {
	var form model.KefuAgentForm
	if err := c.ShouldBindJSON(&form); err != nil || form.AgentID <= 0 {
		common.BadRequest(c, "提交数据不正确")
		return
	}

	if err := kefuAgentService.Update(form); err != nil {
		handleAgentError(c, err, "修改客服失败")
		return
	}
	common.SuccessWithMessage(c, "修改成功", nil)
}

// KefuAgentUpdateStatus 修改客服状态或接单开关
func KefuAgentUpdateStatus(c *gin.Context) {
	var form model.KefuAgentStatusForm
	if err := c.ShouldBindJSON(&form); err != nil || form.AgentID <= 0 {
		common.BadRequest(c, "提交数据不正确")
		return
	}

	if err := kefuAgentService.UpdateStatus(c.Request.Context(), form); err != nil {
		handleAgentError(c, err, "修改客服状态失败")
		return
	}
	common.SuccessWithMessage(c, "修改成功", nil)
}

// KefuAgentWorkStatus 坐席切换自己的工作状态（在线 / 小休 / 离线），只改本人
func KefuAgentWorkStatus(c *gin.Context) {
	userID, ok := common.CurrentUserID(c)
	if !ok {
		common.Unauthorized(c, "请先登录")
		return
	}

	var form model.KefuAgentWorkStatusForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}

	if err := kefuAgentService.UpdateWorkStatus(userID, form); err != nil {
		handleAgentError(c, err, "修改工作状态失败")
		return
	}
	common.SuccessWithMessage(c, "修改成功", nil)
}

// KefuAgentRemove 删除客服档案
func KefuAgentRemove(c *gin.Context) {
	agentID, err := parseID(c.Param("id"))
	if err != nil {
		common.BadRequest(c, "客服id不正确")
		return
	}

	if err := kefuAgentService.Remove(c.Request.Context(), agentID); err != nil {
		handleAgentError(c, err, "删除客服失败")
		return
	}
	common.SuccessWithMessage(c, "删除成功", nil)
}
