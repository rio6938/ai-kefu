package handler

import (
	"github.com/gin-gonic/gin"

	"admin/server/internal/common"
	"admin/server/internal/system/service"
	"admin/server/model"
)

// sysRecordService 是登录日志与操作日志的业务处理
var sysRecordService = service.NewSysRecordService()

// SysLoginRecordPage 分页查询登录日志
func SysLoginRecordPage(c *gin.Context) {
	var param model.LoginRecordParam
	if err := c.ShouldBindQuery(&param); err != nil {
		common.BadRequest(c, "查询参数不正确")
		return
	}

	list, count, err := sysRecordService.PageLoginRecords(param, param.PageParam)
	if err != nil {
		common.ServerError(c, "查询登录日志失败")
		return
	}
	common.Page(c, list, count)
}

// SysLoginRecordList 查询登录日志列表
func SysLoginRecordList(c *gin.Context) {
	var param model.LoginRecordParam
	if err := c.ShouldBindQuery(&param); err != nil {
		common.BadRequest(c, "查询参数不正确")
		return
	}

	list, err := sysRecordService.ListLoginRecords(param)
	if err != nil {
		common.ServerError(c, "查询登录日志失败")
		return
	}
	common.Success(c, list)
}

// SysLoginRecordRemove 按 id 删除登录日志
func SysLoginRecordRemove(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		common.BadRequest(c, "登录日志id不正确")
		return
	}

	if err := sysRecordService.RemoveLoginRecords([]int64{id}); err != nil {
		common.ServerError(c, "删除登录日志失败")
		return
	}
	common.SuccessWithMessage(c, "删除成功", nil)
}

// SysLoginRecordRemoveBatch 批量删除登录日志
func SysLoginRecordRemoveBatch(c *gin.Context) {
	ids, ok := parseIDs(c)
	if !ok {
		return
	}

	if err := sysRecordService.RemoveLoginRecords(ids); err != nil {
		common.ServerError(c, "删除登录日志失败")
		return
	}
	common.SuccessWithMessage(c, "删除成功", nil)
}

// SysOperationRecordPage 分页查询操作日志
func SysOperationRecordPage(c *gin.Context) {
	var param model.OperationRecordParam
	if err := c.ShouldBindQuery(&param); err != nil {
		common.BadRequest(c, "查询参数不正确")
		return
	}

	list, count, err := sysRecordService.PageOperationRecords(param, param.PageParam)
	if err != nil {
		common.ServerError(c, "查询操作日志失败")
		return
	}
	common.Page(c, list, count)
}

// SysOperationRecordList 查询操作日志列表
func SysOperationRecordList(c *gin.Context) {
	var param model.OperationRecordParam
	if err := c.ShouldBindQuery(&param); err != nil {
		common.BadRequest(c, "查询参数不正确")
		return
	}

	list, err := sysRecordService.ListOperationRecords(param)
	if err != nil {
		common.ServerError(c, "查询操作日志失败")
		return
	}
	common.Success(c, list)
}

// SysOperationRecordRemove 按 id 删除操作日志
func SysOperationRecordRemove(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		common.BadRequest(c, "操作日志id不正确")
		return
	}

	if err := sysRecordService.RemoveOperationRecords([]int64{id}); err != nil {
		common.ServerError(c, "删除操作日志失败")
		return
	}
	common.SuccessWithMessage(c, "删除成功", nil)
}

// SysOperationRecordRemoveBatch 批量删除操作日志
func SysOperationRecordRemoveBatch(c *gin.Context) {
	ids, ok := parseIDs(c)
	if !ok {
		return
	}

	if err := sysRecordService.RemoveOperationRecords(ids); err != nil {
		common.ServerError(c, "删除操作日志失败")
		return
	}
	common.SuccessWithMessage(c, "删除成功", nil)
}
