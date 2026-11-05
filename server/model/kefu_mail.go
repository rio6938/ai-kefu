package model

// KefuMailAttachment 是邮件附件的回显结构，存在 kefu_message.payload 与发信记录里
type KefuMailAttachment struct {
	// 文件id, 对应 sys_file.id
	FileID int64 `json:"fileId"`
	// 文件名
	Name string `json:"name"`
	// 文件大小, 字节
	Size int64 `json:"size"`
	// 文件类型
	ContentType string `json:"contentType"`
	// 文件访问地址
	URL string `json:"url"`
}

// KefuMailAttachmentForm 是发送邮件时携带的附件
type KefuMailAttachmentForm struct {
	// 文件id
	FileID int64 `json:"fileId"`
	// 文件名, 为空时按 sys_file 的名称
	Name string `json:"name"`
}

// KefuMailDraftResult 是当前会话的邮件 AI 草稿或待发送记录
type KefuMailDraftResult struct {
	// AI 回复记录id
	ReplyID int64 `json:"replyId"`
	// 会话id
	SessionID int64 `json:"sessionId"`
	// 邮件线程id
	MailThreadID int64 `json:"mailThreadId"`
	// 建议的主题
	Subject string `json:"subject"`
	// 建议的正文, 纯文本
	Content string `json:"content"`
	// 发送状态
	SendStatus string `json:"sendStatus"`
	// 计划发送时间
	ScheduledTime Time `json:"scheduledTime"`
	// 生成时间
	CreateTime Time `json:"createTime"`
}

// KefuMailDraftSendForm 是采用并发送邮件草稿的提交数据
type KefuMailDraftSendForm struct {
	// 会话id
	SessionID int64 `json:"sessionId"`
	// AI 回复记录id, 0 表示直接发送当前客服输入的内容
	ReplyID int64 `json:"replyId"`
	// 主题, 为空时用 Re: 原主题
	Subject string `json:"subject"`
	// 正文, 为空时用草稿内容
	Content string `json:"content"`
	// 附件
	Attachments []KefuMailAttachmentForm `json:"attachments"`
}

// KefuMailAnalysisItem 是邮件统计的一组指标
type KefuMailAnalysisItem struct {
	// 维度值, 如日期、邮箱地址, 汇总时为空
	Label string `json:"label"`
	// 收信数
	InboundCount int64 `json:"inboundCount"`
	// 回信数
	OutboundCount int64 `json:"outboundCount"`
	// AI 自动发送数
	AiSentCount int64 `json:"aiSentCount"`
	// AI 草稿采用数
	AiDraftUsedCount int64 `json:"aiDraftUsedCount"`
	// 发送失败数
	FailedCount int64 `json:"failedCount"`
	// 自动跳过数
	SkippedCount int64 `json:"skippedCount"`
	// 中文邮件数
	ChineseCount int64 `json:"chineseCount"`
	// 英文邮件数
	EnglishCount int64 `json:"englishCount"`
	// 平均首次响应时长, 秒
	AvgFirstResponseSeconds int64 `json:"avgFirstResponseSeconds"`
	// 平均会话时长, 秒
	AvgSessionSeconds int64 `json:"avgSessionSeconds"`
}

// KefuMailAnalysisResult 是邮件统计的返回结构
type KefuMailAnalysisResult struct {
	// 汇总指标
	Summary KefuMailAnalysisItem `json:"summary"`
	// 明细指标, groupBy 为空时只有一条汇总
	Items []KefuMailAnalysisItem `json:"items"`
}

// KefuMailAnalysisParam 是邮件统计的查询参数
type KefuMailAnalysisParam struct {
	// 开始时间, 格式 2006-01-02 15:04:05
	StartTime string `form:"startTime"`
	// 结束时间, 格式 2006-01-02 15:04:05
	EndTime string `form:"endTime"`
	// 渠道id
	ChannelID *int64 `form:"channelId"`
	// 邮箱配置id
	MailboxID *int64 `form:"mailboxId"`
	// 客服id
	AgentID *int64 `form:"agentId"`
	// 分组维度, day/mailbox/agent, 为空不分组
	GroupBy string `form:"groupBy"`
}

// KefuMailTestResult 是邮箱连接测试的结果
type KefuMailTestResult struct {
	// IMAP 是否通过
	IMAPOK bool `json:"imapOk"`
	// SMTP 是否通过
	SMTPOK bool `json:"smtpOk"`
	// 逐项测试的中文说明
	Messages []string `json:"messages"`
}

// KefuMailOAuthURLResult 是获取 OAuth2 授权地址的返回
type KefuMailOAuthURLResult struct {
	// 授权地址, 后台打开新窗口
	URL string `json:"url"`
}
