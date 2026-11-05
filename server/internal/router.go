package internal

import (
	"github.com/gin-gonic/gin"

	"admin/server/internal/kefu"
	"admin/server/internal/middleware"
	"admin/server/internal/system"
	"admin/server/internal/user"
)

// Register 装配全局中间件并注册所有模块的路由
func Register(r *gin.Engine) {
	// 全局中间件：访问日志、panic 恢复、操作日志
	r.Use(middleware.Logger(), gin.Recovery(), middleware.OperationRecord())

	system.Register(r.Group("/admin"))
	// 文件接口按 eleadmin 约定放在根路径下
	system.RegisterFile(r.Group("/file"))
	// 前台用户的注册与登录，不需要后台 token
	user.RegisterAuth(r.Group("/user"))
	// 后台的用户管理
	user.Register(r.Group("/admin/user-center"))
	// 后台的前台用户日志查询
	user.RegisterRecord(r.Group("/admin"))

	// 客服模块：客户端免登录接口、IM 回调、客服端接口与 OpenAI 兼容接口
	kefu.Register(r.Group("/kefu"))
	kefu.RegisterWebhook(r.Group("/kefu/im/webhook"))
	kefu.RegisterAdmin(r.Group("/admin/kefu"))
	kefu.RegisterOpenAI(r.Group("/v1"))
}
