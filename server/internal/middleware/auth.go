package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"admin/server/internal/common"
	"admin/server/pkg/jwt"
)

// Auth 校验请求头里的 token，校验通过后把登录用户写入上下文
func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := bearerToken(c.GetHeader("Authorization"))
		if token == "" {
			common.Unauthorized(c, "请先登录")
			c.Abort()
			return
		}

		claims, err := jwt.Parse(token)
		if err != nil {
			common.Unauthorized(c, "登录状态已失效，请重新登录")
			c.Abort()
			return
		}

		common.SetCurrentUser(c, claims.UserID, claims.Username)
		c.Next()
	}
}

// bearerToken 从 Authorization 请求头中取出 token
func bearerToken(header string) string {
	if header == "" {
		return ""
	}
	if len(header) > 7 && strings.EqualFold(header[:7], "Bearer ") {
		return strings.TrimSpace(header[7:])
	}
	return strings.TrimSpace(header)
}
