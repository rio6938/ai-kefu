package common

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// 响应码约定：0 为成功，错误码沿用 HTTP 状态码的含义
// （前端在响应成功分支里判断 401/403，用于跳登录/提示无权限）
const (
	CodeSuccess      = 0
	CodeBadRequest   = 400
	CodeUnauthorized = 401
	CodeForbidden    = 403
	CodeNotFound     = 404
	CodeServerError  = 500
)

// Response 是统一响应结构，对应前端 ApiResult<T>
type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// PageResult 是分页响应结构，对应前端 PageResult<T>
type PageResult[T any] struct {
	List  []T   `json:"list"`
	Count int64 `json:"count"`
}

// Success 返回成功响应，data 为 nil 时不输出 data 字段
func Success(c *gin.Context, data any) {
	SuccessWithMessage(c, "操作成功", data)
}

// SuccessWithMessage 返回带自定义提示语的成功响应
func SuccessWithMessage(c *gin.Context, message string, data any) {
	write(c, CodeSuccess, message, data)
}

// Page 返回分页成功响应，list 为 nil 时输出空数组
func Page[T any](c *gin.Context, list []T, count int64) {
	if list == nil {
		list = []T{}
	}
	Success(c, PageResult[T]{List: list, Count: count})
}

// Error 返回错误响应
func Error(c *gin.Context, code int, message string) {
	write(c, code, message, nil)
}

// BadRequest 返回 400 错误响应
func BadRequest(c *gin.Context, message string) {
	write(c, CodeBadRequest, message, nil)
}

// Unauthorized 返回 401 错误响应，前端会跳转登录
func Unauthorized(c *gin.Context, message string) {
	write(c, CodeUnauthorized, message, nil)
}

// Forbidden 返回 403 错误响应
func Forbidden(c *gin.Context, message string) {
	write(c, CodeForbidden, message, nil)
}

// NotFound 返回 404 错误响应
func NotFound(c *gin.Context, message string) {
	write(c, CodeNotFound, message, nil)
}

// ServerError 返回 500 错误响应
func ServerError(c *gin.Context, message string) {
	write(c, CodeServerError, message, nil)
}

// write 统一输出响应：HTTP 状态固定 200，业务结果由 body 的 code 表达
// （前端拦截器是在响应成功分支里判断 code 的，返回 4xx/5xx 会被 axios 当作网络错误）
func write(c *gin.Context, code int, message string, data any) {
	c.JSON(http.StatusOK, Response{Code: code, Message: message, Data: data})
}
