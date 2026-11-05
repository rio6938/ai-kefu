package handler

import (
	"errors"

	"github.com/gin-gonic/gin"

	"admin/server/internal/common"
	"admin/server/internal/kefu/service"
	"admin/server/model"
)

// kefuRouteService 是关联链的业务处理
var kefuRouteService = service.NewKefuRouteService()

// handleRouteError 按错误类型输出关联链相关的响应
func handleRouteError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, service.ErrRouteNotFound):
		common.NotFound(c, err.Error())
	case errors.Is(err, service.ErrRouteNameRequired),
		errors.Is(err, service.ErrRouteNameExists),
		errors.Is(err, service.ErrRoutePromptRequired),
		errors.Is(err, service.ErrRoutePromptNotFound),
		errors.Is(err, service.ErrRouteCategoryNotFound),
		errors.Is(err, service.ErrRouteChannelNotFound),
		errors.Is(err, service.ErrRouteChannelBound),
		errors.Is(err, service.ErrRouteStatusInvalid):
		common.BadRequest(c, err.Error())
	default:
		common.ServerError(c, fallback)
	}
}

// KefuRoutePage 分页查询关联链
func KefuRoutePage(c *gin.Context) {
	var param model.KefuRouteParam
	if err := c.ShouldBindQuery(&param); err != nil {
		common.BadRequest(c, "查询参数不正确")
		return
	}

	list, count, err := kefuRouteService.Page(param)
	if err != nil {
		common.ServerError(c, "查询关联链失败")
		return
	}
	common.Page(c, list, count)
}

// KefuRouteList 查询启用中的关联链
func KefuRouteList(c *gin.Context) {
	list, err := kefuRouteService.List()
	if err != nil {
		common.ServerError(c, "查询关联链失败")
		return
	}
	common.Success(c, list)
}

// KefuRouteDetail 查询关联链详情
func KefuRouteDetail(c *gin.Context) {
	routeID, err := parseID(c.Param("id"))
	if err != nil {
		common.BadRequest(c, "关联链id不正确")
		return
	}

	record, err := kefuRouteService.Detail(routeID)
	if err != nil {
		handleRouteError(c, err, "查询关联链失败")
		return
	}
	common.Success(c, record)
}

// KefuRouteAdd 新增关联链
func KefuRouteAdd(c *gin.Context) {
	var form model.KefuRouteForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}

	if err := kefuRouteService.Add(form); err != nil {
		handleRouteError(c, err, "添加关联链失败")
		return
	}
	common.SuccessWithMessage(c, "添加成功", nil)
}

// KefuRouteUpdate 修改关联链
func KefuRouteUpdate(c *gin.Context) {
	var form model.KefuRouteForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}
	if form.RouteID == 0 {
		common.BadRequest(c, "关联链id不能为空")
		return
	}

	if err := kefuRouteService.Update(form); err != nil {
		handleRouteError(c, err, "修改关联链失败")
		return
	}
	common.SuccessWithMessage(c, "修改成功", nil)
}

// KefuRouteUpdateStatus 修改关联链状态
func KefuRouteUpdateStatus(c *gin.Context) {
	var form model.KefuRouteStatusForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}
	if form.RouteID == 0 {
		common.BadRequest(c, "关联链id不能为空")
		return
	}

	if err := kefuRouteService.UpdateStatus(form.RouteID, form.Status); err != nil {
		handleRouteError(c, err, "修改关联链状态失败")
		return
	}
	common.SuccessWithMessage(c, "修改成功", nil)
}

// KefuRouteRemove 删除关联链
func KefuRouteRemove(c *gin.Context) {
	routeID, err := parseID(c.Param("id"))
	if err != nil {
		common.BadRequest(c, "关联链id不正确")
		return
	}

	if err := kefuRouteService.Remove(routeID); err != nil {
		handleRouteError(c, err, "删除关联链失败")
		return
	}
	common.SuccessWithMessage(c, "删除成功", nil)
}
