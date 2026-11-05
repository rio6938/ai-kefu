package middleware

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"admin/server/internal/common"
	"admin/server/model"
	"admin/server/pkg/log"
)

// OperationRecorder 是写入操作日志的能力，由 system 模块注册
type OperationRecorder func(record *model.SysOperationRecord) error

// UserOperationRecorder 是写入前台用户操作日志的能力，由 user 模块注册
type UserOperationRecorder func(record *model.UserOperationRecord) error

// operationRecorder 是全局的操作日志写入函数
var operationRecorder OperationRecorder

// userOperationRecorder 是全局的前台用户操作日志写入函数
var userOperationRecorder UserOperationRecorder

// SetOperationRecorder 注册操作日志写入函数
func SetOperationRecorder(recorder OperationRecorder) {
	operationRecorder = recorder
}

// SetUserOperationRecorder 注册前台用户操作日志写入函数
func SetUserOperationRecorder(recorder UserOperationRecorder) {
	userOperationRecorder = recorder
}

// OperationRecord 记录增删改请求的操作日志，忽略查询请求与日志查询自身；
// 前台用户接口（/user 前缀）写前台用户操作日志，其余接口写后台操作日志
func OperationRecord() gin.HandlerFunc {
	return func(c *gin.Context) {
		userSide := isUserRequest(c.Request.URL.Path)
		if !shouldRecord(c) || (userSide && userOperationRecorder == nil) ||
			(!userSide && operationRecorder == nil) {
			c.Next()
			return
		}

		start := time.Now()
		params := readParams(c)
		writer := &responseWriter{ResponseWriter: c.Writer, body: bytes.NewBuffer(nil)}
		c.Writer = writer

		c.Next()

		status := 0
		errorMessage := strings.TrimSpace(c.Errors.String())
		if writer.Status() >= http.StatusBadRequest || errorMessage != "" {
			status = 1
		}

		if userSide {
			writeUserOperationRecord(c, params, writer.body.String(), errorMessage, status, start)
			return
		}
		writeOperationRecord(c, params, writer.body.String(), errorMessage, status, start)
	}
}

// writeOperationRecord 写入后台操作日志
func writeOperationRecord(c *gin.Context, params, result, errorMessage string, status int, start time.Time) {
	userID, _ := common.CurrentUserID(c)
	record := &model.SysOperationRecord{
		UserID:        userID,
		Username:      common.CurrentUsername(c),
		Module:        moduleName(c.Request.URL.Path),
		Description:   c.Request.Method + " " + c.Request.URL.Path,
		URL:           "/api" + c.Request.URL.Path,
		RequestMethod: c.Request.Method,
		Params:        params,
		Result:        truncate(result, 4000),
		Error:         errorMessage,
		Status:        status,
		SpendTime:     time.Since(start).Milliseconds(),
		IP:            c.ClientIP(),
	}
	// 写日志失败不影响业务响应
	if err := operationRecorder(record); err != nil {
		log.Logger.Warn("write operation record failed", zap.Error(err))
	}
}

// writeUserOperationRecord 写入前台用户操作日志，无登录用户时跳过
func writeUserOperationRecord(c *gin.Context, params, result, errorMessage string, status int, start time.Time) {
	userID, ok := common.CurrentUserID(c)
	if !ok {
		return
	}
	record := &model.UserOperationRecord{
		UserID:        userID,
		Username:      common.CurrentUsername(c),
		Module:        moduleName(c.Request.URL.Path),
		Description:   c.Request.Method + " " + c.Request.URL.Path,
		URL:           "/api" + c.Request.URL.Path,
		RequestMethod: c.Request.Method,
		Params:        params,
		Result:        truncate(result, 4000),
		Error:         errorMessage,
		Status:        status,
		SpendTime:     time.Since(start).Milliseconds(),
		IP:            c.ClientIP(),
	}
	// 写日志失败不影响业务响应
	if err := userOperationRecorder(record); err != nil {
		log.Logger.Warn("write user operation record failed", zap.Error(err))
	}
}

// isUserRequest 判断前台用户接口，前台接口统一在 /user 前缀下
func isUserRequest(path string) bool {
	return path == "/user" || strings.HasPrefix(path, "/user/")
}

// shouldRecord 判断请求是否需要记录操作日志
func shouldRecord(c *gin.Context) bool {
	switch c.Request.Method {
	case http.MethodPost, http.MethodPut, http.MethodDelete:
	default:
		return false
	}
	path := c.Request.URL.Path
	// 登录、退出、日志查询本身不记录，避免噪音与递归
	for _, ignore := range []string{"/login", "/logout", "/register", "/captcha", "/operation-record"} {
		if path == ignore || strings.HasSuffix(path, ignore) {
			return false
		}
	}
	return true
}

// readParams 读取请求参数，读取后重新写回请求体
func readParams(c *gin.Context) string {
	// 配置中心的请求体可能包含密钥，不写进操作日志
	if isSensitivePath(c.Request.URL.Path) {
		return ""
	}
	if c.Request.URL.RawQuery != "" && c.Request.Body == nil {
		return c.Request.URL.RawQuery
	}
	if c.Request.Body == nil || c.Request.ContentLength == 0 {
		return c.Request.URL.RawQuery
	}
	if strings.HasPrefix(c.ContentType(), "multipart/form-data") {
		return "[multipart/form-data]"
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return ""
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

	if c.Request.URL.RawQuery != "" {
		return truncate(c.Request.URL.RawQuery+" "+string(body), 4000)
	}
	return truncate(string(body), 4000)
}

// isSensitivePath 判断请求体不能记入操作日志的接口
func isSensitivePath(path string) bool {
	return path == "/admin/config" || strings.HasPrefix(path, "/admin/config/")
}

// moduleName 按路径前缀取模块名称
func moduleName(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	if (parts[0] == "admin" || parts[0] == "user") && len(parts) > 1 {
		return parts[1]
	}
	return parts[0]
}

// truncate 截断过长的文本，并把非 utf8 的字节替换掉，避免写库失败
func truncate(value string, max int) string {
	value = strings.ToValidUTF8(value, "?")
	if len(value) <= max {
		return value
	}
	// 按字节截断后可能切断多字节字符，再修正一次
	return strings.ToValidUTF8(value[:max], "")
}

// responseWriter 缓存响应体，用于写入操作日志
type responseWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

// Write 同时写入缓存与真实响应
func (w *responseWriter) Write(data []byte) (int, error) {
	w.body.Write(data)
	return w.ResponseWriter.Write(data)
}

// WriteString 同时写入缓存与真实响应
func (w *responseWriter) WriteString(data string) (int, error) {
	w.body.WriteString(data)
	return w.ResponseWriter.WriteString(data)
}
