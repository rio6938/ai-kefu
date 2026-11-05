package handler

import (
	"github.com/gin-gonic/gin"

	"admin/server/internal/common"
	"admin/server/internal/kefu/service"
	"admin/server/model"
)

// 邮件 AI 与统计的业务处理
var (
	// kefuMailAiService 是邮件 AI 草稿的业务处理
	kefuMailAiService = service.NewKefuMailAiService()
	// kefuMailAnalysisService 是邮件统计的业务处理
	kefuMailAnalysisService = service.NewKefuMailAnalysisService()
)

// KefuMailDraftDetail 查询当前会话的邮件草稿或待发送记录
func KefuMailDraftDetail(c *gin.Context) {
	sessionID, err := parseID(c.Param("id"))
	if err != nil || sessionID <= 0 {
		common.BadRequest(c, "会话 id 不正确")
		return
	}
	result, err := kefuMailAiService.GetDraft(currentMailUserID(c), sessionID)
	if err != nil {
		handleMailboxError(c, err, "查询邮件草稿失败")
		return
	}
	common.Success(c, result)
}

// KefuMailDraftSend 采用并发送邮件草稿，可带编辑后的正文与附件
func KefuMailDraftSend(c *gin.Context) {
	sessionID, err := parseID(c.Param("id"))
	if err != nil || sessionID <= 0 {
		common.BadRequest(c, "会话 id 不正确")
		return
	}
	var form model.KefuMailDraftSendForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}
	form.SessionID = sessionID
	if _, err := kefuMailAiService.SendDraftByUser(c.Request.Context(),
		currentMailUserID(c), form); err != nil {
		handleMailboxError(c, err, "发送邮件失败")
		return
	}
	common.SuccessWithMessage(c, "发送成功", nil)
}

// KefuMailDraftRegenerate 重新生成邮件草稿
func KefuMailDraftRegenerate(c *gin.Context) {
	sessionID, err := parseID(c.Param("id"))
	if err != nil || sessionID <= 0 {
		common.BadRequest(c, "会话 id 不正确")
		return
	}
	result, err := kefuMailAiService.Regenerate(c.Request.Context(),
		currentMailUserID(c), sessionID)
	if err != nil {
		handleMailboxError(c, err, "重新生成失败")
		return
	}
	common.Success(c, result)
}

// KefuMailDraftDiscard 丢弃邮件草稿或取消待发送
func KefuMailDraftDiscard(c *gin.Context) {
	sessionID, err := parseID(c.Param("id"))
	if err != nil || sessionID <= 0 {
		common.BadRequest(c, "会话 id 不正确")
		return
	}
	if err := kefuMailAiService.Discard(currentMailUserID(c), sessionID); err != nil {
		handleMailboxError(c, err, "丢弃草稿失败")
		return
	}
	common.SuccessWithMessage(c, "已丢弃", nil)
}

// KefuMailAnalysis 邮件统计聚合
func KefuMailAnalysis(c *gin.Context) {
	var param model.KefuMailAnalysisParam
	if err := c.ShouldBindQuery(&param); err != nil {
		common.BadRequest(c, "查询参数不正确")
		return
	}
	result, err := kefuMailAnalysisService.Overview(param)
	if err != nil {
		common.ServerError(c, "查询邮件统计失败")
		return
	}
	common.Success(c, result)
}
