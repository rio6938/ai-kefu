package im

import (
	"encoding/base64"
	"testing"
)

// TestDecodePayload 校验回调里的 base64 payload 能解析成业务协议
func TestDecodePayload(t *testing.T) {
	raw := `{"version":1,"type":"text","role":"visitor","content":"你好","sessionId":88}`
	notify := &MessageNotify{
		ChannelID:   "session_88",
		ClientMsgNo: "visitor_1_1",
		Payload:     base64.StdEncoding.EncodeToString([]byte(raw)),
	}

	payload, err := notify.DecodePayload()
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.Content != "你好" || payload.Role != PayloadRoleVisitor {
		t.Fatalf("unexpected payload %+v", payload)
	}
	if got := notify.SessionID(); got != 88 {
		t.Fatalf("unexpected session id %d", got)
	}
}

// TestDecodePayloadPlainJSON 明文 JSON 也兼容，便于本地联调
func TestDecodePayloadPlainJSON(t *testing.T) {
	notify := &MessageNotify{
		ChannelID: "session_12",
		Payload:   `{"version":1,"type":"text","role":"agent","content":"hi"}`,
	}

	payload, err := notify.DecodePayload()
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.Role != PayloadRoleAgent || payload.Content != "hi" {
		t.Fatalf("unexpected payload %+v", payload)
	}
}

// TestSessionIDFromInvalidChannel 非法频道标识解析为 0，由调用方跳过处理
func TestSessionIDFromInvalidChannel(t *testing.T) {
	cases := map[string]int64{
		"session_0":    0,
		"session_abc":  0,
		"session_1024": 1024,
		"group_1024":   0,
		"":             0,
	}
	for channelID, want := range cases {
		notify := &MessageNotify{ChannelID: channelID}
		if got := notify.SessionID(); got != want {
			t.Fatalf("channel %q expect %d, got %d", channelID, want, got)
		}
	}
	if got := ChannelIDOfSession(7); got != "session_7" {
		t.Fatalf("unexpected channel id %q", got)
	}
}

// TestDecodeOnlineNotify 校验在线状态回调兼容单条、数组与 events 外壳三种形态
func TestDecodeOnlineNotify(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"single", `{"uid":"agent_1","online":1}`, 1},
		{"array", `[{"uid":"agent_1","online":1},{"uid":"agent_2","online":0}]`, 2},
		{"events", `{"events":[{"uid":"agent_1","online":1}]}`, 1},
	}
	for _, item := range cases {
		list, err := DecodeOnlineNotify([]byte(item.body))
		if err != nil {
			t.Fatalf("%s: decode online notify: %v", item.name, err)
		}
		if len(list) != item.want {
			t.Fatalf("%s: expect %d events, got %d", item.name, item.want, len(list))
		}
	}

	list, err := DecodeOnlineNotify([]byte(`{"events":[{"uid":"agent_9","online":1}]}`))
	if err != nil {
		t.Fatalf("decode events: %v", err)
	}
	if list[0].UID != "agent_9" || list[0].Online != 1 {
		t.Fatalf("unexpected event %+v", list[0])
	}

	if _, err := DecodeOnlineNotify(nil); err == nil {
		t.Fatal("expect error when body is empty")
	}
	if _, err := DecodeOnlineNotify([]byte("not json")); err == nil {
		t.Fatal("expect error when body is not json")
	}
}
