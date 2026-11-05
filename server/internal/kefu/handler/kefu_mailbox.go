package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"admin/server/internal/common"
	"admin/server/internal/kefu/service"
	"admin/server/model"
)

// 邮箱配置相关的业务处理
var (
	// kefuMailboxService 是邮箱配置的业务处理
	kefuMailboxService = service.NewKefuMailboxService()
	// kefuMailSyncService 是 IMAP 收信的业务处理
	kefuMailSyncService = service.NewKefuMailSyncService()
	// kefuMailInboundService 是入站邮件的业务处理
	kefuMailInboundService = service.NewKefuMailInboundService()
	// kefuMailLogService 是发信记录的业务处理
	kefuMailLogService = service.NewKefuMailLogService()
	// kefuMailSendService 是出站邮件的业务处理
	kefuMailSendService = service.NewKefuMailSendService()
)

// currentMailUserID 返回当前登录用户 id，鉴权中间件已保证存在
func currentMailUserID(c *gin.Context) int64 {
	userID, _ := common.CurrentUserID(c)
	return userID
}

// handleMailboxError 按错误类型输出邮箱相关的响应
func handleMailboxError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, service.ErrMailboxNotFound),
		errors.Is(err, service.ErrMailLogNotFound),
		errors.Is(err, service.ErrMailDraftNotFound):
		common.NotFound(c, err.Error())
	case errors.Is(err, service.ErrMailChannelOnly),
		errors.Is(err, service.ErrMailEmailRequired),
		errors.Is(err, service.ErrMailEmailInvalid),
		errors.Is(err, service.ErrMailEmailExists),
		errors.Is(err, service.ErrMailProviderInvalid),
		errors.Is(err, service.ErrMailAuthTypeInvalid),
		errors.Is(err, service.ErrMailReplyModeInvalid),
		errors.Is(err, service.ErrMailLanguageInvalid),
		errors.Is(err, service.ErrMailPasswordRequired),
		errors.Is(err, service.ErrMailCredentialsMissing),
		errors.Is(err, service.ErrMailOAuthRequired),
		errors.Is(err, service.ErrMailOAuthNotConfigured),
		errors.Is(err, service.ErrMailOAuthStateInvalid),
		errors.Is(err, service.ErrMailDisabled),
		errors.Is(err, service.ErrMailboxStatusInvalid),
		errors.Is(err, service.ErrMailSyncBusy),
		errors.Is(err, service.ErrMailSyncDisabled),
		errors.Is(err, service.ErrMailAttachmentTooLarge),
		errors.Is(err, service.ErrMailAttachmentNotFound),
		errors.Is(err, service.ErrMailResendInvalid),
		errors.Is(err, service.ErrMailDraftContentRequired),
		errors.Is(err, service.ErrMailDraftStatusInvalid),
		errors.Is(err, service.ErrMailSendForbidden):
		common.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrSessionNoPermission),
		errors.Is(err, service.ErrNotAgent):
		common.Forbidden(c, err.Error())
	default:
		common.ServerError(c, fallback)
	}
}

// KefuMailProviderList 返回邮箱服务商预设
func KefuMailProviderList(c *gin.Context) {
	common.Success(c, kefuMailboxService.Providers())
}

// KefuMailboxDetail 查询渠道绑定的邮箱配置，凭据脱敏
func KefuMailboxDetail(c *gin.Context) {
	channelID, err := parseID(c.Param("id"))
	if err != nil || channelID <= 0 {
		common.BadRequest(c, "渠道 id 不正确")
		return
	}
	result, err := kefuMailboxService.Get(channelID)
	if err != nil {
		handleMailboxError(c, err, "查询邮箱配置失败")
		return
	}
	common.Success(c, result)
}

// KefuMailboxSave 新增或修改渠道绑定的邮箱配置
func KefuMailboxSave(c *gin.Context) {
	channelID, err := parseID(c.Param("id"))
	if err != nil || channelID <= 0 {
		common.BadRequest(c, "渠道 id 不正确")
		return
	}
	var form model.KefuMailboxForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}
	if err := kefuMailboxService.Save(channelID, form); err != nil {
		handleMailboxError(c, err, "保存邮箱配置失败")
		return
	}
	common.SuccessWithMessage(c, "保存成功", nil)
}

// KefuMailboxTest 测试 IMAP 与 SMTP 连接，不发测试邮件
func KefuMailboxTest(c *gin.Context) {
	channelID, err := parseID(c.Param("id"))
	if err != nil || channelID <= 0 {
		common.BadRequest(c, "渠道 id 不正确")
		return
	}
	var form model.KefuMailboxTestForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}
	result, err := kefuMailboxService.Test(c.Request.Context(), channelID, form)
	if err != nil {
		handleMailboxError(c, err, "连接测试失败")
		return
	}
	common.Success(c, result)
}

// KefuMailboxSync 立即同步一次收件箱
func KefuMailboxSync(c *gin.Context) {
	channelID, err := parseID(c.Param("id"))
	if err != nil || channelID <= 0 {
		common.BadRequest(c, "渠道 id 不正确")
		return
	}
	result, err := kefuMailSyncService.SyncChannel(c.Request.Context(), channelID)
	if err != nil {
		handleMailboxError(c, err, "同步失败")
		return
	}
	common.SuccessWithMessage(c, "同步完成", result)
}

// KefuMailboxOAuthURL 获取 OAuth2 授权地址
func KefuMailboxOAuthURL(c *gin.Context) {
	channelID, err := parseID(c.Param("id"))
	if err != nil || channelID <= 0 {
		common.BadRequest(c, "渠道 id 不正确")
		return
	}
	result, err := kefuMailboxService.OAuthURL(channelID, currentMailUserID(c))
	if err != nil {
		handleMailboxError(c, err, "获取授权地址失败")
		return
	}
	common.Success(c, result)
}

// KefuMailboxOAuthRevoke 解除 OAuth2 授权
func KefuMailboxOAuthRevoke(c *gin.Context) {
	channelID, err := parseID(c.Param("id"))
	if err != nil || channelID <= 0 {
		common.BadRequest(c, "渠道 id 不正确")
		return
	}
	if err := kefuMailboxService.RevokeOAuth(channelID); err != nil {
		handleMailboxError(c, err, "解除授权失败")
		return
	}
	common.SuccessWithMessage(c, "已解除授权", nil)
}

// KefuMailOAuthCallback 处理 OAuth2 回调，免登录，返回一个会自动关闭的页面
func KefuMailOAuthCallback(c *gin.Context) {
	err := kefuMailboxService.OAuthCallback(c.Request.Context(),
		c.Query("state"), c.Query("code"))
	if err != nil {
		writeOAuthPage(c, "授权失败："+err.Error())
		return
	}
	writeOAuthPage(c, "授权成功，请关闭本窗口并回到邮箱设置刷新")
}

// writeOAuthPage 输出授权结果页面，成功后自动关闭窗口
func writeOAuthPage(c *gin.Context, message string) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Status(http.StatusOK)
	_, _ = c.Writer.WriteString(fmt.Sprintf(
		`<!doctype html><html lang="zh-CN"><head><meta charset="utf-8">`+
			`<title>邮箱授权</title></head><body style="font-family:sans-serif;padding:32px">`+
			`<p>%s</p><script>setTimeout(function(){window.close()},1500)</script></body></html>`,
		strings.ReplaceAll(message, "<", "&lt;")))
}

// KefuMailLogReparse 按收信记录重新解析一封邮件
func KefuMailLogReparse(c *gin.Context) {
	mailID, err := parseID(c.Param("id"))
	if err != nil || mailID <= 0 {
		common.BadRequest(c, "邮件 id 不正确")
		return
	}
	if err := kefuMailInboundService.Rehandle(c.Request.Context(), mailID); err != nil {
		handleMailboxError(c, err, "重新解析失败")
		return
	}
	common.SuccessWithMessage(c, "重新解析完成", nil)
}

// KefuMailLogPage 分页查询发信记录
func KefuMailLogPage(c *gin.Context) {
	var param model.KefuMailLogParam
	if err := c.ShouldBindQuery(&param); err != nil {
		common.BadRequest(c, "查询参数不正确")
		return
	}
	list, count, err := kefuMailLogService.Page(param)
	if err != nil {
		common.ServerError(c, "查询发信记录失败")
		return
	}
	common.Page(c, list, count)
}

// KefuMailLogResend 重发失败的邮件
func KefuMailLogResend(c *gin.Context) {
	mailID, err := parseID(c.Param("id"))
	if err != nil || mailID <= 0 {
		common.BadRequest(c, "邮件 id 不正确")
		return
	}
	if err := kefuMailSendService.Resend(c.Request.Context(), mailID); err != nil {
		if errors.Is(err, service.ErrMailLogNotFound) ||
			errors.Is(err, service.ErrMailResendInvalid) {
			handleMailboxError(c, err, "重发失败")
			return
		}
		common.BadRequest(c, err.Error())
		return
	}
	common.SuccessWithMessage(c, "重发成功", nil)
}
