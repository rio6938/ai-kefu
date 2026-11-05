package handler

import (
	"errors"

	"github.com/gin-gonic/gin"

	"admin/server/internal/common"
	"admin/server/internal/kefu/service"
	"admin/server/model"
)

// kefuDialogueService 是对话库的业务处理
var kefuDialogueService = service.NewKefuDialogueService()

// handleDialogueError 按错误类型输出对话库相关的响应
func handleDialogueError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, service.ErrDialogueNotFound):
		common.NotFound(c, err.Error())
	case errors.Is(err, service.ErrSessionNotFound):
		common.NotFound(c, err.Error())
	case errors.Is(err, service.ErrSessionNoPermission):
		common.Forbidden(c, err.Error())
	case errors.Is(err, service.ErrDialogueTitleRequired),
		errors.Is(err, service.ErrDialogueTitleExists),
		errors.Is(err, service.ErrDialogueMessagesRequired),
		errors.Is(err, service.ErrDialogueMessageRoleInvalid),
		errors.Is(err, service.ErrDialogueMessageContentRequired),
		errors.Is(err, service.ErrDialogueRouteNotFound),
		errors.Is(err, service.ErrDialogueStatusInvalid):
		common.BadRequest(c, err.Error())
	default:
		common.ServerError(c, fallback)
	}
}

// KefuDialoguePage 分页查询对话库
func KefuDialoguePage(c *gin.Context) {
	var param model.KefuDialogueParam
	if err := c.ShouldBindQuery(&param); err != nil {
		common.BadRequest(c, "查询参数不正确")
		return
	}

	list, count, err := kefuDialogueService.Page(param)
	if err != nil {
		common.ServerError(c, "查询对话库失败")
		return
	}
	common.Page(c, list, count)
}

// KefuDialogueDetail 查询对话库条目详情
func KefuDialogueDetail(c *gin.Context) {
	dialogueID, err := parseID(c.Param("id"))
	if err != nil {
		common.BadRequest(c, "对话id不正确")
		return
	}

	record, err := kefuDialogueService.Detail(dialogueID)
	if err != nil {
		handleDialogueError(c, err, "查询对话失败")
		return
	}
	common.Success(c, record)
}

// KefuDialogueSessionDraft 把会话记录整理成对话库草稿，供运营修改后保存
func KefuDialogueSessionDraft(c *gin.Context) {
	userID, ok := common.CurrentUserID(c)
	if !ok {
		common.Unauthorized(c, "请先登录")
		return
	}
	sessionID, err := parseID(c.Query("sessionId"))
	if err != nil {
		common.BadRequest(c, "会话id不正确")
		return
	}

	draft, err := kefuDialogueService.SessionDraft(userID, sessionID)
	if err != nil {
		handleDialogueError(c, err, "导入会话失败")
		return
	}
	common.Success(c, draft)
}

// KefuDialogueAdd 新增对话库条目
func KefuDialogueAdd(c *gin.Context) {
	var form model.KefuDialogueForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}

	if err := kefuDialogueService.Add(form); err != nil {
		handleDialogueError(c, err, "添加对话失败")
		return
	}
	common.SuccessWithMessage(c, "添加成功", nil)
}

// KefuDialogueUpdate 修改对话库条目
func KefuDialogueUpdate(c *gin.Context) {
	var form model.KefuDialogueForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}
	if form.DialogueID == 0 {
		common.BadRequest(c, "对话id不能为空")
		return
	}

	if err := kefuDialogueService.Update(form); err != nil {
		handleDialogueError(c, err, "修改对话失败")
		return
	}
	common.SuccessWithMessage(c, "修改成功", nil)
}

// KefuDialogueUpdateStatus 修改对话库条目的状态
func KefuDialogueUpdateStatus(c *gin.Context) {
	var form model.KefuDialogueStatusForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}
	if form.DialogueID == 0 {
		common.BadRequest(c, "对话id不能为空")
		return
	}

	if err := kefuDialogueService.UpdateStatus(form.DialogueID, form.Status); err != nil {
		handleDialogueError(c, err, "修改对话状态失败")
		return
	}
	common.SuccessWithMessage(c, "修改成功", nil)
}

// KefuDialogueRemove 删除对话库条目
func KefuDialogueRemove(c *gin.Context) {
	dialogueID, err := parseID(c.Param("id"))
	if err != nil {
		common.BadRequest(c, "对话id不正确")
		return
	}

	if err := kefuDialogueService.Remove(dialogueID); err != nil {
		handleDialogueError(c, err, "删除对话失败")
		return
	}
	common.SuccessWithMessage(c, "删除成功", nil)
}
