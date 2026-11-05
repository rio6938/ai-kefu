package kefu

import (
	"testing"

	"github.com/gin-gonic/gin"
)

// TestRegisterRoutes 校验四组路由能正常装配，路径参数与静态路径不冲突
func TestRegisterRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	Register(engine.Group("/kefu"))
	RegisterWebhook(engine.Group("/kefu/im/webhook"))
	RegisterAdmin(engine.Group("/admin/kefu"))
	RegisterOpenAI(engine.Group("/v1"))

	routes := engine.Routes()
	if len(routes) == 0 {
		t.Fatal("expect routes registered")
	}

	// 抽查几个关键路径，避免注册顺序调整后把路由覆盖掉
	exists := make(map[string]bool, len(routes))
	for _, route := range routes {
		exists[route.Method+" "+route.Path] = true
	}
	for _, want := range []string{
		"POST /kefu/bootstrap",
		"GET /kefu/channel/:code/config",
		"GET /kefu/session/:id/messages",
		"POST /kefu/im/webhook/message",
		"POST /kefu/im/webhook/online",
		"GET /admin/kefu/channel/page",
		"GET /admin/kefu/channel/list",
		"GET /admin/kefu/session/page",
		"GET /admin/kefu/session/:id",
		"POST /admin/kefu/session/:id/message",
		"PUT /admin/kefu/session/assign",
		"PUT /admin/kefu/session/transfer",
		"PUT /admin/kefu/session/close",
		"GET /admin/kefu/workbench/summary",
		"PUT /admin/kefu/session/ai-mode",
		"POST /admin/kefu/session/:id/ai-interrupt",
		"POST /admin/kefu/session/:id/chat/completions",
		"GET /admin/kefu/session/:id/ai-prompt",
		"PUT /admin/kefu/session/ai-prompt",
		"GET /admin/kefu/prompt/page",
		"GET /admin/kefu/prompt/list",
		"GET /admin/kefu/knowledge/page",
		"POST /admin/kefu/knowledge",
		"PUT /admin/kefu/knowledge",
		"POST /admin/kefu/knowledge/search",
		"POST /admin/kefu/knowledge/:id/reindex",
		"DELETE /admin/kefu/knowledge/:id",
		"GET /admin/kefu/dialogue/page",
		"GET /admin/kefu/dialogue/session-draft",
		"GET /admin/kefu/dialogue/:id",
		"POST /admin/kefu/dialogue",
		"PUT /admin/kefu/dialogue/status",
		"GET /admin/kefu/knowledge-category/page",
		"GET /admin/kefu/knowledge-category/list",
		"POST /admin/kefu/knowledge-category",
		"PUT /admin/kefu/knowledge-category",
		"PUT /admin/kefu/knowledge-category/status",
		"DELETE /admin/kefu/knowledge-category/:id",
		"GET /admin/kefu/route/page",
		"GET /admin/kefu/route/list",
		"GET /admin/kefu/route/:id",
		"POST /admin/kefu/route",
		"PUT /admin/kefu/route",
		"PUT /admin/kefu/route/status",
		"DELETE /admin/kefu/route/:id",
		"GET /admin/kefu/agent/page",
		"GET /admin/kefu/agent/list",
		"GET /admin/kefu/model/page",
		"GET /admin/kefu/model/list",
		"POST /admin/kefu/model/:id/test",
		"PUT /admin/kefu/agent/status",
		"DELETE /admin/kefu/agent/:id",
		"GET /admin/kefu/api-key/page",
		"GET /kefu/quick-reply",
		"POST /kefu/file/upload",
		"GET /kefu/file/raw/*path",
		"GET /admin/kefu/quick-reply/page",
		"GET /admin/kefu/quick-reply/list",
		"POST /admin/kefu/quick-reply",
		"PUT /admin/kefu/quick-reply",
		"PUT /admin/kefu/quick-reply/status",
		"POST /admin/kefu/quick-reply/:id/use",
		"DELETE /admin/kefu/quick-reply/:id",
		"POST /v1/chat/completions",
		"GET /v1/models",
	} {
		if !exists[want] {
			t.Fatalf("route %q not registered", want)
		}
	}
}
