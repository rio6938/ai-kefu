package handler

import (
	"errors"

	"github.com/gin-gonic/gin"

	"admin/server/internal/common"
	"admin/server/internal/system/service"
	"admin/server/model"
)

// sysConfigService 是配置中心的业务处理
var sysConfigService = service.NewSysConfigService()

// handleConfigError 按错误类型输出配置相关的响应
func handleConfigError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, service.ErrConfigNotFound):
		common.NotFound(c, err.Error())
	case errors.Is(err, service.ErrConfigGroupRequired),
		errors.Is(err, service.ErrConfigItemRequired),
		errors.Is(err, service.ErrConfigValueInvalid):
		common.BadRequest(c, err.Error())
	default:
		common.ServerError(c, fallback)
	}
}

// SysConfigList 查询配置项列表
func SysConfigList(c *gin.Context) {
	var param model.ConfigParam
	if err := c.ShouldBindQuery(&param); err != nil {
		common.BadRequest(c, "查询参数不正确")
		return
	}

	list, err := sysConfigService.List(param.GroupCode)
	if err != nil {
		common.ServerError(c, "查询配置失败")
		return
	}
	common.Success(c, list)
}

// SysConfigSaveGroup 按分组保存配置，一个分组一次提交
func SysConfigSaveGroup(c *gin.Context) {
	var form model.ConfigGroupForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}

	userID, _ := common.CurrentUserID(c)
	if err := sysConfigService.SaveGroup(form, userID); err != nil {
		handleConfigError(c, err, "保存配置失败")
		return
	}
	common.SuccessWithMessage(c, "保存成功", nil)
}
