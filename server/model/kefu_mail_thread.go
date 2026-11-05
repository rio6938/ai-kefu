package model

// 邮件线程状态
const (
	// KefuMailThreadStatusActive 是活跃
	KefuMailThreadStatusActive = 0
	// KefuMailThreadStatusClosed 是已关闭
	KefuMailThreadStatusClosed = 1
)

// KefuMailThread 是邮件线程，对应 kefu_mail_thread 表；
// 一个线程对应一个客服会话，线程内进出邮件都算一次
type KefuMailThread struct {
	// 线程id
	ThreadID int64 `gorm:"column:thread_id;primaryKey;autoIncrement;comment:线程id" json:"threadId"`
	// 渠道id
	ChannelID int64 `gorm:"column:channel_id;not null;uniqueIndex:uk_mail_thread_root;comment:渠道id" json:"channelId"`
	// 渠道标识
	ChannelCode string `gorm:"column:channel_code;size:64;comment:渠道标识" json:"channelCode"`
	// 发件人对应的访客id
	VisitorID int64 `gorm:"column:visitor_id;not null;index;comment:发件人对应的访客id" json:"visitorId"`
	// 当前关联的会话id
	SessionID int64 `gorm:"column:session_id;not null;index;comment:当前关联的会话id" json:"sessionId"`
	// 线程根邮件的 Message-ID
	RootMessageID string `gorm:"column:root_message_id;size:255;not null;uniqueIndex:uk_mail_thread_root;comment:线程根邮件的Message-ID" json:"rootMessageId"`
	// 原始主题
	Subject string `gorm:"column:subject;size:512;comment:原始主题" json:"subject"`
	// 归一化主题, 去掉 Re/Fwd/回复/转发 前缀后转小写
	SubjectKey string `gorm:"column:subject_key;size:512;comment:归一化主题" json:"subjectKey"`
	// 最近一封客户来信的 Message-ID
	LastInboundMessageID string `gorm:"column:last_inbound_message_id;size:255;comment:最近一封客户来信的Message-ID" json:"lastInboundMessageId"`
	// 线程引用链, 空格分隔
	References string `gorm:"column:references;type:text;comment:线程引用链" json:"references"`
	// 线程内邮件数, 进出都算
	MailCount int `gorm:"column:mail_count;not null;default:0;comment:线程内邮件数" json:"mailCount"`
	// 最近一封邮件时间
	LastMailTime Time `gorm:"column:last_mail_time;comment:最近一封邮件时间" json:"lastMailTime"`
	// 状态, 0活跃, 1已关闭
	Status int `gorm:"column:status;not null;default:0;comment:状态, 0活跃, 1已关闭" json:"status"`
	// 是否删除, 0否, 1是
	Deleted int `gorm:"column:deleted;not null;default:0;comment:是否删除" json:"deleted"`
	// 创建时间
	CreateTime Time `gorm:"column:create_time;comment:创建时间" json:"createTime"`
	// 更新时间
	UpdateTime Time `gorm:"column:update_time;comment:更新时间" json:"updateTime"`
}

// TableName 指定表名
func (KefuMailThread) TableName() string {
	return "kefu_mail_thread"
}
