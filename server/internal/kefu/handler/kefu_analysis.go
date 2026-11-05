package handler

import (
	"errors"

	"github.com/gin-gonic/gin"

	"admin/server/internal/common"
	"admin/server/internal/kefu/service"
	"admin/server/model"
)

// kefuAnalysisService 是分析页的业务处理
var kefuAnalysisService = service.NewKefuAnalysisService()

// handleAnalysisError 按错误类型输出分析相关的响应
func handleAnalysisError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, service.ErrNotAgent):
		common.Forbidden(c, err.Error())
	default:
		common.ServerError(c, fallback)
	}
}

// KefuAnalysisOverview 查询管理员视角的分析数据
func KefuAnalysisOverview(c *gin.Context) {
	var param model.KefuAnalysisParam
	if err := c.ShouldBindQuery(&param); err != nil {
		common.BadRequest(c, "查询参数不正确")
		return
	}

	data, err := kefuAnalysisService.Overview(param)
	if err != nil {
		handleAnalysisError(c, err, "查询分析数据失败")
		return
	}
	common.Success(c, data)
}

// KefuAnalysisAgent 查询客服视角的分析数据
func KefuAnalysisAgent(c *gin.Context) {
	userID, ok := common.CurrentUserID(c)
	if !ok {
		common.Unauthorized(c, "请先登录")
		return
	}

	var param model.KefuAnalysisParam
	if err := c.ShouldBindQuery(&param); err != nil {
		common.BadRequest(c, "查询参数不正确")
		return
	}

	data, err := kefuAnalysisService.AgentView(userID, param)
	if err != nil {
		handleAnalysisError(c, err, "查询分析数据失败")
		return
	}
	common.Success(c, data)
}
