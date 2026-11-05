package server

import (
	"github.com/gin-gonic/gin"

	"admin/server/pkg/log"
)

// Engine 是全局 gin 引擎，Init 之后可用
var Engine *gin.Engine

// Init 按运行模式创建 gin 引擎；全局中间件由 internal.Register 装配
func Init(mode string) *gin.Engine {
	gin.SetMode(mode)
	// gin 的路由调试信息与 panic 信息转到日志通道，避免直接输出到控制台
	gin.DefaultWriter = log.GinWriter()
	gin.DefaultErrorWriter = log.GinErrorWriter()

	e := gin.New()
	Engine = e
	return e
}
