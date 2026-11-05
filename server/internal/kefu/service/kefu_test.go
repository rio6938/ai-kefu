package service

import (
	"testing"

	"admin/server/internal/kefu/im"
	"admin/server/model"
)

// TestResolveFromRole 校验消息角色的解析: 优先用 payload 里的角色, 没有时按 uid 前缀推断
func TestResolveFromRole(t *testing.T) {
	cases := []struct {
		name    string
		payload *im.Payload
		fromUID string
		want    string
	}{
		{"payload visitor", &im.Payload{Role: im.PayloadRoleVisitor}, "visitor_1", model.KefuMessageFromVisitor},
		{"payload agent", &im.Payload{Role: im.PayloadRoleAgent}, "agent_1", model.KefuMessageFromAgent},
		{"uid visitor", &im.Payload{}, "visitor_9", model.KefuMessageFromVisitor},
		{"uid agent", &im.Payload{}, "agent_9", model.KefuMessageFromAgent},
		{"unknown uid", &im.Payload{}, "other", model.KefuMessageFromSystem},
	}
	for _, item := range cases {
		if got := resolveFromRole(item.payload, item.fromUID); got != item.want {
			t.Fatalf("%s: expect %q, got %q", item.name, item.want, got)
		}
	}
}

// TestMessageContentType 校验未知的内容类型按文本处理
func TestMessageContentType(t *testing.T) {
	if got := messageContentType(model.KefuMessageTypeImage); got != model.KefuMessageTypeImage {
		t.Fatalf("expect image, got %q", got)
	}
	if got := messageContentType("unknown"); got != model.KefuMessageTypeText {
		t.Fatalf("expect text, got %q", got)
	}
}

// TestGenerateSessionNo 校验会话编号长度固定, 便于展示与检索
func TestGenerateSessionNo(t *testing.T) {
	first := generateSessionNo()
	if len(first) != 19 || first[0] != 'S' {
		t.Fatalf("unexpected session no %q", first)
	}
}

// TestAgentUID 校验 IM 用户标识格式
func TestAgentUID(t *testing.T) {
	if got := agentUID(12); got != "agent_12" {
		t.Fatalf("unexpected agent uid %q", got)
	}
	if got := visitorUID(12); got != "visitor_12" {
		t.Fatalf("unexpected visitor uid %q", got)
	}
}
