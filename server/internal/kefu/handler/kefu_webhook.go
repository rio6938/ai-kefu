package handler

import (
	"io"

	"github.com/gin-gonic/gin"

	"admin/server/internal/common"
	"admin/server/internal/kefu/service"
)

// kefuWebhookService 是 IM 回调的业务处理
var kefuWebhookService = service.NewKefuWebhookService()

// webhookBodyLimit 是回调报文的大小上限
const webhookBodyLimit = 4 << 20

// readWebhookBody 校验回调密钥并读取报文，校验或读取失败时已经输出响应
func readWebhookBody(c *gin.Context) ([]byte, bool) {
	token := c.GetHeader("token")
	if token == "" {
		token = c.Query("token")
	}
	if !kefuWebhookService.ValidateToken(token) {
		common.Unauthorized(c, "回调密钥不正确")
		return nil, false
	}

	body, err := io.ReadAll(io.LimitReader(c.Request.Body, webhookBodyLimit))
	if err != nil {
		common.BadRequest(c, "读取回调数据失败")
		return nil, false
	}
	return body, true
}

// KefuWebhookMessage 接收 IM 的消息回调，先落库再返回；
// 校验共享密钥，密钥可以在请求头 token 或查询参数 token 里传
func KefuWebhookMessage(c *gin.Context) {
	if event := c.Query("event"); event != "" && event != "msg.notify" {
		common.BadRequest(c, "不支持的回调事件")
		return
	}

	body, ok := readWebhookBody(c)
	if !ok {
		return
	}

	kefuWebhookService.HandleMessage(c.Request.Context(), body)
	common.Success(c, nil)
}

// KefuWebhookOnline 接收 IM 的在线状态回调，更新客服在线缓存并补充分配排队会话
func KefuWebhookOnline(c *gin.Context) {
	if event := c.Query("event"); event != "" && event != "user.onlinestatus" {
		common.BadRequest(c, "不支持的回调事件")
		return
	}

	body, ok := readWebhookBody(c)
	if !ok {
		return
	}

	kefuWebhookService.HandleOnline(c.Request.Context(), body)
	common.Success(c, nil)
}
