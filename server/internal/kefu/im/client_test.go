package im

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"admin/server/pkg/config"
)

// newTestClient 用测试服务启动一个 IM 客户端
func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	if err := Init(config.WuKongIM{ApiURL: server.URL, WsURL: "ws://127.0.0.1:5200", AiUID: "ai_service"}); err != nil {
		t.Fatalf("init client: %v", err)
	}
	return Default
}

// TestSendMessageSuccess 发送成功时返回消息id，payload 按 base64 传输
func TestSendMessageSuccess(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/message/send" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}

		var req MessageSendReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if req.ClientMsgNo != "reply_1" || req.FromUID != "ai_service" {
			t.Errorf("unexpected request %+v", req)
		}
		// payload 是 base64 字符串，解码后应为业务协议
		var payload map[string]any
		if err := json.Unmarshal(req.Payload, &payload); err != nil {
			t.Errorf("payload should be json bytes: %v", err)
		}

		_, _ = w.Write([]byte(`{"message_id":1024,"client_msg_no":"reply_1","message_seq":8}`))
	})

	resp, err := client.SendMessage(context.Background(), &MessageSendReq{
		ClientMsgNo: "reply_1",
		FromUID:     "ai_service",
		ChannelID:   "session_88",
		ChannelType: ChannelTypeGroup,
		Payload:     []byte(`{"version":1,"type":"ai_answer"}`),
	})
	if err != nil {
		t.Fatalf("send message: %v", err)
	}
	if resp.MessageID != 1024 || resp.MessageSeq != 8 {
		t.Fatalf("unexpected response %+v", resp)
	}
}

// TestSendMessageReasonFailed reason 非空时按失败处理，即使 HTTP 是 200
func TestSendMessageReasonFailed(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":200,"data":{"reason":"channel not exist"}}`))
	})

	_, err := client.SendMessage(context.Background(), &MessageSendReq{ClientMsgNo: "reply_1"})
	if err == nil {
		t.Fatal("expect error when reason is not empty")
	}
	var imErr *Error
	if !errors.As(err, &imErr) {
		t.Fatalf("expect *Error, got %T", err)
	}
	if imErr.Reason != "channel not exist" {
		t.Fatalf("unexpected reason %q", imErr.Reason)
	}
}

// TestAppendEventRetry 5xx 会重试，最终成功后返回结果
func TestAppendEventRetry(t *testing.T) {
	attempts := 0
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(`{"status":200,"data":{"message_id":9,"message_seq":3}}`))
	})

	resp, err := client.AppendEvent(context.Background(), &MessageEventReq{
		EventID:   "reply_1:delta:1",
		EventType: EventTypeStreamDelta,
	})
	if err != nil {
		t.Fatalf("append event: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("want 2 attempts, got %d", attempts)
	}
	if resp.MessageSeq != 3 {
		t.Fatalf("unexpected response %+v", resp)
	}
}

// TestOnlineStatus 校验 uid 数组请求与设备维度的数组返回
func TestOnlineStatus(t *testing.T) {
	var body string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		body = string(raw)
		_, _ = w.Write([]byte(`{"status":200,"data":[{"uid":"agent_1","device_flag":1,"online":1},{"uid":"agent_1","device_flag":2,"online":0},{"uid":"agent_2","device_flag":1,"online":0}]}`))
	})

	status, err := client.OnlineStatus(context.Background(), []string{"agent_1", "agent_2", "agent_3"})
	if err != nil {
		t.Fatalf("online status: %v", err)
	}
	if body != `["agent_1","agent_2","agent_3"]` {
		t.Fatalf("unexpected request body %s", body)
	}
	if !status["agent_1"] || status["agent_2"] || status["agent_3"] {
		t.Fatalf("unexpected online status %+v", status)
	}
}
