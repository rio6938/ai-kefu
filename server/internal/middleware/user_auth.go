package middleware

import (
	"github.com/gin-gonic/gin"

	"admin/server/internal/common"
	"admin/server/pkg/jwt"
)

// UserAuth 校验前台用户 token，校验通过后把登录用户写入上下文；
// 前台 token 使用独立密钥，与后台 token 互不通用
func UserAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := bearerToken(c.GetHeader("Authorization"))
		if token == "" {
			common.Unauthorized(c, "请先登录")
			c.Abort()
			return
		}

		claims, err := jwt.ParseUser(token)
		if err != nil {
			common.Unauthorized(c, "登录状态已失效，请重新登录")
			c.Abort()
			return
		}

		common.SetCurrentUser(c, claims.UserID, claims.Username)
		c.Next()
	}
}
