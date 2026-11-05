package model

import "admin/server/internal/common"

// 快捷回复的适用范围
const (
	// KefuQuickReplyScopeVisitor 是访客快捷问题，访客点击直接发送
	KefuQuickReplyScopeVisitor = "visitor"
	// KefuQuickReplyScopeAgent 是客服快捷话术，客服点击插入输入框
	KefuQuickReplyScopeAgent = "agent"
)

// 快捷回复状态
const (
	// KefuQuickReplyStatusNormal 是启用
	KefuQuickReplyStatusNormal = 0
	// KefuQuickReplyStatusDisabled 是停用
	KefuQuickReplyStatusDisabled = 1
)

// KefuQuickReply 是快捷回复，对应 kefu_quick_reply 表；
// scope 区分访客快捷问题与客服快捷话术，复用同一张表与同一个后台页面
type KefuQuickReply struct {
	// 快捷回复id
	QuickReplyID int64 `gorm:"column:quick_reply_id;primaryKey;autoIncrement;comment:快捷回复id" json:"quickReplyId"`
	// 适用范围, visitor 访客快捷问题, agent 客服快捷话术
	Scope string `gorm:"column:scope;size:16;not null;index;comment:适用范围" json:"scope"`
	// 标题
	Title string `gorm:"column:title;size:128;not null;comment:标题" json:"title"`
	// 内容
	Content string `gorm:"column:content;type:text;not null;comment:内容" json:"content"`
	// 分组编码, 为空表示未分组
	GroupCode string `gorm:"column:group_code;size:64;comment:分组编码" json:"groupCode"`
	// 排序号
	SortNumber int `gorm:"column:sort_number;not null;default:0;comment:排序号" json:"sortNumber"`
	// 使用次数
	UseCount int64 `gorm:"column:use_count;not null;default:0;comment:使用次数" json:"useCount"`
	// 状态, 0启用, 1停用
	Status int `gorm:"column:status;not null;default:0;comment:状态, 0启用, 1停用" json:"status"`
	// 是否删除, 0否, 1是
	Deleted int `gorm:"column:deleted;not null;default:0;comment:是否删除" json:"deleted"`
	// 创建人id
	CreateUserID int64 `gorm:"column:create_user_id;not null;default:0;comment:创建人id" json:"createUserId"`
	// 创建时间
	CreateTime Time `gorm:"column:create_time;comment:创建时间" json:"createTime"`
	// 更新时间
	UpdateTime Time `gorm:"column:update_time;comment:更新时间" json:"updateTime"`
}

// TableName 指定表名
func (KefuQuickReply) TableName() string {
	return "kefu_quick_reply"
}

// KefuQuickReplyParam 是快捷回复的查询参数
type KefuQuickReplyParam struct {
	common.PageParam
	// 适用范围, visitor 访客快捷问题, agent 客服快捷话术
	Scope string `form:"scope"`
	// 状态, 0启用, 1停用
	Status *int `form:"status"`
	// 分组编码
	GroupCode string `form:"groupCode"`
	// 关键词, 匹配标题与内容
	Keyword string `form:"keyword"`
}

// KefuQuickReplyForm 是快捷回复的编辑提交数据
type KefuQuickReplyForm struct {
	// 快捷回复id
	QuickReplyID int64 `json:"quickReplyId"`
	// 适用范围, visitor 访客快捷问题, agent 客服快捷话术
	Scope string `json:"scope"`
	// 标题
	Title string `json:"title"`
	// 内容
	Content string `json:"content"`
	// 分组编码, 为空表示未分组
	GroupCode string `json:"groupCode"`
	// 排序号
	SortNumber int `json:"sortNumber"`
	// 状态, 0启用, 1停用
	Status int `json:"status"`
}

// KefuQuickReplyStatusForm 是快捷回复状态开关的提交数据
type KefuQuickReplyStatusForm struct {
	// 快捷回复id
	QuickReplyID int64 `json:"quickReplyId"`
	// 状态, 0启用, 1停用
	Status int `json:"status"`
}

// KefuQuickReplyItem 是快捷回复对外的精简结构，聊天窗只拿标题与内容
type KefuQuickReplyItem struct {
	// 快捷回复id
	QuickReplyID int64 `json:"quickReplyId"`
	// 标题
	Title string `json:"title"`
	// 内容
	Content string `json:"content"`
	// 分组编码, 为空表示未分组
	GroupCode string `json:"groupCode"`
}
