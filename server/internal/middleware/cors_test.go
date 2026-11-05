package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"admin/server/pkg/config"
)

// newCorsRouter 构造只挂跨域中间件、返回成功响应的测试路由
func newCorsRouter(cfg config.Cors) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(Cors(cfg))
	engine.GET("/kefu/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})
	engine.OPTIONS("/kefu/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})
	return engine
}

// TestCorsDisabledByDefault 未配置 allow_origins 时不输出任何跨域响应头
func TestCorsDisabledByDefault(t *testing.T) {
	engine := newCorsRouter(config.Cors{})

	req := httptest.NewRequest(http.MethodGet, "/kefu/ping", nil)
	req.Header.Set("Origin", "https://chat.example.com")
	writer := httptest.NewRecorder()
	engine.ServeHTTP(writer, req)

	if got := writer.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expect no allow-origin header, got %q", got)
	}
	if got := writer.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("expect no allow-credentials header, got %q", got)
	}
}

// TestCorsPreflight 允许的来源预检通过并返回跨域响应头
func TestCorsPreflight(t *testing.T) {
	engine := newCorsRouter(config.Cors{
		AllowOrigins:     []string{"https://chat.example.com"},
		AllowCredentials: true,
	})

	req := httptest.NewRequest(http.MethodOptions, "/kefu/ping", nil)
	req.Header.Set("Origin", "https://chat.example.com")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	req.Header.Set("Access-Control-Request-Headers", "Content-Type")
	writer := httptest.NewRecorder()
	engine.ServeHTTP(writer, req)

	if writer.Code != http.StatusNoContent {
		t.Fatalf("want status %d, got %d", http.StatusNoContent, writer.Code)
	}
	if got := writer.Header().Get("Access-Control-Allow-Origin"); got != "https://chat.example.com" {
		t.Fatalf("unexpected allow-origin header %q", got)
	}
	if got := writer.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("unexpected allow-credentials header %q", got)
	}
	if got := writer.Header().Get("Access-Control-Allow-Methods"); got != corsAllowMethods {
		t.Fatalf("unexpected allow-methods header %q", got)
	}
}

// TestCorsOriginNotAllowed 不在白名单里的来源不输出跨域头，预检直接拒绝
func TestCorsOriginNotAllowed(t *testing.T) {
	engine := newCorsRouter(config.Cors{AllowOrigins: []string{"https://chat.example.com"}})

	req := httptest.NewRequest(http.MethodOptions, "/kefu/ping", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	writer := httptest.NewRecorder()
	engine.ServeHTTP(writer, req)

	if writer.Code != http.StatusForbidden {
		t.Fatalf("want status %d, got %d", http.StatusForbidden, writer.Code)
	}
	if got := writer.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expect no allow-origin header, got %q", got)
	}
}
