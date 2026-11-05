package handler

import (
	"errors"

	"github.com/gin-gonic/gin"

	"admin/server/internal/common"
	"admin/server/internal/kefu/service"
	"admin/server/model"
)

// kefuQuickReplyService 是快捷回复的业务处理
var kefuQuickReplyService = service.NewKefuQuickReplyService()

// handleQuickReplyError 按错误类型输出快捷回复相关的响应
func handleQuickReplyError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, service.ErrQuickReplyNotFound):
		common.NotFound(c, err.Error())
	case errors.Is(err, service.ErrChannelNotFound):
		common.NotFound(c, service.ErrChannelNotFound.Error())
	case errors.Is(err, service.ErrQuickReplyScopeInvalid),
		errors.Is(err, service.ErrQuickReplyTitleRequired),
		errors.Is(err, service.ErrQuickReplyContentRequired),
		errors.Is(err, service.ErrQuickReplyTitleExists),
		errors.Is(err, service.ErrQuickReplyStatusInvalid),
		errors.Is(err, service.ErrChannelDisabled),
		errors.Is(err, service.ErrChannelCodeRequired):
		common.BadRequest(c, err.Error())
	default:
		common.ServerError(c, fallback)
	}
}

// KefuQuickReplyPage 分页查询快捷回复
func KefuQuickReplyPage(c *gin.Context) {
	var param model.KefuQuickReplyParam
	if err := c.ShouldBindQuery(&param); err != nil {
		common.BadRequest(c, "查询参数不正确")
		return
	}

	list, count, err := kefuQuickReplyService.Page(param)
	if err != nil {
		common.ServerError(c, "查询快捷回复失败")
		return
	}
	common.Page(c, list, count)
}

// KefuQuickReplyList 查询启用中的客服快捷话术
func KefuQuickReplyList(c *gin.Context) {
	list, err := kefuQuickReplyService.AgentList()
	if err != nil {
		common.ServerError(c, "查询快捷回复失败")
		return
	}
	common.Success(c, list)
}

// KefuQuickReplyVisitorList 查询访客可用的快捷问题，聊天窗免登录调用
func KefuQuickReplyVisitorList(c *gin.Context) {
	list, err := kefuQuickReplyService.VisitorList(c.Query("channelCode"))
	if err != nil {
		handleQuickReplyError(c, err, "查询快捷问题失败")
		return
	}
	common.Success(c, list)
}

// KefuQuickReplyAdd 新增快捷回复
func KefuQuickReplyAdd(c *gin.Context) {
	var form model.KefuQuickReplyForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}

	if err := kefuQuickReplyService.Add(form); err != nil {
		handleQuickReplyError(c, err, "添加快捷回复失败")
		return
	}
	common.SuccessWithMessage(c, "添加成功", nil)
}

// KefuQuickReplyUpdate 修改快捷回复
func KefuQuickReplyUpdate(c *gin.Context) {
	var form model.KefuQuickReplyForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}
	if form.QuickReplyID == 0 {
		common.BadRequest(c, "快捷回复id不能为空")
		return
	}

	if err := kefuQuickReplyService.Update(form); err != nil {
		handleQuickReplyError(c, err, "修改快捷回复失败")
		return
	}
	common.SuccessWithMessage(c, "修改成功", nil)
}

// KefuQuickReplyUpdateStatus 修改快捷回复状态
func KefuQuickReplyUpdateStatus(c *gin.Context) {
	var form model.KefuQuickReplyStatusForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}
	if form.QuickReplyID == 0 {
		common.BadRequest(c, "快捷回复id不能为空")
		return
	}

	if err := kefuQuickReplyService.UpdateStatus(form.QuickReplyID, form.Status); err != nil {
		handleQuickReplyError(c, err, "修改快捷回复状态失败")
		return
	}
	common.SuccessWithMessage(c, "修改成功", nil)
}

// KefuQuickReplyUse 累加快捷回复的使用次数，客服插入话术后调用
func KefuQuickReplyUse(c *gin.Context) {
	quickReplyID, err := parseID(c.Param("id"))
	if err != nil {
		common.BadRequest(c, "快捷回复id不正确")
		return
	}

	if err := kefuQuickReplyService.MarkUsed(quickReplyID); err != nil {
		handleQuickReplyError(c, err, "累加快捷回复使用次数失败")
		return
	}
	common.SuccessWithMessage(c, "操作成功", nil)
}

// KefuQuickReplyRemove 删除快捷回复
func KefuQuickReplyRemove(c *gin.Context) {
	quickReplyID, err := parseID(c.Param("id"))
	if err != nil {
		common.BadRequest(c, "快捷回复id不正确")
		return
	}

	if err := kefuQuickReplyService.Remove(quickReplyID); err != nil {
		handleQuickReplyError(c, err, "删除快捷回复失败")
		return
	}
	common.SuccessWithMessage(c, "删除成功", nil)
}
