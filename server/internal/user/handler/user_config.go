package handler

import (
	"strings"

	"github.com/gin-gonic/gin"

	"admin/server/internal/common"
	"admin/server/model"
	"admin/server/pkg/setting"
)

// UserConfig 下发允许公开的前台配置项，不需要登录；
// keys 为逗号分隔的配置键，为空时返回全部公开配置，密钥类配置不会下发
func UserConfig(c *gin.Context) {
	var param model.ConfigPublicParam
	if err := c.ShouldBindQuery(&param); err != nil {
		common.BadRequest(c, "查询参数不正确")
		return
	}

	keys := make([]string, 0)
	if value := strings.TrimSpace(param.Keys); value != "" {
		keys = strings.Split(value, ",")
	}
	common.Success(c, setting.PublicValues(keys))
}
