package middleware

import (
	"errors"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"admin/server/internal/common"
	"admin/server/pkg/log"
)

// errNoPermission 是登录用户没有接口权限时的提示
var errNoPermission = errors.New("无权限操作")

// AuthorityLoader 是按用户id加载权限标识的能力，由 system 模块注册；
// 返回权限标识集合、是否为超级管理员以及加载失败的错误
type AuthorityLoader func(userID int64) (codes map[string]struct{}, isAdmin bool, err error)

// authorityLoader 是全局的权限加载函数
var authorityLoader AuthorityLoader

// SetAuthorityLoader 注册权限加载函数
func SetAuthorityLoader(loader AuthorityLoader) {
	authorityLoader = loader
}

// Permission 校验登录用户是否拥有接口要求的权限标识，
// 超级管理员角色直接放行；没有权限时返回 403 并让操作日志记为失败
func Permission(authority string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := common.CurrentUserID(c)
		if !ok {
			common.Unauthorized(c, "请先登录")
			c.Abort()
			return
		}
		if authorityLoader == nil {
			common.ServerError(c, "校验权限失败")
			c.Abort()
			return
		}

		codes, isAdmin, err := authorityLoader(userID)
		if err != nil {
			log.Logger.Error("load user authority failed",
				zap.Int64("userId", userID), zap.Error(err))
			common.ServerError(c, "校验权限失败")
			c.Abort()
			return
		}
		if isAdmin {
			c.Next()
			return
		}
		if _, ok := codes[authority]; ok {
			c.Next()
			return
		}

		// 记录到 gin 的错误列表，让操作日志把被拒绝的写请求记为失败
		_ = c.Error(errNoPermission)
		common.Forbidden(c, errNoPermission.Error())
		c.Abort()
	}
}
