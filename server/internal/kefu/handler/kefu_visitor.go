package handler

import (
	"errors"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"admin/server/internal/common"
	"admin/server/internal/kefu/service"
	"admin/server/model"
	"admin/server/pkg/log"
)

// kefuVisitorService 是访客的业务处理
var kefuVisitorService = service.NewKefuVisitorService()

// handleVisitorError 按错误类型输出访客相关的响应
func handleVisitorError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, service.ErrVisitorNotFound):
		common.NotFound(c, err.Error())
	case errors.Is(err, service.ErrChannelNotFound):
		common.NotFound(c, service.ErrChannelNotFound.Error())
	case errors.Is(err, service.ErrChannelDisabled),
		errors.Is(err, service.ErrChannelCodeRequired),
		errors.Is(err, service.ErrVisitorEmailInvalid):
		common.BadRequest(c, err.Error())
	default:
		log.Logger.Error("kefu bootstrap failed", zap.Error(err))
		common.ServerError(c, fallback)
	}
}

// KefuVisitorTrack 上报访客浏览轨迹，开关未开启时后端直接忽略
func KefuVisitorTrack(c *gin.Context) {
	var form model.KefuBrowseTrackForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}

	if err := kefuVisitorService.TrackBrowse(form); err != nil {
		handleVisitorError(c, err, "上报浏览轨迹失败")
		return
	}
	common.Success(c, nil)
}

// KefuVisitorBootstrap 处理访客接入，返回 IM 连接凭据与当前会话
func KefuVisitorBootstrap(c *gin.Context) {
	var form model.KefuBootstrapForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}

	result, err := kefuVisitorService.Bootstrap(c.Request.Context(), form, c.ClientIP())
	if err != nil {
		handleVisitorError(c, err, "接入客服失败")
		return
	}
	common.Success(c, result)
}
