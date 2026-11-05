package kefu

import (
	"testing"

	"admin/server/internal/kefu/im"
	"admin/server/pkg/config"
)

// TestInitWithoutIMURL 未配置 IM 地址时初始化失败，避免带着坏配置启动
func TestInitWithoutIMURL(t *testing.T) {
	err := Init(config.WuKongIM{}, config.Qdrant{URL: "http://127.0.0.1:6334"}, config.Cors{})
	if err == nil {
		t.Fatal("expect error when wukongim.api_url is empty")
	}
}

// TestInitWithUnreachableQdrant 向量库不可达时只记 Warn，不阻塞启动
func TestInitWithUnreachableQdrant(t *testing.T) {
	err := Init(
		config.WuKongIM{ApiURL: "http://127.0.0.1:5001", WsURL: "ws://127.0.0.1:5200", AiUID: "ai_service"},
		config.Qdrant{URL: "http://127.0.0.1:1", CollectionPrefix: "kefu", VectorSize: 8},
		config.Cors{AllowOrigins: []string{"https://chat.example.com"}},
	)
	if err != nil {
		t.Fatalf("init should not fail when qdrant is unreachable: %v", err)
	}
	if im.Default == nil {
		t.Fatal("im client should be initialized")
	}
	if len(corsConfig.AllowOrigins) != 1 {
		t.Fatalf("unexpected cors config %+v", corsConfig)
	}
}
