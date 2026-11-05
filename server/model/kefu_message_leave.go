package model

import "admin/server/internal/common"

// 留言的处理状态
const (
	// KefuMessageLeaveStatusPending 是待处理
	KefuMessageLeaveStatusPending = 0
	// KefuMessageLeaveStatusHandled 是已处理
	KefuMessageLeaveStatusHandled = 1
)

// 留言的联系方式类型
const (
	// KefuMessageLeaveContactPhone 是手机号
	KefuMessageLeaveContactPhone = "phone"
	// KefuMessageLeaveContactEmail 是邮箱
	KefuMessageLeaveContactEmail = "email"
)

// KefuMessageLeave 是客服离线留言，对应 kefu_message_leave 表；
// 访客在无人工客服在线时提交，坐席在会话记录页的「留言」页签处理
type KefuMessageLeave struct {
	// 留言id
	LeaveID int64 `gorm:"column:leave_id;primaryKey;autoIncrement;comment:留言id" json:"leaveId"`
	// 会话id
	SessionID int64 `gorm:"column:session_id;not null;index;comment:会话id" json:"sessionId"`
	// 访客id
	VisitorID int64 `gorm:"column:visitor_id;not null;index;comment:访客id" json:"visitorId"`
	// 渠道标识
	ChannelCode string `gorm:"column:channel_code;size:64;comment:渠道标识" json:"channelCode"`
	// 访客姓名
	Name string `gorm:"column:name;size:64;comment:访客姓名" json:"name"`
	// 联系方式类型, phone 手机号, email 邮箱
	ContactType string `gorm:"column:contact_type;size:16;not null;comment:联系方式类型" json:"contactType"`
	// 联系方式
	Contact string `gorm:"column:contact;size:128;not null;comment:联系方式" json:"contact"`
	// 留言内容
	Content string `gorm:"column:content;type:text;comment:留言内容" json:"content"`
	// 处理状态, 0待处理, 1已处理
	Status int `gorm:"column:status;not null;default:0;comment:处理状态, 0待处理, 1已处理" json:"status"`
	// 处理人id
	HandleUserID int64 `gorm:"column:handle_user_id;not null;default:0;comment:处理人id" json:"handleUserId"`
	// 处理时间
	HandleTime Time `gorm:"column:handle_time;comment:处理时间" json:"handleTime"`
	// 处理备注
	Remark string `gorm:"column:remark;size:500;comment:处理备注" json:"remark"`
	// 是否删除, 0否, 1是
	Deleted int `gorm:"column:deleted;not null;default:0;comment:是否删除" json:"deleted"`
	// 创建时间
	CreateTime Time `gorm:"column:create_time;comment:创建时间" json:"createTime"`
	// 更新时间
	UpdateTime Time `gorm:"column:update_time;comment:更新时间" json:"updateTime"`
	// 处理人账号, 查询时回填
	HandleUsername string `gorm:"-" json:"handleUsername,omitempty"`
}

// TableName 指定表名
func (KefuMessageLeave) TableName() string {
	return "kefu_message_leave"
}

// KefuMessageLeaveParam 是留言的查询参数
type KefuMessageLeaveParam struct {
	common.PageParam
	// 处理状态, 0待处理, 1已处理
	Status *int `form:"status"`
	// 渠道标识
	ChannelCode string `form:"channelCode"`
	// 联系方式类型
	ContactType string `form:"contactType"`
	// 关键词, 匹配姓名与联系方式
	Keyword string `form:"keyword"`
}

// KefuMessageLeaveForm 是访客提交留言的数据
type KefuMessageLeaveForm struct {
	// 访客姓名
	Name string `json:"name"`
	// 联系方式类型, phone 手机号, email 邮箱
	ContactType string `json:"contactType"`
	// 联系方式
	Contact string `json:"contact"`
	// 留言内容
	Content string `json:"content"`
}

// KefuMessageLeaveHandleForm 是坐席处理留言的提交数据
type KefuMessageLeaveHandleForm struct {
	// 留言id
	LeaveID int64 `json:"leaveId"`
	// 处理备注
	Remark string `json:"remark"`
	// 是否给访客补发一条消息, 会话已结束时忽略
	Reply bool `json:"reply"`
}
