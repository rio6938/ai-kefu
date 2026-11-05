package handler

import (
	"github.com/gin-gonic/gin"

	"admin/server/internal/common"
	"admin/server/internal/user/service"
	"admin/server/model"
)

// userRecordService 是前台用户登录日志与操作日志的业务处理
var userRecordService = service.NewUserRecordService()

// UserLoginRecordPage 分页查询登录日志
func UserLoginRecordPage(c *gin.Context) {
	var param model.UserLoginRecordParam
	if err := c.ShouldBindQuery(&param); err != nil {
		common.BadRequest(c, "查询参数不正确")
		return
	}

	list, count, err := userRecordService.PageLoginRecords(param, param.PageParam)
	if err != nil {
		common.ServerError(c, "查询登录日志失败")
		return
	}
	common.Page(c, list, count)
}

// UserLoginRecordList 查询登录日志列表
func UserLoginRecordList(c *gin.Context) {
	var param model.UserLoginRecordParam
	if err := c.ShouldBindQuery(&param); err != nil {
		common.BadRequest(c, "查询参数不正确")
		return
	}

	list, err := userRecordService.ListLoginRecords(param)
	if err != nil {
		common.ServerError(c, "查询登录日志失败")
		return
	}
	common.Success(c, list)
}

// UserLoginRecordRemove 按 id 删除登录日志
func UserLoginRecordRemove(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		common.BadRequest(c, "登录日志id不正确")
		return
	}

	if err := userRecordService.RemoveLoginRecords([]int64{id}); err != nil {
		common.ServerError(c, "删除登录日志失败")
		return
	}
	common.SuccessWithMessage(c, "删除成功", nil)
}

// UserLoginRecordRemoveBatch 批量删除登录日志
func UserLoginRecordRemoveBatch(c *gin.Context) {
	var ids []int64
	if err := c.ShouldBindJSON(&ids); err != nil || len(ids) == 0 {
		common.BadRequest(c, "请选择要删除的登录日志")
		return
	}

	if err := userRecordService.RemoveLoginRecords(ids); err != nil {
		common.ServerError(c, "删除登录日志失败")
		return
	}
	common.SuccessWithMessage(c, "删除成功", nil)
}

// UserOperationRecordPage 分页查询操作日志
func UserOperationRecordPage(c *gin.Context) {
	var param model.UserOperationRecordParam
	if err := c.ShouldBindQuery(&param); err != nil {
		common.BadRequest(c, "查询参数不正确")
		return
	}

	list, count, err := userRecordService.PageOperationRecords(param, param.PageParam)
	if err != nil {
		common.ServerError(c, "查询操作日志失败")
		return
	}
	common.Page(c, list, count)
}

// UserOperationRecordList 查询操作日志列表
func UserOperationRecordList(c *gin.Context) {
	var param model.UserOperationRecordParam
	if err := c.ShouldBindQuery(&param); err != nil {
		common.BadRequest(c, "查询参数不正确")
		return
	}

	list, err := userRecordService.ListOperationRecords(param)
	if err != nil {
		common.ServerError(c, "查询操作日志失败")
		return
	}
	common.Success(c, list)
}

// UserOperationRecordRemove 按 id 删除操作日志
func UserOperationRecordRemove(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		common.BadRequest(c, "操作日志id不正确")
		return
	}

	if err := userRecordService.RemoveOperationRecords([]int64{id}); err != nil {
		common.ServerError(c, "删除操作日志失败")
		return
	}
	common.SuccessWithMessage(c, "删除成功", nil)
}

// UserOperationRecordRemoveBatch 批量删除操作日志
func UserOperationRecordRemoveBatch(c *gin.Context) {
	var ids []int64
	if err := c.ShouldBindJSON(&ids); err != nil || len(ids) == 0 {
		common.BadRequest(c, "请选择要删除的操作日志")
		return
	}

	if err := userRecordService.RemoveOperationRecords(ids); err != nil {
		common.ServerError(c, "删除操作日志失败")
		return
	}
	common.SuccessWithMessage(c, "删除成功", nil)
}
