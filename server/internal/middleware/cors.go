package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"admin/server/pkg/config"
)

// 预检请求响应头里固定返回的方法与请求头
const (
	corsAllowMethods = "GET, POST, PUT, DELETE, OPTIONS"
	corsAllowHeaders = "Authorization, Content-Type, X-Kefu-Rag"
	corsMaxAge       = "86400"
)

// Cors 按配置输出跨域响应头，只挂在 /kefu 与 /v1 公开路由上，/admin 一律不开放跨域；
// 未配置 allow_origins 时不输出任何 Access-Control-* 头（同源反代场景无需跨域）
func Cors(cfg config.Cors) gin.HandlerFunc {
	origins := make(map[string]struct{}, len(cfg.AllowOrigins))
	allowAll := false
	for _, origin := range cfg.AllowOrigins {
		value := strings.TrimSpace(origin)
		if value == "" {
			continue
		}
		if value == "*" {
			allowAll = true
			continue
		}
		origins[strings.ToLower(value)] = struct{}{}
	}
	enabled := allowAll || len(origins) > 0

	return func(c *gin.Context) {
		if !enabled {
			c.Next()
			return
		}

		origin := c.GetHeader("Origin")
		// 同源请求不带 Origin，不需要（也不应该）输出跨域响应头
		if origin == "" {
			c.Next()
			return
		}
		_, matched := origins[strings.ToLower(origin)]
		if !allowAll && !matched {
			// 来源不允许时不输出跨域头，预检直接拒绝，避免浏览器误判为通过
			if isPreflight(c) {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
			c.Next()
			return
		}

		header := c.Writer.Header()
		// 允许携带 cookie 时不能返回 *，统一回显请求来源
		if allowAll && !cfg.AllowCredentials {
			header.Set("Access-Control-Allow-Origin", "*")
		} else {
			header.Set("Access-Control-Allow-Origin", origin)
			header.Add("Vary", "Origin")
		}
		if cfg.AllowCredentials {
			header.Set("Access-Control-Allow-Credentials", "true")
		}

		if isPreflight(c) {
			header.Set("Access-Control-Allow-Methods", corsAllowMethods)
			allowHeaders := strings.TrimSpace(c.GetHeader("Access-Control-Request-Headers"))
			if allowHeaders == "" {
				allowHeaders = corsAllowHeaders
			}
			header.Set("Access-Control-Allow-Headers", allowHeaders)
			header.Set("Access-Control-Max-Age", corsMaxAge)
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// isPreflight 判断是否为跨域预检请求
func isPreflight(c *gin.Context) bool {
	return c.Request.Method == http.MethodOptions &&
		c.GetHeader("Access-Control-Request-Method") != ""
}
