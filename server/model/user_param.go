package model

import "admin/server/internal/common"

// UserParam 是前台用户的查询参数
type UserParam struct {
	common.PageParam
	// 账号
	Username string `form:"username"`
	// 昵称
	Nickname string `form:"nickname"`
	// 手机号
	Phone string `form:"phone"`
	// 邮箱
	Email string `form:"email"`
	// 状态, 0正常, 1冻结
	Status *int `form:"status"`
}

// UserForm 是前台用户的编辑提交数据
type UserForm struct {
	// id
	ID int64 `json:"id"`
	// 账号
	Username string `json:"username"`
	// 密码，新增时必填
	Password string `json:"password"`
	// 昵称
	Nickname string `json:"nickname"`
	// 头像
	Avatar string `json:"avatar"`
	// 手机号
	Phone string `json:"phone"`
	// 邮箱
	Email string `json:"email"`
	// 状态, 0正常, 1冻结
	Status int `json:"status"`
}

// UserStatusForm 是修改前台用户状态的提交数据
type UserStatusForm struct {
	// id
	ID int64 `json:"id"`
	// 状态, 0正常, 1冻结
	Status int `json:"status"`
}

// UserPasswordForm 是重置前台用户密码的提交数据
type UserPasswordForm struct {
	// id
	ID int64 `json:"id"`
	// 新密码
	Password string `json:"password"`
}
