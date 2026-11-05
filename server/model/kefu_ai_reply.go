package model

import "admin/server/internal/common"

// AI 回复状态
const (
	// KefuAiReplyStatusStreaming 是流式输出中
	KefuAiReplyStatusStreaming = "streaming"
	// KefuAiReplyStatusFinished 是正常完成
	KefuAiReplyStatusFinished = "finished"
	// KefuAiReplyStatusInterrupted 是被人工中断
	KefuAiReplyStatusInterrupted = "interrupted"
	// KefuAiReplyStatusError 是超时或报错
	KefuAiReplyStatusError = "error"
)

// AI 回复渠道
const (
	// KefuAiReplyChannelWeb 是网页渠道
	KefuAiReplyChannelWeb = "web"
	// KefuAiReplyChannelMail 是邮箱渠道
	KefuAiReplyChannelMail = "mail"
)

// 邮件 AI 回复的发送状态
const (
	// KefuAiReplySendNone 是不需要发送, 网页固定该值
	KefuAiReplySendNone = "none"
	// KefuAiReplySendDraft 是草稿
	KefuAiReplySendDraft = "draft"
	// KefuAiReplySendScheduled 是待发送
	KefuAiReplySendScheduled = "scheduled"
	// KefuAiReplySendSent 是已发送
	KefuAiReplySendSent = "sent"
	// KefuAiReplySendCancelled 是已取消
	KefuAiReplySendCancelled = "cancelled"
	// KefuAiReplySendFailed 是发送失败
	KefuAiReplySendFailed = "failed"
)

// KefuAiReply 是一次完整的 AI 回复记录，对应 kefu_ai_reply 表；
// 基础消息 + N 个 stream.delta + stream.finish 对应一条记录
type KefuAiReply struct {
	// 回复id
	ReplyID int64 `gorm:"column:reply_id;primaryKey;autoIncrement;comment:回复id" json:"replyId"`
	// 会话id
	SessionID int64 `gorm:"column:session_id;not null;index;comment:会话id" json:"sessionId"`
	// 触发本次回复的客户消息id
	TriggerMessageID int64 `gorm:"column:trigger_message_id;not null;default:0;comment:触发本次回复的客户消息id" json:"triggerMessageId"`
	// IM 侧幂等消息编号, reply_{replyId}
	ClientMsgNo string `gorm:"column:client_msg_no;size:64;index;comment:IM侧幂等消息编号" json:"clientMsgNo"`
	// 使用的模型配置id
	ModelID int64 `gorm:"column:model_id;not null;default:0;comment:使用的模型配置id" json:"modelId"`
	// 回复状态, streaming/finished/interrupted/error
	Status string `gorm:"column:status;size:16;not null;comment:回复状态" json:"status"`
	// 中断来源, agent_interrupt 表示客服中断, 其它为系统原因
	InterruptBy string `gorm:"column:interrupt_by;size:32;comment:中断来源" json:"interruptBy"`
	// AI 回复的完整文本
	Content string `gorm:"column:content;type:text;comment:AI回复的完整文本" json:"content"`
	// 本次使用的完整提示词快照, 便于排查问题
	PromptSnapshot string `gorm:"column:prompt_snapshot;type:text;comment:本次使用的完整提示词快照" json:"promptSnapshot"`
	// RAG 命中片段, json 数组
	RagHits string `gorm:"column:rag_hits;type:text;comment:RAG命中片段" json:"ragHits"`
	// 提示词消耗的 token 数
	PromptTokens int `gorm:"column:prompt_tokens;not null;default:0;comment:提示词消耗的token数" json:"promptTokens"`
	// 回复消耗的 token 数
	CompletionTokens int `gorm:"column:completion_tokens;not null;default:0;comment:回复消耗的token数" json:"completionTokens"`
	// 首字耗时, 毫秒
	FirstTokenMs int `gorm:"column:first_token_ms;not null;default:0;comment:首字耗时, 毫秒" json:"firstTokenMs"`
	// 总耗时, 毫秒
	LatencyMs int `gorm:"column:latency_ms;not null;default:0;comment:总耗时, 毫秒" json:"latencyMs"`
	// 回复渠道, web/mail
	Channel string `gorm:"column:channel;size:16;not null;default:web;comment:回复渠道" json:"channel"`
	// 邮件线程id, 网页为 0
	MailThreadID int64 `gorm:"column:mail_thread_id;not null;default:0;index;comment:邮件线程id" json:"mailThreadId"`
	// AI 建议的邮件主题
	Subject string `gorm:"column:subject;size:512;comment:AI建议的邮件主题" json:"subject"`
	// 邮件发送状态, none/draft/scheduled/sent/cancelled/failed
	SendStatus string `gorm:"column:send_status;size:16;not null;default:none;comment:邮件发送状态" json:"sendStatus"`
	// 延时自动发送的时间
	ScheduledTime Time `gorm:"column:scheduled_time;comment:延时自动发送的时间" json:"scheduledTime"`
	// 是否删除, 0否, 1是
	Deleted int `gorm:"column:deleted;not null;default:0;comment:是否删除" json:"deleted"`
	// 创建时间
	CreateTime Time `gorm:"column:create_time;comment:创建时间" json:"createTime"`
	// 更新时间
	UpdateTime Time `gorm:"column:update_time;comment:更新时间" json:"updateTime"`
}

// TableName 指定表名
func (KefuAiReply) TableName() string {
	return "kefu_ai_reply"
}

// KefuAiReplyParam 是 AI 回复记录的查询参数
type KefuAiReplyParam struct {
	common.PageParam
	// 会话id
	SessionID *int64 `form:"sessionId"`
	// 模型配置id
	ModelID *int64 `form:"modelId"`
	// 回复状态
	Status string `form:"status"`
}

// KefuAiInterruptResult 是 AI 干预接口的返回
// （replyId 为被中断的回复id, 0 表示当前没有进行中的回复）
type KefuAiInterruptResult struct {
	// 被中断的 AI 回复id
	ReplyID int64 `json:"replyId"`
}

// 工作台右键 AI 生成的动作
const (
	// KefuAiDraftActionReply 是针对客户消息生成回复草稿
	KefuAiDraftActionReply = "reply"
	// KefuAiDraftActionPolish 是润色客服已输入的草稿
	KefuAiDraftActionPolish = "polish"
	// KefuAiDraftActionSummary 是总结本次会话
	KefuAiDraftActionSummary = "summary"
)

// KefuAiDraftForm 是工作台右键 AI 生成草稿的提交数据，
// 与 OpenAI 对话接口共用同一个路由，所以额外字段都挂在请求体里
type KefuAiDraftForm struct {
	// 触发的客户消息id, 总结会话时可以为 0
	MessageID int64 `json:"messageId"`
	// 生成动作, reply/polish/summary
	Action string `json:"action"`
	// 客服已输入的草稿, 润色时必填
	Prompt string `json:"prompt"`
}
