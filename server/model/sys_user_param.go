package model

import "admin/server/internal/common"

// SysUserParam 是系统用户的查询参数
type SysUserParam struct {
	common.PageParam
	// 账号
	Username string `form:"username"`
	// 昵称
	Nickname string `form:"nickname"`
	// 性别
	Sex string `form:"sex"`
	// 手机号
	Phone string `form:"phone"`
	// 状态, 0正常, 1冻结
	Status *int `form:"status"`
}

// SysUserForm 是系统用户的编辑提交数据
type SysUserForm struct {
	// 用户id
	UserID int64 `json:"userId"`
	// 账号
	Username string `json:"username"`
	// 密码，新增时必填
	Password string `json:"password"`
	// 昵称
	Nickname string `json:"nickname"`
	// 头像
	Avatar string `json:"avatar"`
	// 性别
	Sex string `json:"sex"`
	// 手机号
	Phone string `json:"phone"`
	// 邮箱
	Email string `json:"email"`
	// 出生日期
	Birthday string `json:"birthday"`
	// 个人简介
	Introduction string `json:"introduction"`
	// 状态, 0正常, 1冻结
	Status int `json:"status"`
	// 角色列表
	Roles []Role `json:"roles"`
}

// SysUserStatusForm 是修改用户状态的提交数据
type SysUserStatusForm struct {
	// 用户id
	UserID int64 `json:"userId"`
	// 状态, 0正常, 1冻结
	Status int `json:"status"`
}

// SysUserPasswordForm 是重置用户密码的提交数据
type SysUserPasswordForm struct {
	// 用户id
	UserID int64 `json:"userId"`
	// 新密码
	Password string `json:"password"`
}

// SysUserInfoForm 是修改个人信息提交的数据
type SysUserInfoForm struct {
	// 昵称
	Nickname string `json:"nickname"`
	// 性别
	Sex string `json:"sex"`
	// 手机号
	Phone string `json:"phone"`
	// 邮箱
	Email string `json:"email"`
	// 个人简介
	Introduction string `json:"introduction"`
}

// SysUserOwnPasswordForm 是修改自己密码提交的数据
type SysUserOwnPasswordForm struct {
	// 原密码
	OldPassword string `json:"oldPassword"`
	// 新密码
	Password string `json:"password"`
}
