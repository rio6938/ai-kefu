package model

import "admin/server/internal/common"

// 消息发送者角色
const (
	// KefuMessageFromVisitor 是访客
	KefuMessageFromVisitor = "visitor"
	// KefuMessageFromAgent 是客服
	KefuMessageFromAgent = "agent"
	// KefuMessageFromAi 是 AI 服务号
	KefuMessageFromAi = "ai"
	// KefuMessageFromSystem 是系统消息
	KefuMessageFromSystem = "system"
)

// 消息内容类型
const (
	// KefuMessageTypeText 是文本
	KefuMessageTypeText = "text"
	// KefuMessageTypeImage 是图片
	KefuMessageTypeImage = "image"
	// KefuMessageTypeFile 是文件
	KefuMessageTypeFile = "file"
	// KefuMessageTypeSystem 是系统消息
	KefuMessageTypeSystem = "system"
	// KefuMessageTypeAiAnswer 是 AI 回答, 流式消息的基础消息用它
	KefuMessageTypeAiAnswer = "ai_answer"
)

// KefuMessage 是会话消息，对应 kefu_message 表；
// client_msg_no 是 IM 侧的幂等键，重复投递只落一条
type KefuMessage struct {
	// 消息id
	MessageID int64 `gorm:"column:message_id;primaryKey;autoIncrement;comment:消息id" json:"messageId"`
	// 会话id
	SessionID int64 `gorm:"column:session_id;not null;index;comment:会话id" json:"sessionId"`
	// IM 侧幂等消息编号
	ClientMsgNo string `gorm:"column:client_msg_no;size:64;not null;uniqueIndex;comment:IM侧幂等消息编号" json:"clientMsgNo"`
	// IM 消息id
	IMMessageID string `gorm:"column:im_message_id;size:64;comment:IM消息id" json:"imMessageId"`
	// IM 消息序号
	IMMessageSeq int64 `gorm:"column:im_message_seq;not null;default:0;comment:IM消息序号" json:"imMessageSeq"`
	// 发送者角色, visitor/agent/ai/system
	FromRole string `gorm:"column:from_role;size:16;not null;comment:发送者角色" json:"fromRole"`
	// 发送者标识, 如 visitor_1 / agent_2 / ai_service
	FromID string `gorm:"column:from_id;size:64;comment:发送者标识" json:"fromId"`
	// 内容类型, text/image/file/system/ai_answer
	ContentType string `gorm:"column:content_type;size:16;not null;comment:内容类型" json:"contentType"`
	// 文本内容
	Content string `gorm:"column:content;type:text;comment:文本内容" json:"content"`
	// 消息原始 payload, json 文本
	Payload string `gorm:"column:payload;type:text;comment:消息原始payload" json:"payload"`
	// 是否为 AI 草稿, 1 表示客服采纳了右键 AI 生成的草稿
	AiGenerated int `gorm:"column:ai_generated;not null;default:0;comment:是否为AI生成" json:"aiGenerated"`
	// 引用回复的原消息id, 0 表示不是引用
	QuoteMessageID int64 `gorm:"column:quote_message_id;not null;default:0;comment:引用回复的原消息id" json:"quoteMessageId"`
	// 引用的原消息摘要, 冗余存一份, 原消息删除后仍可展示
	QuoteContent string `gorm:"column:quote_content;size:500;comment:引用的原消息摘要" json:"quoteContent"`
	// 引用的原消息发送者角色
	QuoteRole string `gorm:"column:quote_role;size:16;comment:引用的原消息发送者角色" json:"quoteRole"`
	// 关联的 AI 回复id, 0 表示不是 AI 回复
	AiReplyID int64 `gorm:"column:ai_reply_id;not null;default:0;comment:关联的AI回复id" json:"aiReplyId"`
	// 消息状态, 0正常, 1已撤回
	Status int `gorm:"column:status;not null;default:0;comment:消息状态, 0正常, 1已撤回" json:"status"`
	// 是否删除, 0否, 1是
	Deleted int `gorm:"column:deleted;not null;default:0;comment:是否删除" json:"deleted"`
	// 创建时间
	CreateTime Time `gorm:"column:create_time;comment:创建时间" json:"createTime"`
	// 更新时间
	UpdateTime Time `gorm:"column:update_time;comment:更新时间" json:"updateTime"`
}

// TableName 指定表名
func (KefuMessage) TableName() string {
	return "kefu_message"
}

// KefuMessageParam 是消息的查询参数
type KefuMessageParam struct {
	common.PageParam
	// 会话id
	SessionID *int64 `form:"sessionId"`
	// 发送者角色
	FromRole string `form:"fromRole"`
	// 内容类型
	ContentType string `form:"contentType"`
	// IM 侧幂等消息编号
	ClientMsgNo string `form:"clientMsgNo"`
	// 起始消息id, 拉历史消息时用作游标
	StartMessageID *int64 `form:"startMessageId"`
}

// KefuMessageForm 是发送消息的提交数据
type KefuMessageForm struct {
	// 会话id
	SessionID int64 `json:"sessionId"`
	// 内容类型, text/image/file
	ContentType string `json:"contentType"`
	// 文本内容
	Content string `json:"content"`
	// 消息附加信息, json 文本
	Payload string `json:"payload"`
	// 是否为 AI 草稿, 1 表示客服采纳了右键 AI 生成的草稿
	AiGenerated int `json:"aiGenerated"`
	// 引用回复的原消息id, 0 表示不引用
	QuoteMessageID int64 `json:"quoteMessageId"`
}
