package handler

import (
	"errors"

	"github.com/gin-gonic/gin"

	"admin/server/internal/common"
	"admin/server/internal/kefu/service"
	"admin/server/model"
)

// kefuChannelService 是渠道的业务处理
var kefuChannelService = service.NewKefuChannelService()

// handleChannelError 按错误类型输出渠道相关的响应
func handleChannelError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, service.ErrChannelNotFound):
		common.NotFound(c, err.Error())
	case errors.Is(err, service.ErrChannelDisabled),
		errors.Is(err, service.ErrChannelCodeRequired),
		errors.Is(err, service.ErrChannelNameRequired),
		errors.Is(err, service.ErrChannelCodeExists),
		errors.Is(err, service.ErrChannelConfigInvalid),
		errors.Is(err, service.ErrChannelStatusInvalid):
		common.BadRequest(c, err.Error())
	default:
		common.ServerError(c, fallback)
	}
}

// KefuChannelPage 分页查询渠道
func KefuChannelPage(c *gin.Context) {
	var param model.KefuChannelParam
	if err := c.ShouldBindQuery(&param); err != nil {
		common.BadRequest(c, "查询参数不正确")
		return
	}

	list, count, err := kefuChannelService.Page(param)
	if err != nil {
		common.ServerError(c, "查询渠道失败")
		return
	}
	common.Page(c, list, count)
}

// KefuChannelList 查询全部渠道，供关联链编辑弹窗多选渠道
func KefuChannelList(c *gin.Context) {
	list, err := kefuChannelService.List()
	if err != nil {
		common.ServerError(c, "查询渠道失败")
		return
	}
	common.Success(c, list)
}

// KefuChannelAdd 新增渠道
func KefuChannelAdd(c *gin.Context) {
	var form model.KefuChannelForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}

	if err := kefuChannelService.Add(form); err != nil {
		handleChannelError(c, err, "添加渠道失败")
		return
	}
	common.SuccessWithMessage(c, "添加成功", nil)
}

// KefuChannelUpdate 修改渠道
func KefuChannelUpdate(c *gin.Context) {
	var form model.KefuChannelForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}
	if form.ChannelID == 0 {
		common.BadRequest(c, "渠道id不能为空")
		return
	}

	if err := kefuChannelService.Update(form); err != nil {
		handleChannelError(c, err, "修改渠道失败")
		return
	}
	common.SuccessWithMessage(c, "修改成功", nil)
}

// KefuChannelUpdateStatus 修改渠道状态
func KefuChannelUpdateStatus(c *gin.Context) {
	var form model.KefuChannelStatusForm
	if err := c.ShouldBindJSON(&form); err != nil {
		common.BadRequest(c, "提交数据不正确")
		return
	}
	if form.ChannelID == 0 {
		common.BadRequest(c, "渠道id不能为空")
		return
	}

	if err := kefuChannelService.UpdateStatus(form.ChannelID, form.Status); err != nil {
		handleChannelError(c, err, "修改渠道状态失败")
		return
	}
	common.SuccessWithMessage(c, "修改成功", nil)
}

// KefuChannelConfig 返回渠道对外的公开配置，聊天窗免登录调用
func KefuChannelConfig(c *gin.Context) {
	channelCode := c.Param("code")
	if channelCode == "" {
		common.BadRequest(c, "渠道标识不能为空")
		return
	}

	config, err := kefuChannelService.Config(channelCode)
	if err != nil {
		handleChannelError(c, err, "查询渠道配置失败")
		return
	}
	common.Success(c, config)
}
