package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"admin/server/internal/common"
)

// newPermissionContext 构造带登录用户（userID 为 0 时表示未登录）的测试上下文
func newPermissionContext(userID int64) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	writer := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(writer)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	if userID > 0 {
		common.SetCurrentUser(c, userID, "tester")
	}
	return c, writer
}

// decodeResponse 解析统一响应结构
func decodeResponse(t *testing.T, writer *httptest.ResponseRecorder) common.Response {
	t.Helper()
	var res common.Response
	if err := json.Unmarshal(writer.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return res
}

// TestPermissionWithoutLogin 未登录时返回 401
func TestPermissionWithoutLogin(t *testing.T) {
	c, writer := newPermissionContext(0)
	Permission("sys:user:list")(c)

	if res := decodeResponse(t, writer); res.Code != common.CodeUnauthorized {
		t.Fatalf("want code %d, got %d", common.CodeUnauthorized, res.Code)
	}
	if !c.IsAborted() {
		t.Fatal("request should be aborted")
	}
}

// TestPermissionWithoutLoader 未注册权限加载函数时返回 500
func TestPermissionWithoutLoader(t *testing.T) {
	SetAuthorityLoader(nil)
	c, writer := newPermissionContext(1)
	Permission("sys:user:list")(c)

	if res := decodeResponse(t, writer); res.Code != common.CodeServerError {
		t.Fatalf("want code %d, got %d", common.CodeServerError, res.Code)
	}
	if !c.IsAborted() {
		t.Fatal("request should be aborted")
	}
}

// TestPermissionLoaderError 加载权限失败时拒绝请求，不降级放行
func TestPermissionLoaderError(t *testing.T) {
	SetAuthorityLoader(func(int64) (map[string]struct{}, bool, error) {
		return nil, false, errors.New("db down")
	})
	c, writer := newPermissionContext(1)
	Permission("sys:user:list")(c)

	if res := decodeResponse(t, writer); res.Code != common.CodeServerError {
		t.Fatalf("want code %d, got %d", common.CodeServerError, res.Code)
	}
	if !c.IsAborted() {
		t.Fatal("request should be aborted")
	}
}

// TestPermissionAdminPass 超级管理员不需要权限标识即可放行
func TestPermissionAdminPass(t *testing.T) {
	SetAuthorityLoader(func(int64) (map[string]struct{}, bool, error) {
		return nil, true, nil
	})
	c, writer := newPermissionContext(1)
	Permission("sys:user:list")(c)

	if c.IsAborted() {
		t.Fatalf("admin should pass, got body %s", writer.Body.String())
	}
	if len(c.Errors) > 0 {
		t.Fatalf("admin should not record error, got %v", c.Errors)
	}
}

// TestPermissionWithAuthority 拥有权限标识时放行
func TestPermissionWithAuthority(t *testing.T) {
	SetAuthorityLoader(func(int64) (map[string]struct{}, bool, error) {
		return map[string]struct{}{"sys:user:list": {}}, false, nil
	})
	c, writer := newPermissionContext(1)
	Permission("sys:user:list")(c)

	if c.IsAborted() {
		t.Fatalf("should pass, got body %s", writer.Body.String())
	}
}

// TestPermissionDenied 没有权限标识时返回 403，并记录 gin 错误供操作日志使用
func TestPermissionDenied(t *testing.T) {
	SetAuthorityLoader(func(int64) (map[string]struct{}, bool, error) {
		return map[string]struct{}{"sys:user:save": {}}, false, nil
	})
	c, writer := newPermissionContext(1)
	Permission("sys:user:list")(c)

	res := decodeResponse(t, writer)
	if res.Code != common.CodeForbidden {
		t.Fatalf("want code %d, got %d", common.CodeForbidden, res.Code)
	}
	if res.Message != errNoPermission.Error() {
		t.Fatalf("want message %q, got %q", errNoPermission.Error(), res.Message)
	}
	if !c.IsAborted() {
		t.Fatal("request should be aborted")
	}
	if len(c.Errors) == 0 {
		t.Fatal("denied request should record gin error for operation log")
	}
}
