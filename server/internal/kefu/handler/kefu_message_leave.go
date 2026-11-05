package handler

import (
	"errors"

	"github.com/gin-gonic/gin"

	"admin/server/internal/common"
	"admin/server/internal/kefu/service"
	"admin/server/model"
)

// kefuMessageLeaveService 是离线留言的业务处理
var kefuMessageLeaveService = service.NewKefuMessageLeaveService()

// handleMessageLeaveError 按错误类型输出留言相关的响应
func handleMessageLeaveError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, service.ErrMessageLeaveNotFound):
		common.NotFound(c, err.Error())
	case errors.Is(err, service.ErrSessionNotFound):
		common.NotFound(c, err.Error())
	case errors.Is(err, service.ErrVisitorNotFound):
		common.NotFound(c, service.ErrVisitorNotFound.Error())
	case errors.Is(err, service.ErrSessionNoPermission),
		errors.Is(err, service.ErrNotAgent):
		common.Forbidden(c, err.Error())
	case errors.Is(err, service.ErrMessageLeaveContactTypeInvalid),
		errors.Is(err, service.ErrMessageLeaveContactInvalid),
		errors.Is(err, service.ErrMessageLeaveContentRequired),
		errors.Is(err, service.ErrMessageLeaveHandled):
		common.BadRequest(c, err.Error())
	default:
		common.ServerError(c, fallback)
	}
}

// KefuClientSessionLeaveMessage 访客提交留言，聊天窗无人在线或会话转留言时调用
func KefuClientSessionLeaveMessage(c *gin.Context) {
	sessionID, err := parseID(c.Param("id"))
	if err != nil {
		common.BadRequest(c, "会话id不正确")
		return
	}

	var form model.KefuMessageLeaveForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}

	leave, err := kefuMessageLeaveService.Submit(
		c.Request.Context(), sessionID, c.Query("visitorKey"), form)
	if err != nil {
		handleMessageLeaveError(c, err, "提交留言失败")
		return
	}
	common.SuccessWithMessage(c, "已收到您的留言", leave)
}

// KefuMessageLeavePage 分页查询留言
func KefuMessageLeavePage(c *gin.Context) {
	var param model.KefuMessageLeaveParam
	if err := c.ShouldBindQuery(&param); err != nil {
		common.BadRequest(c, "查询参数不正确")
		return
	}

	list, count, err := kefuMessageLeaveService.Page(param)
	if err != nil {
		handleMessageLeaveError(c, err, "查询留言失败")
		return
	}
	common.Page(c, list, count)
}

// KefuMessageLeaveHandle 坐席处理留言
func KefuMessageLeaveHandle(c *gin.Context) {
	userID, ok := common.CurrentUserID(c)
	if !ok {
		common.Unauthorized(c, "请先登录")
		return
	}

	var form model.KefuMessageLeaveHandleForm
	if err := c.ShouldBindJSON(&form); err != nil || form.LeaveID <= 0 {
		common.BadRequest(c, "提交数据不正确")
		return
	}

	if err := kefuMessageLeaveService.Handle(c.Request.Context(), userID, form); err != nil {
		handleMessageLeaveError(c, err, "处理留言失败")
		return
	}
	common.SuccessWithMessage(c, "处理成功", nil)
}
