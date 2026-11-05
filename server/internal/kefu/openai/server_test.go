package openai

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestWriteError 校验错误响应符合 OpenAI 规范: HTTP 状态码 + {error:{message,type,code}}
func TestWriteError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	WriteError(c, NewAPIError(http.StatusUnauthorized,
		ErrorTypeAuthentication, CodeInvalidAPIKey, "缺少 API 密钥"))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expect 401, got %d", rec.Code)
	}
	var body ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if body.Error.Code != CodeInvalidAPIKey || body.Error.Type != ErrorTypeAuthentication {
		t.Fatalf("unexpected error body %+v", body.Error)
	}
	if body.Error.Message != "缺少 API 密钥" {
		t.Fatalf("unexpected message %q", body.Error.Message)
	}
}

// TestAsAPIError 校验未知错误按服务端错误处理，已包装的 APIError 原样返回
func TestAsAPIError(t *testing.T) {
	origin := NewAPIError(http.StatusBadRequest, ErrorTypeInvalidRequest, CodeInvalidBody, "提交数据不正确")
	if got := AsAPIError(origin); got != origin {
		t.Fatalf("expect origin api error, got %+v", got)
	}

	got := AsAPIError(errors.New("数据库连接失败"))
	if got.Status != http.StatusInternalServerError || got.Type != ErrorTypeServer {
		t.Fatalf("unexpected api error %+v", got)
	}
	if got.Message != "数据库连接失败" {
		t.Fatalf("unexpected message %q", got.Message)
	}
}

// TestNewCompletionID 校验响应id带 chatcmpl- 前缀且不重复
func TestNewCompletionID(t *testing.T) {
	first := NewCompletionID()
	if len(first) <= len("chatcmpl-") || first[:len("chatcmpl-")] != "chatcmpl-" {
		t.Fatalf("unexpected completion id %q", first)
	}
	if first == NewCompletionID() {
		t.Fatal("completion id should be random")
	}
}
