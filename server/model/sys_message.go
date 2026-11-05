package model

import "admin/server/internal/common"

// 消息类型
const (
	// MessageTypeNotice 是系统通知
	MessageTypeNotice = 1
	// MessageTypeTodo 是待办事项
	MessageTypeTodo = 2
)

// 消息状态
const (
	// MessageStatusUnread 是未读 / 未完成
	MessageStatusUnread = 0
	// MessageStatusRead 是已读 / 已完成
	MessageStatusRead = 1
)

// SysMessage 是我的消息，对应 sys_message 表
type SysMessage struct {
	// id
	ID int64 `gorm:"column:id;primaryKey;autoIncrement;comment:id" json:"id"`
	// 接收人id
	UserID int64 `gorm:"column:user_id;not null;index;comment:接收人id" json:"userId"`
	// 消息类型, 1通知, 2待办
	Type int `gorm:"column:type;not null;default:1;comment:消息类型, 1通知, 2待办" json:"type"`
	// 标题
	Title string `gorm:"column:title;size:128;not null;comment:标题" json:"title"`
	// 内容
	Content string `gorm:"column:content;size:512;comment:内容" json:"content"`
	// 状态, 0未读/未完成, 1已读/已完成
	Status int `gorm:"column:status;not null;default:0;comment:状态" json:"status"`
	// 是否删除, 0否, 1是
	Deleted int `gorm:"column:deleted;not null;default:0;comment:是否删除" json:"deleted"`
	// 创建时间
	CreateTime Time `gorm:"column:create_time;comment:创建时间" json:"createTime"`
	// 更新时间
	UpdateTime Time `gorm:"column:update_time;comment:更新时间" json:"updateTime"`
}

// TableName 指定表名
func (SysMessage) TableName() string {
	return "sys_message"
}

// MessageParam 是我的消息的查询参数
type MessageParam struct {
	common.PageParam
	// 消息类型, 1通知, 2待办
	Type *int `form:"type"`
	// 标题
	Title string `form:"title"`
	// 状态, 0未读/未完成, 1已读/已完成
	Status *int `form:"status"`
}

// MessageStatusForm 是消息状态修改的提交数据
type MessageStatusForm struct {
	// 消息id
	IDs []int64 `json:"ids"`
	// 状态, 0未读/未完成, 1已读/已完成
	Status *int `json:"status"`
}

// MessageUnreadNum 是我的消息未读数量，通知与待办分别统计
type MessageUnreadNum struct {
	// 通知未读数量
	Notice int64 `json:"notice"`
	// 待办未完成数量
	Todo int64 `json:"todo"`
}
