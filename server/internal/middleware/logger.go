package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"admin/server/pkg/log"
)

// Logger 用 zap 记录访问日志
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		status := c.Writer.Status()
		fields := []zap.Field{
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.String("query", c.Request.URL.RawQuery),
			zap.String("ip", c.ClientIP()),
			zap.Int("status", status),
			zap.Duration("latency", time.Since(start)),
		}
		if errs := c.Errors.String(); errs != "" {
			fields = append(fields, zap.String("errors", errs))
		}

		switch {
		case status >= http.StatusInternalServerError:
			log.Access.Error("http", fields...)
		case status >= http.StatusBadRequest:
			log.Access.Warn("http", fields...)
		default:
			log.Access.Info("http", fields...)
		}
	}
}
