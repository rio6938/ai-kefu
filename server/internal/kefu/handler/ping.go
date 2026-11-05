package handler

import (
	"github.com/gin-gonic/gin"

	"admin/server/internal/common"
)

// KefuPing 是客服模块的连通性检查，用于确认路由与鉴权已经装配完成
func KefuPing(c *gin.Context) {
	common.Success(c, "pong")
}
