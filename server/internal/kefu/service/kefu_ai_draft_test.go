package service

import (
	"strings"
	"testing"

	"admin/server/model"
)

// TestIsDraftAction 校验草稿动作白名单
func TestIsDraftAction(t *testing.T) {
	for _, action := range []string{
		model.KefuAiDraftActionReply,
		model.KefuAiDraftActionPolish,
		model.KefuAiDraftActionSummary,
	} {
		if !isDraftAction(action) {
			t.Fatalf("expect action %q supported", action)
		}
	}
	if isDraftAction("unknown") {
		t.Fatal("unexpected action supported")
	}
}

// TestDraftInstruction 校验草稿动作说明: 回复带上触发消息, 润色带上客服草稿
func TestDraftInstruction(t *testing.T) {
	trigger := &model.KefuMessage{Content: "什么时候发货?"}
	got := draftInstruction(model.KefuAiDraftActionReply, trigger, "")
	if !strings.Contains(got, "什么时候发货?") {
		t.Fatalf("reply instruction should contain trigger content, got %q", got)
	}

	got = draftInstruction(model.KefuAiDraftActionReply, nil, "")
	if got != aiDraftReplyInstruction {
		t.Fatalf("unexpected reply instruction %q", got)
	}

	got = draftInstruction(model.KefuAiDraftActionPolish, trigger, "明天发货")
	if !strings.Contains(got, "明天发货") || !strings.Contains(got, aiDraftPolishInstruction) {
		t.Fatalf("unexpected polish instruction %q", got)
	}

	got = draftInstruction(model.KefuAiDraftActionSummary, trigger, "")
	if got != aiDraftSummaryInstruction {
		t.Fatalf("unexpected summary instruction %q", got)
	}
}
