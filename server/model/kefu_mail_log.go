package model

import "admin/server/internal/common"

// 邮件收发方向
const (
	// KefuMailDirectionIn 是入站
	KefuMailDirectionIn = "in"
	// KefuMailDirectionOut 是出站
	KefuMailDirectionOut = "out"
)

// 邮件记录状态
const (
	// KefuMailStatusReceived 是入站已接收
	KefuMailStatusReceived = "received"
	// KefuMailStatusSkipped 是入站已跳过
	KefuMailStatusSkipped = "skipped"
	// KefuMailStatusFailed 是入站解析失败
	KefuMailStatusFailed = "failed"
	// KefuMailStatusSending 是出站发送中
	KefuMailStatusSending = "sending"
	// KefuMailStatusSent 是出站已发送
	KefuMailStatusSent = "sent"
)

// 邮件跳过原因
const (
	// KefuMailSkipAutoSubmitted 是自动回复头
	KefuMailSkipAutoSubmitted = "auto_submitted"
	// KefuMailSkipBulk 是群发邮件
	KefuMailSkipBulk = "bulk"
	// KefuMailSkipLoop 是自环邮件
	KefuMailSkipLoop = "loop"
	// KefuMailSkipDuplicate 是重复投递
	KefuMailSkipDuplicate = "duplicate"
	// KefuMailSkipBounce 是退信
	KefuMailSkipBounce = "bounce"
	// KefuMailSkipSenderMissing 是发件人为空
	KefuMailSkipSenderMissing = "sender_missing"
	// KefuMailSkipSpam 是垃圾邮件
	KefuMailSkipSpam = "spam"
	// KefuMailSkipOther 是其它原因
	KefuMailSkipOther = "other"
)

// KefuMailLog 是邮件收发记录，对应 kefu_mail_log 表；
// 每封实际收发的邮件一条，mail_key 保证幂等
type KefuMailLog struct {
	// 邮件记录id
	MailID int64 `gorm:"column:mail_id;primaryKey;autoIncrement;comment:邮件记录id" json:"mailId"`
	// 邮箱配置id
	MailboxID int64 `gorm:"column:mailbox_id;not null;uniqueIndex:uk_mail_log_key;index;comment:邮箱配置id" json:"mailboxId"`
	// 渠道id
	ChannelID int64 `gorm:"column:channel_id;not null;index;comment:渠道id" json:"channelId"`
	// 邮件线程id
	ThreadID int64 `gorm:"column:thread_id;not null;default:0;index;comment:邮件线程id" json:"threadId"`
	// 会话id
	SessionID int64 `gorm:"column:session_id;not null;default:0;index;comment:会话id" json:"sessionId"`
	// 关联的 kefu_message id, 0 表示尚未落消息
	KefuMessageID int64 `gorm:"column:kefu_message_id;not null;default:0;comment:关联的消息id" json:"kefuMessageId"`
	// 方向, in/out
	Direction string `gorm:"column:direction;size:8;not null;comment:方向" json:"direction"`
	// 幂等键, 入站 in:{uid}, 出站 out:{message_id}
	MailKey string `gorm:"column:mail_key;size:128;not null;uniqueIndex:uk_mail_log_key;comment:幂等键" json:"mailKey"`
	// IMAP UID, 出站为 0
	UID int64 `gorm:"column:uid;not null;default:0;comment:IMAP UID" json:"uid"`
	// 邮件 Message-ID
	MessageID string `gorm:"column:message_id;size:255;index;comment:邮件Message-ID" json:"messageId"`
	// In-Reply-To
	InReplyTo string `gorm:"column:in_reply_to;size:512;comment:In-Reply-To" json:"inReplyTo"`
	// References
	References string `gorm:"column:references;type:text;comment:References" json:"references"`
	// 发件人地址
	FromAddr string `gorm:"column:from_addr;size:255;comment:发件人地址" json:"fromAddr"`
	// 发件人名称
	FromName string `gorm:"column:from_name;size:255;comment:发件人名称" json:"fromName"`
	// 收件人地址, 多个用英文逗号分隔
	ToAddr string `gorm:"column:to_addr;size:1024;comment:收件人地址" json:"toAddr"`
	// 抄送地址, 多个用英文逗号分隔
	CcAddr string `gorm:"column:cc_addr;size:1024;comment:抄送地址" json:"ccAddr"`
	// 主题
	Subject string `gorm:"column:subject;size:512;comment:主题" json:"subject"`
	// 邮件时间
	SentTime Time `gorm:"column:sent_time;comment:邮件时间" json:"sentTime"`
	// 原始大小, 字节
	Size int64 `gorm:"column:size;not null;default:0;comment:原始大小" json:"size"`
	// 附件数量
	AttachmentCount int `gorm:"column:attachment_count;not null;default:0;comment:附件数量" json:"attachmentCount"`
	// 状态, received/skipped/failed/sending/sent
	Status string `gorm:"column:status;size:16;not null;comment:状态" json:"status"`
	// 跳过原因
	SkipReason string `gorm:"column:skip_reason;size:32;comment:跳过原因" json:"skipReason"`
	// 失败原因, 中文
	Error string `gorm:"column:error;size:512;comment:失败原因" json:"error"`
	// 原始邮件存储路径
	RawPath string `gorm:"column:raw_path;size:255;comment:原始邮件存储路径" json:"rawPath"`
	// 是否删除, 0否, 1是
	Deleted int `gorm:"column:deleted;not null;default:0;comment:是否删除" json:"deleted"`
	// 创建时间
	CreateTime Time `gorm:"column:create_time;comment:创建时间" json:"createTime"`
	// 更新时间
	UpdateTime Time `gorm:"column:update_time;comment:更新时间" json:"updateTime"`
}

// TableName 指定表名
func (KefuMailLog) TableName() string {
	return "kefu_mail_log"
}

// KefuMailLogParam 是邮件收发记录的查询参数
type KefuMailLogParam struct {
	common.PageParam
	// 邮箱配置id
	MailboxID *int64 `form:"mailboxId"`
	// 渠道id
	ChannelID *int64 `form:"channelId"`
	// 会话id
	SessionID *int64 `form:"sessionId"`
	// 方向, in/out
	Direction string `form:"direction"`
	// 状态
	Status string `form:"status"`
	// 起始时间, 格式 2006-01-02 15:04:05
	StartTime string `form:"startTime"`
	// 结束时间, 格式 2006-01-02 15:04:05
	EndTime string `form:"endTime"`
	// 关键词, 匹配主题与发件人
	Keyword string `form:"keyword"`
}

// KefuMailLogResendForm 是重发失败邮件的提交数据
type KefuMailLogResendForm struct {
	// 邮件记录id
	MailID int64 `json:"mailId"`
}
