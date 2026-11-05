package handler

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"

	"admin/server/internal/common"
	"admin/server/internal/kefu/service"
	"admin/server/model"
)

// kefuSessionService 是会话的业务处理
var kefuSessionService = service.NewKefuSessionService()

// kefuAiService 是 AI 自动回复的业务处理
var kefuAiService = service.NewKefuAiService()

// kefuTransferService 是会话转接的业务处理
var kefuTransferService = service.NewKefuTransferService()

// kefuHandoffService 是访客主动转人工的业务处理
var kefuHandoffService = service.NewKefuHandoffService()

// handleSessionError 按错误类型输出会话相关的响应
func handleSessionError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, service.ErrSessionNotFound):
		common.NotFound(c, err.Error())
	case errors.Is(err, service.ErrVisitorNotFound):
		common.NotFound(c, service.ErrVisitorNotFound.Error())
	case errors.Is(err, service.ErrSessionNoPermission),
		errors.Is(err, service.ErrNotAgent):
		common.Forbidden(c, err.Error())
	case errors.Is(err, service.ErrSessionStatusInvalid),
		errors.Is(err, service.ErrSessionAlreadyAssigned),
		errors.Is(err, service.ErrSessionIDsRequired),
		errors.Is(err, service.ErrMessageQuoteInvalid),
		errors.Is(err, service.ErrSessionRateDisabled),
		errors.Is(err, service.ErrSessionRateScoreInvalid),
		errors.Is(err, service.ErrSessionNotClosed),
		errors.Is(err, service.ErrSessionRated),
		errors.Is(err, service.ErrMessageContentRequired),
		errors.Is(err, service.ErrMessageContentTypeInvalid),
		errors.Is(err, service.ErrAssignAgentForbidden),
		errors.Is(err, service.ErrAiModeInvalid),
		errors.Is(err, service.ErrAgentDisabled),
		errors.Is(err, service.ErrTransferReasonRequired),
		errors.Is(err, service.ErrTransferTargetInvalid),
		errors.Is(err, service.ErrTransferSelf),
		errors.Is(err, service.ErrTransferLimitReached),
		errors.Is(err, service.ErrNoAgentAvailable):
		common.BadRequest(c, err.Error())
	default:
		common.ServerError(c, fallback)
	}
}

// KefuSessionPage 分页查询会话
func KefuSessionPage(c *gin.Context) {
	userID, ok := common.CurrentUserID(c)
	if !ok {
		common.Unauthorized(c, "请先登录")
		return
	}

	var param model.KefuSessionParam
	if err := c.ShouldBindQuery(&param); err != nil {
		common.BadRequest(c, "查询参数不正确")
		return
	}

	list, count, err := kefuSessionService.Page(userID, param)
	if err != nil {
		handleSessionError(c, err, "查询会话失败")
		return
	}
	common.Page(c, list, count)
}

// KefuSessionDetail 查询会话详情
func KefuSessionDetail(c *gin.Context) {
	userID, ok := common.CurrentUserID(c)
	if !ok {
		common.Unauthorized(c, "请先登录")
		return
	}
	sessionID, err := parseID(c.Param("id"))
	if err != nil {
		common.BadRequest(c, "会话id不正确")
		return
	}

	session, err := kefuSessionService.Detail(userID, sessionID)
	if err != nil {
		handleSessionError(c, err, "查询会话失败")
		return
	}
	common.Success(c, session)
}

// KefuSessionMessages 查询会话的消息
func KefuSessionMessages(c *gin.Context) {
	userID, ok := common.CurrentUserID(c)
	if !ok {
		common.Unauthorized(c, "请先登录")
		return
	}
	sessionID, err := parseID(c.Param("id"))
	if err != nil {
		common.BadRequest(c, "会话id不正确")
		return
	}

	var param model.KefuMessageParam
	if err := c.ShouldBindQuery(&param); err != nil {
		common.BadRequest(c, "查询参数不正确")
		return
	}

	list, count, err := kefuSessionService.Messages(userID, sessionID, param)
	if err != nil {
		handleSessionError(c, err, "查询会话消息失败")
		return
	}
	common.Page(c, list, count)
}

// KefuSessionMessageSearch 在会话内搜索消息正文，命中返回最新 50 条
func KefuSessionMessageSearch(c *gin.Context) {
	userID, ok := common.CurrentUserID(c)
	if !ok {
		common.Unauthorized(c, "请先登录")
		return
	}
	sessionID, err := parseID(c.Param("id"))
	if err != nil {
		common.BadRequest(c, "会话id不正确")
		return
	}
	keyword := strings.TrimSpace(c.Query("keyword"))
	if keyword == "" {
		common.BadRequest(c, "请输入搜索关键词")
		return
	}

	list, err := kefuSessionService.SearchMessages(userID, sessionID, keyword)
	if err != nil {
		handleSessionError(c, err, "搜索会话消息失败")
		return
	}
	common.Success(c, list)
}

// KefuSessionTransfers 查询会话的转接记录，会话详情页回放分配与转接过程
func KefuSessionTransfers(c *gin.Context) {
	userID, ok := common.CurrentUserID(c)
	if !ok {
		common.Unauthorized(c, "请先登录")
		return
	}
	sessionID, err := parseID(c.Param("id"))
	if err != nil {
		common.BadRequest(c, "会话id不正确")
		return
	}

	list, err := kefuSessionService.Transfers(userID, sessionID)
	if err != nil {
		handleSessionError(c, err, "查询转接记录失败")
		return
	}
	common.Success(c, list)
}

// KefuClientSessionTyping 访客正在输入，聊天窗节流上报
func KefuClientSessionTyping(c *gin.Context) {
	sessionID, err := parseID(c.Param("id"))
	if err != nil {
		common.BadRequest(c, "会话id不正确")
		return
	}

	if err := kefuSessionService.ClientTyping(
		c.Request.Context(), sessionID, c.Query("visitorKey")); err != nil {
		handleSessionError(c, err, "上报输入状态失败")
		return
	}
	common.Success(c, nil)
}

// KefuSessionTyping 客服正在输入，工作台节流上报
func KefuSessionTyping(c *gin.Context) {
	userID, ok := common.CurrentUserID(c)
	if !ok {
		common.Unauthorized(c, "请先登录")
		return
	}
	sessionID, err := parseID(c.Param("id"))
	if err != nil {
		common.BadRequest(c, "会话id不正确")
		return
	}

	if err := kefuSessionService.AgentTyping(
		c.Request.Context(), userID, sessionID); err != nil {
		handleSessionError(c, err, "上报输入状态失败")
		return
	}
	common.Success(c, nil)
}

// KefuSessionRead 清空会话未读
func KefuSessionRead(c *gin.Context) {
	userID, ok := common.CurrentUserID(c)
	if !ok {
		common.Unauthorized(c, "请先登录")
		return
	}
	sessionID, err := parseID(c.Param("id"))
	if err != nil {
		common.BadRequest(c, "会话id不正确")
		return
	}

	if err := kefuSessionService.Read(userID, sessionID); err != nil {
		handleSessionError(c, err, "清空未读失败")
		return
	}
	common.SuccessWithMessage(c, "已读", nil)
}

// KefuSessionSendMessage 客服发送消息
func KefuSessionSendMessage(c *gin.Context) {
	userID, ok := common.CurrentUserID(c)
	if !ok {
		common.Unauthorized(c, "请先登录")
		return
	}
	sessionID, err := parseID(c.Param("id"))
	if err != nil {
		common.BadRequest(c, "会话id不正确")
		return
	}

	var form model.KefuMessageForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}
	form.SessionID = sessionID

	message, err := kefuSessionService.SendMessage(c.Request.Context(), userID, sessionID, form)
	if err != nil {
		handleSessionError(c, err, "发送消息失败")
		return
	}
	common.SuccessWithMessage(c, "发送成功", message)
}

// KefuSessionAssign 领取待接待的会话
func KefuSessionAssign(c *gin.Context) {
	userID, ok := common.CurrentUserID(c)
	if !ok {
		common.Unauthorized(c, "请先登录")
		return
	}

	var form model.KefuSessionAssignForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}
	if form.SessionID == 0 {
		common.BadRequest(c, "会话id不能为空")
		return
	}

	if err := kefuSessionService.Assign(c.Request.Context(), userID, form); err != nil {
		handleSessionError(c, err, "领取会话失败")
		return
	}
	common.SuccessWithMessage(c, "领取成功", nil)
}

// KefuSessionClose 结束会话
func KefuSessionClose(c *gin.Context) {
	userID, ok := common.CurrentUserID(c)
	if !ok {
		common.Unauthorized(c, "请先登录")
		return
	}

	var form model.KefuSessionCloseForm
	if err := c.ShouldBindJSON(&form); err != nil || len(form.SessionIDs) == 0 {
		common.BadRequest(c, "请选择要结束的会话")
		return
	}

	if err := kefuSessionService.Close(userID, form); err != nil {
		handleSessionError(c, err, "结束会话失败")
		return
	}
	common.SuccessWithMessage(c, "结束成功", nil)
}

// KefuSessionRemark 保存会话内部备注，只落库不推送
func KefuSessionRemark(c *gin.Context) {
	userID, ok := common.CurrentUserID(c)
	if !ok {
		common.Unauthorized(c, "请先登录")
		return
	}

	var form model.KefuSessionRemarkForm
	if err := c.ShouldBindJSON(&form); err != nil || form.SessionID <= 0 {
		common.BadRequest(c, "提交数据不正确")
		return
	}

	if err := kefuSessionService.SetRemark(userID, form); err != nil {
		handleSessionError(c, err, "保存备注失败")
		return
	}
	common.SuccessWithMessage(c, "保存成功", nil)
}

// KefuSessionTags 覆盖式设置会话标签
func KefuSessionTags(c *gin.Context) {
	userID, ok := common.CurrentUserID(c)
	if !ok {
		common.Unauthorized(c, "请先登录")
		return
	}

	var form model.KefuSessionTagsForm
	if err := c.ShouldBindJSON(&form); err != nil || form.SessionID <= 0 {
		common.BadRequest(c, "提交数据不正确")
		return
	}

	if err := kefuSessionService.SetTags(userID, form); err != nil {
		handleSessionError(c, err, "保存标签失败")
		return
	}
	common.SuccessWithMessage(c, "保存成功", nil)
}

// KefuSessionTransfer 转接会话，未指定目标客服时按分配算法挑一名客服
func KefuSessionTransfer(c *gin.Context) {
	userID, ok := common.CurrentUserID(c)
	if !ok {
		common.Unauthorized(c, "请先登录")
		return
	}

	var form model.KefuSessionTransferForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}
	if form.SessionID == 0 {
		common.BadRequest(c, "会话id不能为空")
		return
	}

	if err := kefuTransferService.Transfer(c.Request.Context(), userID, form); err != nil {
		handleSessionError(c, err, "转接会话失败")
		return
	}
	common.SuccessWithMessage(c, "转接成功", nil)
}

// KefuWorkbenchSummary 查询客服工作台的顶部统计
func KefuWorkbenchSummary(c *gin.Context) {
	userID, ok := common.CurrentUserID(c)
	if !ok {
		common.Unauthorized(c, "请先登录")
		return
	}

	summary, err := kefuSessionService.Summary(userID)
	if err != nil {
		handleSessionError(c, err, "查询工作台统计失败")
		return
	}
	common.Success(c, summary)
}

// KefuAgentImCredential 返回客服连接 IM 的凭据
func KefuAgentImCredential(c *gin.Context) {
	userID, ok := common.CurrentUserID(c)
	if !ok {
		common.Unauthorized(c, "请先登录")
		return
	}

	credential, err := kefuSessionService.ImCredential(c.Request.Context(), userID)
	if err != nil {
		handleSessionError(c, err, "获取 IM 连接凭据失败")
		return
	}
	common.Success(c, credential)
}

// KefuSessionAiMode 修改会话的 AI 自动回复开关，关闭时中断进行中的 AI 回复
func KefuSessionAiMode(c *gin.Context) {
	userID, ok := common.CurrentUserID(c)
	if !ok {
		common.Unauthorized(c, "请先登录")
		return
	}

	var form model.KefuSessionAiModeForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}
	if form.SessionID == 0 {
		common.BadRequest(c, "会话id不能为空")
		return
	}

	result, err := kefuAiService.SetAiMode(c.Request.Context(), userID, form)
	if err != nil {
		handleSessionError(c, err, "修改 AI 开关失败")
		return
	}
	common.SuccessWithMessage(c, "修改成功", result)
}

// KefuSessionAiInterrupt 中断会话进行中的 AI 回复
func KefuSessionAiInterrupt(c *gin.Context) {
	userID, ok := common.CurrentUserID(c)
	if !ok {
		common.Unauthorized(c, "请先登录")
		return
	}
	sessionID, err := parseID(c.Param("id"))
	if err != nil {
		common.BadRequest(c, "会话id不正确")
		return
	}

	result, err := kefuAiService.Interrupt(c.Request.Context(), userID, sessionID)
	if err != nil {
		handleSessionError(c, err, "中断 AI 回复失败")
		return
	}
	common.SuccessWithMessage(c, "已中断", result)
}

// KefuSessionAiPrompt 查看会话当前使用的 AI 提示词
func KefuSessionAiPrompt(c *gin.Context) {
	userID, ok := common.CurrentUserID(c)
	if !ok {
		common.Unauthorized(c, "请先登录")
		return
	}
	sessionID, err := parseID(c.Param("id"))
	if err != nil {
		common.BadRequest(c, "会话id不正确")
		return
	}

	result, err := kefuAiService.Prompt(userID, sessionID)
	if err != nil {
		handleSessionError(c, err, "查询 AI 提示词失败")
		return
	}
	common.Success(c, result)
}

// KefuSessionAiPromptSave 保存会话临时提示词
func KefuSessionAiPromptSave(c *gin.Context) {
	userID, ok := common.CurrentUserID(c)
	if !ok {
		common.Unauthorized(c, "请先登录")
		return
	}

	var form model.KefuSessionAiPromptForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}
	if form.SessionID == 0 {
		common.BadRequest(c, "会话id不能为空")
		return
	}

	if err := kefuAiService.SavePrompt(userID, form); err != nil {
		handleSessionError(c, err, "保存 AI 提示词失败")
		return
	}
	common.SuccessWithMessage(c, "保存成功", nil)
}

// KefuClientSessionDetail 返回会话详情，聊天窗按访客标识校验归属
func KefuClientSessionDetail(c *gin.Context) {
	sessionID, err := parseID(c.Param("id"))
	if err != nil {
		common.BadRequest(c, "会话id不正确")
		return
	}

	session, err := kefuSessionService.ClientDetail(sessionID, c.Query("visitorKey"))
	if err != nil {
		handleSessionError(c, err, "查询会话失败")
		return
	}
	common.Success(c, session)
}

// KefuClientSessionMessages 返回会话消息，聊天窗重连后用它恢复终态
func KefuClientSessionMessages(c *gin.Context) {
	sessionID, err := parseID(c.Param("id"))
	if err != nil {
		common.BadRequest(c, "会话id不正确")
		return
	}

	var param model.KefuMessageParam
	if err := c.ShouldBindQuery(&param); err != nil {
		common.BadRequest(c, "查询参数不正确")
		return
	}

	list, count, err := kefuSessionService.ClientMessages(sessionID, c.Query("visitorKey"), param)
	if err != nil {
		handleSessionError(c, err, "查询会话消息失败")
		return
	}
	common.Page(c, list, count)
}

// KefuClientSessionHandoff 访客主动转人工，聊天窗点「转人工」时调用
func KefuClientSessionHandoff(c *gin.Context) {
	sessionID, err := parseID(c.Param("id"))
	if err != nil {
		common.BadRequest(c, "会话id不正确")
		return
	}

	result, err := kefuHandoffService.HandoffByVisitor(
		c.Request.Context(), sessionID, c.Query("visitorKey"))
	if err != nil {
		handleSessionError(c, err, "转人工失败")
		return
	}
	common.SuccessWithMessage(c, "已转人工", result)
}

// KefuClientSessionRate 访客评价会话，聊天窗会话结束后调用
func KefuClientSessionRate(c *gin.Context) {
	sessionID, err := parseID(c.Param("id"))
	if err != nil {
		common.BadRequest(c, "会话id不正确")
		return
	}

	var form model.KefuSessionRateForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}

	if err := kefuSessionService.Rate(sessionID, c.Query("visitorKey"), form); err != nil {
		handleSessionError(c, err, "提交评价失败")
		return
	}
	common.SuccessWithMessage(c, "感谢您的评价", nil)
}

// KefuClientSessionRead 清空会话未读，聊天窗打开时调用
func KefuClientSessionRead(c *gin.Context) {
	sessionID, err := parseID(c.Param("id"))
	if err != nil {
		common.BadRequest(c, "会话id不正确")
		return
	}

	if err := kefuSessionService.ClientRead(sessionID, c.Query("visitorKey")); err != nil {
		handleSessionError(c, err, "清空未读失败")
		return
	}
	common.SuccessWithMessage(c, "已读", nil)
}
