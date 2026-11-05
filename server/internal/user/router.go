package user

import (
	"github.com/gin-gonic/gin"

	"admin/server/internal/middleware"
	"admin/server/internal/user/handler"
	"admin/server/internal/user/service"
)

// Register 注册后台用户管理的路由，路径前缀由上层传入
func Register(rg *gin.RouterGroup) {
	auth := rg.Group("", middleware.Auth())
	auth.GET("/page", middleware.Permission("sys:user-center:list"), handler.UserPage)
	auth.GET("/existence", middleware.Permission("sys:user-center:list"), handler.UserExistence)
	auth.POST("", middleware.Permission("sys:user-center:save"), handler.UserAdd)
	auth.PUT("", middleware.Permission("sys:user-center:update"), handler.UserUpdate)
	auth.PUT("/status", middleware.Permission("sys:user-center:update"), handler.UserUpdateStatus)
	auth.PUT("/password", middleware.Permission("sys:user-center:update"), handler.UserUpdatePassword)
	auth.DELETE("/batch", middleware.Permission("sys:user-center:remove"), handler.UserRemoveBatch)
	auth.GET("/:id", middleware.Permission("sys:user-center:list"), handler.UserDetail)
	auth.DELETE("/:id", middleware.Permission("sys:user-center:remove"), handler.UserRemove)
}

// RegisterRecord 注册前台用户日志的查询路由，路径前缀由上层传入
func RegisterRecord(rg *gin.RouterGroup) {
	// 前台用户操作日志写入能力注册到中间件
	middleware.SetUserOperationRecorder(service.NewUserRecordService().AddOperationRecord)

	auth := rg.Group("", middleware.Auth())

	// 登录日志
	loginRecord := auth.Group("/user-login-record")
	loginRecord.GET("/page", middleware.Permission("user:login-record:list"), handler.UserLoginRecordPage)
	loginRecord.GET("", middleware.Permission("user:login-record:list"), handler.UserLoginRecordList)
	loginRecord.DELETE("/batch", middleware.Permission("user:login-record:remove"), handler.UserLoginRecordRemoveBatch)
	loginRecord.DELETE("/:id", middleware.Permission("user:login-record:remove"), handler.UserLoginRecordRemove)

	// 操作日志
	operationRecord := auth.Group("/user-operation-record")
	operationRecord.GET("/page", middleware.Permission("user:operation-record:list"), handler.UserOperationRecordPage)
	operationRecord.GET("", middleware.Permission("user:operation-record:list"), handler.UserOperationRecordList)
	operationRecord.DELETE("/batch", middleware.Permission("user:operation-record:remove"), handler.UserOperationRecordRemoveBatch)
	operationRecord.DELETE("/:id", middleware.Permission("user:operation-record:remove"), handler.UserOperationRecordRemove)
}

// RegisterAuth 注册前台用户注册与登录的路由，不需要登录
func RegisterAuth(rg *gin.RouterGroup) {
	// 前台可读的公开配置，如是否开放注册、是否开放找回密码
	rg.GET("/config", handler.UserConfig)

	rg.POST("/register", handler.UserRegister)
	rg.POST("/login", handler.UserLogin)

	// 前台用户的登录态接口，需要前台用户 token；
	// 前台用户没有后台角色与菜单，不参与后台接口权限校验
	auth := rg.Group("", middleware.UserAuth())
	auth.GET("/auth/user", handler.UserAuthUser)
	auth.POST("/logout", handler.UserLogout)
}
