package handler

import (
	"errors"

	"github.com/gin-gonic/gin"

	"admin/server/internal/common"
	"admin/server/internal/system/service"
	"admin/server/model"
)

// sysMessageService 是我的消息的业务处理
var sysMessageService = service.NewSysMessageService()

// handleMessageError 按错误类型输出消息相关的响应
func handleMessageError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, service.ErrMessageTypeInvalid),
		errors.Is(err, service.ErrMessageIDsRequired),
		errors.Is(err, service.ErrMessageStatusInvalid):
		common.BadRequest(c, err.Error())
	default:
		common.ServerError(c, fallback)
	}
}

// SysMessagePage 分页查询我的消息
func SysMessagePage(c *gin.Context) {
	var param model.MessageParam
	if err := c.ShouldBindQuery(&param); err != nil {
		common.BadRequest(c, "查询参数不正确")
		return
	}

	userID, ok := common.CurrentUserID(c)
	if !ok {
		common.Unauthorized(c, "请先登录")
		return
	}

	list, count, err := sysMessageService.Page(userID, param)
	if err != nil {
		handleMessageError(c, err, "查询消息失败")
		return
	}
	common.Page(c, list, count)
}

// SysMessageUnreadNum 查询我的消息未读数量
func SysMessageUnreadNum(c *gin.Context) {
	userID, ok := common.CurrentUserID(c)
	if !ok {
		common.Unauthorized(c, "请先登录")
		return
	}

	num, err := sysMessageService.UnreadNum(userID)
	if err != nil {
		handleMessageError(c, err, "查询未读数量失败")
		return
	}
	common.Success(c, num)
}

// SysMessageUpdateStatus 修改我的消息状态
func SysMessageUpdateStatus(c *gin.Context) {
	var form model.MessageStatusForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}

	userID, ok := common.CurrentUserID(c)
	if !ok {
		common.Unauthorized(c, "请先登录")
		return
	}

	if err := sysMessageService.UpdateStatus(userID, form); err != nil {
		handleMessageError(c, err, "修改消息状态失败")
		return
	}
	common.SuccessWithMessage(c, "操作成功", nil)
}

// SysMessageRemoveBatch 批量删除我的消息
func SysMessageRemoveBatch(c *gin.Context) {
	ids, ok := parseIDs(c)
	if !ok {
		return
	}

	userID, ok := common.CurrentUserID(c)
	if !ok {
		common.Unauthorized(c, "请先登录")
		return
	}

	if err := sysMessageService.Remove(userID, ids); err != nil {
		handleMessageError(c, err, "删除消息失败")
		return
	}
	common.SuccessWithMessage(c, "删除成功", nil)
}
