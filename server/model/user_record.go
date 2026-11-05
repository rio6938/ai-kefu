package model

import "admin/server/internal/common"

// UserLoginRecord 是前台用户的登录日志，对应 user_login_record 表
type UserLoginRecord struct {
	// id
	ID int64 `gorm:"column:id;primaryKey;autoIncrement;comment:id" json:"id"`
	// 用户id
	UserID int64 `gorm:"column:user_id;comment:用户id" json:"userId"`
	// 账号
	Username string `gorm:"column:username;size:64;comment:账号" json:"username"`
	// 昵称
	Nickname string `gorm:"column:nickname;size:64;comment:昵称" json:"nickname"`
	// IP地址
	IP string `gorm:"column:ip;size:64;comment:IP地址" json:"ip"`
	// 设备型号
	Device string `gorm:"column:device;size:64;comment:设备型号" json:"device"`
	// 操作系统
	OS string `gorm:"column:os;size:64;comment:操作系统" json:"os"`
	// 浏览器
	Browser string `gorm:"column:browser;size:64;comment:浏览器" json:"browser"`
	// 操作类型, 0登录成功, 1登录失败, 2退出登录, 3刷新TOKEN, 4注册账号
	LoginType int `gorm:"column:login_type;not null;default:0;comment:操作类型" json:"loginType"`
	// 备注
	Comments string `gorm:"column:comments;size:255;comment:备注" json:"comments"`
	// 创建时间
	CreateTime Time `gorm:"column:create_time;comment:创建时间" json:"createTime"`
	// 更新时间
	UpdateTime Time `gorm:"column:update_time;comment:更新时间" json:"updateTime"`
}

// TableName 指定表名
func (UserLoginRecord) TableName() string {
	return "user_login_record"
}

// UserLoginRecordParam 是前台用户登录日志的查询参数
type UserLoginRecordParam struct {
	common.PageParam
	// 账号
	Username string `form:"username"`
	// 昵称
	Nickname string `form:"nickname"`
	// IP地址
	IP string `form:"ip"`
	// 操作类型
	LoginType *int `form:"loginType"`
	// 开始时间
	CreateTimeStart string `form:"createTimeStart"`
	// 结束时间
	CreateTimeEnd string `form:"createTimeEnd"`
}

// UserOperationRecord 是前台用户的操作日志，对应 user_operation_record 表
type UserOperationRecord struct {
	// id
	ID int64 `gorm:"column:id;primaryKey;autoIncrement;comment:id" json:"id"`
	// 用户id
	UserID int64 `gorm:"column:user_id;comment:用户id" json:"userId"`
	// 账号
	Username string `gorm:"column:username;size:64;comment:账号" json:"username"`
	// 昵称
	Nickname string `gorm:"column:nickname;size:64;comment:昵称" json:"nickname"`
	// 操作模块
	Module string `gorm:"column:module;size:64;comment:操作模块" json:"module"`
	// 操作功能
	Description string `gorm:"column:description;size:128;comment:操作功能" json:"description"`
	// 请求地址
	URL string `gorm:"column:url;size:255;comment:请求地址" json:"url"`
	// 请求方式
	RequestMethod string `gorm:"column:request_method;size:16;comment:请求方式" json:"requestMethod"`
	// 执行方法
	Method string `gorm:"column:method;size:255;comment:执行方法" json:"method"`
	// 请求参数
	Params string `gorm:"column:params;type:text;comment:请求参数" json:"params"`
	// 返回结果
	Result string `gorm:"column:result;type:text;comment:返回结果" json:"result"`
	// 异常信息
	Error string `gorm:"column:error;type:text;comment:异常信息" json:"error"`
	// 状态, 0正常, 1异常
	Status int `gorm:"column:status;not null;default:0;comment:状态" json:"status"`
	// 耗时, 单位毫秒
	SpendTime int64 `gorm:"column:spend_time;comment:耗时" json:"spendTime"`
	// IP地址
	IP string `gorm:"column:ip;size:64;comment:IP地址" json:"ip"`
	// 创建时间
	CreateTime Time `gorm:"column:create_time;comment:创建时间" json:"createTime"`
	// 更新时间
	UpdateTime Time `gorm:"column:update_time;comment:更新时间" json:"updateTime"`
}

// TableName 指定表名
func (UserOperationRecord) TableName() string {
	return "user_operation_record"
}

// UserOperationRecordParam 是前台用户操作日志的查询参数
type UserOperationRecordParam struct {
	common.PageParam
	// 账号
	Username string `form:"username"`
	// 昵称
	Nickname string `form:"nickname"`
	// 操作模块
	Module string `form:"module"`
	// 操作功能
	Description string `form:"description"`
	// 状态
	Status *int `form:"status"`
	// 开始时间
	CreateTimeStart string `form:"createTimeStart"`
	// 结束时间
	CreateTimeEnd string `form:"createTimeEnd"`
}
