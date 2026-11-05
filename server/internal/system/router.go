package system

import (
	"github.com/gin-gonic/gin"

	"admin/server/internal/middleware"
	"admin/server/internal/system/handler"
	"admin/server/internal/system/service"
)

// Register 注册 system 模块的路由，路径前缀由上层传入
func Register(rg *gin.RouterGroup) {
	// 操作日志写入能力注册到中间件
	middleware.SetOperationRecorder(service.NewSysRecordService().AddOperationRecord)
	// 接口权限校验能力注册到中间件，需要早于本模块与其它模块的登录路由注册
	middleware.SetAuthorityLoader(service.NewSysRbacService().LoadAuthorities)

	rg.GET("/ping", handler.Ping)

	// 图形验证码和登录，不需要登录即可访问
	rg.GET("/captcha", handler.SysCaptcha)
	rg.POST("/login", handler.SysUserLogin)

	// 以下路由统一要求登录，auth 分组内按资源再分组
	auth := rg.Group("", middleware.Auth())

	// 当前登录用户，登录用户都能读取自己的信息，只有修改才校验权限
	auth.GET("/auth/user", handler.SysAuthUser)
	auth.PUT("/auth/user", middleware.Permission("sys:auth:user"), handler.SysUserInfo)
	auth.PUT("/auth/password", middleware.Permission("sys:auth:password"), handler.SysAuthPassword)
	auth.DELETE("/logout", handler.SysUserLogout)

	// 用户管理
	user := auth.Group("/user")
	user.GET("/page", middleware.Permission("sys:user:list"), handler.SysUserPage)
	user.GET("/existence", middleware.Permission("sys:user:list"), handler.SysUserExistence)
	user.GET("", middleware.Permission("sys:user:list"), handler.SysUserList)
	user.POST("", middleware.Permission("sys:user:save"), handler.SysUserAdd)
	user.PUT("", middleware.Permission("sys:user:update"), handler.SysUserUpdate)
	user.PUT("/status", middleware.Permission("sys:user:update"), handler.SysUserUpdateStatus)
	user.PUT("/password", middleware.Permission("sys:user:update"), handler.SysUserUpdatePassword)
	user.DELETE("/batch", middleware.Permission("sys:user:remove"), handler.SysUserRemoveBatch)
	user.GET("/:id", middleware.Permission("sys:user:list"), handler.SysUserDetail)
	user.DELETE("/:id", middleware.Permission("sys:user:remove"), handler.SysUserRemove)

	// 角色管理
	role := auth.Group("/role")
	role.GET("/page", middleware.Permission("sys:role:list"), handler.SysRolePage)
	// 不分页的角色列表给用户编辑等表单做下拉选项，只校验登录
	role.GET("", handler.SysRoleList)
	role.POST("", middleware.Permission("sys:role:save"), handler.SysRoleAdd)
	role.PUT("", middleware.Permission("sys:role:update"), handler.SysRoleUpdate)
	role.DELETE("/batch", middleware.Permission("sys:role:remove"), handler.SysRoleRemoveBatch)
	role.DELETE("/:id", middleware.Permission("sys:role:remove"), handler.SysRoleRemove)

	// 角色菜单授权，回显属于查看角色，保存属于修改角色
	roleMenu := auth.Group("/role-menu")
	roleMenu.GET("/:roleId", middleware.Permission("sys:role:list"), handler.SysRoleMenuList)
	roleMenu.PUT("/:roleId", middleware.Permission("sys:role:update"), handler.SysRoleMenuUpdate)

	// 菜单管理
	menu := auth.Group("/menu")
	menu.GET("/page", middleware.Permission("sys:menu:list"), handler.SysMenuPage)
	menu.GET("", middleware.Permission("sys:menu:list"), handler.SysMenuList)
	menu.POST("", middleware.Permission("sys:menu:save"), handler.SysMenuAdd)
	menu.PUT("", middleware.Permission("sys:menu:update"), handler.SysMenuUpdate)
	menu.DELETE("/:id", middleware.Permission("sys:menu:remove"), handler.SysMenuRemove)

	// 字典与字典数据
	dictionary := auth.Group("/dictionary")
	// 不分页的字典列表给各处下拉选项使用，只校验登录
	dictionary.GET("", handler.SysDictionaryList)
	dictionary.GET("/page", middleware.Permission("sys:dict:list"), handler.SysDictionaryPage)
	dictionary.POST("", middleware.Permission("sys:dict:save"), handler.SysDictionaryAdd)
	dictionary.PUT("", middleware.Permission("sys:dict:update"), handler.SysDictionaryUpdate)
	dictionary.DELETE("/:id", middleware.Permission("sys:dict:remove"), handler.SysDictionaryRemove)

	dictionaryData := auth.Group("/dictionary-data")
	// 不分页的字典数据给字典标签、下拉选项使用，只校验登录
	dictionaryData.GET("", handler.SysDictionaryDataList)
	dictionaryData.GET("/page", middleware.Permission("sys:dict:list"), handler.SysDictionaryDataPage)
	dictionaryData.POST("", middleware.Permission("sys:dict:save"), handler.SysDictionaryDataAdd)
	dictionaryData.PUT("", middleware.Permission("sys:dict:update"), handler.SysDictionaryDataUpdate)
	dictionaryData.DELETE("/batch", middleware.Permission("sys:dict:remove"), handler.SysDictionaryDataRemoveBatch)
	dictionaryData.DELETE("/:id", middleware.Permission("sys:dict:remove"), handler.SysDictionaryDataRemove)

	// 配置中心
	config := auth.Group("/config")
	config.GET("", middleware.Permission("sys:config:list"), handler.SysConfigList)
	config.PUT("/group", middleware.Permission("sys:config:update"), handler.SysConfigSaveGroup)

	// 用户文件
	userFile := auth.Group("/user-file")
	userFile.GET("/page", middleware.Permission("sys:user-file:list"), handler.SysUserFilePage)
	userFile.GET("", middleware.Permission("sys:user-file:list"), handler.SysUserFileList)
	userFile.POST("", middleware.Permission("sys:user-file:save"), handler.SysUserFileAdd)
	userFile.PUT("", middleware.Permission("sys:user-file:update"), handler.SysUserFileUpdate)
	userFile.DELETE("/batch", middleware.Permission("sys:user-file:remove"), handler.SysUserFileRemoveBatch)
	userFile.DELETE("/:id", middleware.Permission("sys:user-file:remove"), handler.SysUserFileRemove)

	// 登录日志
	loginRecord := auth.Group("/login-record")
	loginRecord.GET("/page", middleware.Permission("sys:login-record:list"), handler.SysLoginRecordPage)
	loginRecord.GET("", middleware.Permission("sys:login-record:list"), handler.SysLoginRecordList)
	loginRecord.DELETE("/batch", middleware.Permission("sys:login-record:remove"), handler.SysLoginRecordRemoveBatch)
	loginRecord.DELETE("/:id", middleware.Permission("sys:login-record:remove"), handler.SysLoginRecordRemove)

	// 操作日志
	operationRecord := auth.Group("/operation-record")
	operationRecord.GET("/page", middleware.Permission("sys:operation-record:list"), handler.SysOperationRecordPage)
	operationRecord.GET("", middleware.Permission("sys:operation-record:list"), handler.SysOperationRecordList)
	operationRecord.DELETE("/batch", middleware.Permission("sys:operation-record:remove"), handler.SysOperationRecordRemoveBatch)
	operationRecord.DELETE("/:id", middleware.Permission("sys:operation-record:remove"), handler.SysOperationRecordRemove)

	// 我的消息, 只涉及当前登录用户自己的数据, 只校验登录
	message := auth.Group("/message")
	message.GET("/page", handler.SysMessagePage)
	message.GET("/unread", handler.SysMessageUnreadNum)
	message.PUT("/status", handler.SysMessageUpdateStatus)
	message.DELETE("/batch", handler.SysMessageRemoveBatch)
}

// RegisterFile 注册文件模块的路由，前端的文件接口没有 /system 前缀
func RegisterFile(rg *gin.RouterGroup) {
	auth := rg.Group("", middleware.Auth())
	// 上传是换头像、富文本插图等页面内的基础能力，只校验登录
	auth.POST("/upload", handler.SysFileUpload)
	auth.POST("/upload/base64", handler.SysFileUploadBase64)
	auth.GET("/page", middleware.Permission("sys:file:list"), handler.SysFilePage)
	auth.GET("/list", middleware.Permission("sys:file:list"), handler.SysFileList)
	auth.DELETE("/remove/batch", middleware.Permission("sys:file:remove"), handler.SysFileRemoveBatch)
	auth.DELETE("/remove/:id", middleware.Permission("sys:file:remove"), handler.SysFileRemove)

	// 文件访问与下载用作头像、图片预览和附件，只校验登录
	auth.GET("/raw/*path", handler.SysFilePath)
	auth.GET("/thumbnail/*path", handler.SysFilePath)
	auth.GET("/download/*path", func(c *gin.Context) {
		c.Set("download", true)
		handler.SysFilePath(c)
	})
}
