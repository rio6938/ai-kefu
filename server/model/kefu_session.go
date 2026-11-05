package model

import "admin/server/internal/common"

// 会话状态
const (
	// KefuSessionStatusQueueing 是待接待，客户要求转人工后等待客服领取
	KefuSessionStatusQueueing = 0
	// KefuSessionStatusAi 是 AI 进行中，会话由 AI 自动回复
	KefuSessionStatusAi = 1
	// KefuSessionStatusClosed 是已结束
	KefuSessionStatusClosed = 2
	// KefuSessionStatusLeaveMessage 是留言，待接待超时转留言
	KefuSessionStatusLeaveMessage = 3
	// KefuSessionStatusHuman 是人工进行中，会话已被客服接待
	KefuSessionStatusHuman = 4
)

// AI 自动回复开关
const (
	// KefuSessionAiModeOff 是关闭 AI 自动回复
	KefuSessionAiModeOff = 0
	// KefuSessionAiModeOn 是开启 AI 自动回复
	KefuSessionAiModeOn = 1
)

// KefuSession 是客服会话，对应 kefu_session 表；
// 一个访客同时只有一个未结束的会话，对应一个 IM 群频道 session_{sessionId}
type KefuSession struct {
	// 会话id
	SessionID int64 `gorm:"column:session_id;primaryKey;autoIncrement;comment:会话id" json:"sessionId"`
	// 会话编号
	SessionNo string `gorm:"column:session_no;size:64;not null;uniqueIndex;comment:会话编号" json:"sessionNo"`
	// 渠道标识
	ChannelCode string `gorm:"column:channel_code;size:64;comment:渠道标识" json:"channelCode"`
	// 访客id
	VisitorID int64 `gorm:"column:visitor_id;not null;index;comment:访客id" json:"visitorId"`
	// 客服id, 0 表示还未分配
	AgentID int64 `gorm:"column:agent_id;not null;default:0;index;comment:客服id" json:"agentId"`
	// 会话建立时快照的关联链id, 转接不修改
	RouteID int64 `gorm:"column:route_id;not null;default:0;index;comment:会话建立时快照的关联链id" json:"routeId"`
	// 会话状态, 0待接待, 1 AI 进行中, 2已结束, 3留言, 4人工进行中
	Status int `gorm:"column:status;not null;default:0;comment:会话状态, 0待接待, 1 AI 进行中, 2已结束, 3留言, 4人工进行中" json:"status"`
	// AI 自动回复开关, 0关闭, 1开启
	AiMode int `gorm:"column:ai_mode;not null;default:1;comment:AI自动回复开关, 0关闭, 1开启" json:"aiMode"`
	// AI 回复是否失败, 0否, 1是, 工作台据此高亮提醒人工接管
	AiFailed int `gorm:"column:ai_failed;not null;default:0;comment:AI回复是否失败" json:"aiFailed"`
	// 会话临时提示词, 只对本会话生效
	PromptCustom string `gorm:"column:prompt_custom;type:text;comment:会话临时提示词" json:"promptCustom"`
	// 来源页面地址
	SourceURL string `gorm:"column:source_url;size:512;comment:来源页面地址" json:"sourceUrl"`
	// 进入待接待的时间
	QueueTime Time `gorm:"column:queue_time;comment:进入待接待的时间" json:"queueTime"`
	// 接待开始时间
	StartTime Time `gorm:"column:start_time;comment:接待开始时间" json:"startTime"`
	// 结束时间
	EndTime Time `gorm:"column:end_time;comment:结束时间" json:"endTime"`
	// 结束原因
	CloseReason string `gorm:"column:close_reason;size:255;comment:结束原因" json:"closeReason"`
	// 最后一条消息时间
	LastMessageTime Time `gorm:"column:last_message_time;comment:最后一条消息时间" json:"lastMessageTime"`
	// 消息总数
	MessageCount int `gorm:"column:message_count;not null;default:0;comment:消息总数" json:"messageCount"`
	// 未读消息数
	UnreadCount int `gorm:"column:unread_count;not null;default:0;comment:未读消息数" json:"unreadCount"`
	// 是否删除, 0否, 1是
	Deleted int `gorm:"column:deleted;not null;default:0;comment:是否删除" json:"deleted"`
	// 创建时间
	CreateTime Time `gorm:"column:create_time;comment:创建时间" json:"createTime"`
	// 更新时间
	UpdateTime Time `gorm:"column:update_time;comment:更新时间" json:"updateTime"`
	// 访客信息, 查询时回填
	Visitor *KefuVisitor `gorm:"-" json:"visitor,omitempty"`
	// 客服信息, 查询时回填
	Agent *KefuAgent `gorm:"-" json:"agent,omitempty"`
	// 规则命中的建议转接, 查询时回填, 没有命中时为 nil
	SuggestTransfer *KefuTransferSuggestion `gorm:"-" json:"suggestTransfer,omitempty"`
	// 访客是否在线, 查询时按 IM 在线路由回填
	VisitorOnline bool `gorm:"-" json:"visitorOnline"`
	// 关联链名称, 查询时回填
	RouteName string `gorm:"-" json:"routeName"`
	// 排队名次, 从 1 开始, 只在待接待时有值, 查询时回填
	QueuePosition int `gorm:"-" json:"queuePosition"`
	// 待接待会话总数, 查询时回填
	QueueSize int64 `gorm:"-" json:"queueSize"`
	// 预计等待秒数, 按排队名次与人平等待时长估算, 查询时回填
	EstimatedWaitSeconds int64 `gorm:"-" json:"estimatedWaitSeconds"`
	// 已等待秒数, 从进入待接待的时间算起, 查询时回填
	QueueWaitSeconds int64 `gorm:"-" json:"queueWaitSeconds"`
	// 最后一条消息的摘要, 查询时回填
	LastMessage string `gorm:"-" json:"lastMessage"`
	// 会话标签, 查询时回填
	Tags []string `gorm:"-" json:"tags"`
	// 内部备注, 仅坐席可见, 不推送也不出现在消息回放里
	Remark string `gorm:"column:remark;size:500;comment:内部备注" json:"remark"`
	// 满意度评分, 1-5, 0 表示未评价
	RateScore int `gorm:"column:rate_score;not null;default:0;comment:满意度评分, 1-5, 0 表示未评价" json:"rateScore"`
	// 满意度标签, 英文逗号分隔, 取自字典 kefu_rate_tag
	RateTags string `gorm:"column:rate_tags;size:255;comment:满意度标签" json:"rateTags"`
	// 满意度备注
	RateRemark string `gorm:"column:rate_remark;size:255;comment:满意度备注" json:"rateRemark"`
	// 评价时间
	RateTime Time `gorm:"column:rate_time;comment:评价时间" json:"rateTime"`
	// 渠道类型, 查询时回填
	ChannelType string `gorm:"-" json:"channelType"`
	// 邮件主题, 邮件会话回填
	MailSubject string `gorm:"-" json:"mailSubject"`
	// 邮件发件人, 邮件会话回填
	MailFrom string `gorm:"-" json:"mailFrom"`
	// 邮件收件人, 邮件会话回填
	MailTo string `gorm:"-" json:"mailTo"`
	// 发件人的历史邮件线程数, 邮件会话回填
	MailThreadCount int64 `gorm:"-" json:"mailThreadCount"`
	// 邮箱最近同步时间, 邮件会话回填
	MailLastSyncTime Time `gorm:"-" json:"mailLastSyncTime"`
}

// TableName 指定表名
func (KefuSession) TableName() string {
	return "kefu_session"
}

// KefuSessionParam 是会话的查询参数
type KefuSessionParam struct {
	common.PageParam
	// 会话编号
	SessionNo string `form:"sessionNo"`
	// 渠道标识
	ChannelCode string `form:"channelCode"`
	// 访客id
	VisitorID *int64 `form:"visitorId"`
	// 客服id
	AgentID *int64 `form:"agentId"`
	// 会话状态
	Status *int `form:"status"`
	// AI 自动回复开关
	AiMode *int `form:"aiMode"`
	// AI 回复是否失败
	AiFailed *int `form:"aiFailed"`
	// 开始时间, 格式 2006-01-02 15:04:05
	StartTime string `form:"startTime"`
	// 结束时间, 格式 2006-01-02 15:04:05
	EndTime string `form:"endTime"`
	// 关键词, 匹配会话编号与访客昵称
	Keyword string `form:"keyword"`
	// 标签编码, 按 kefu_session_tag 关联过滤
	TagCode string `form:"tagCode"`
	// 渠道类型, email 时只看邮箱会话
	ChannelType string `form:"channelType"`
}

// KefuSessionCloseForm 是结束会话的提交数据
type KefuSessionCloseForm struct {
	// 会话id, 批量结束时使用
	SessionIDs []int64 `json:"sessionIds"`
	// 结束原因
	CloseReason string `json:"closeReason"`
}

// KefuSessionAiModeForm 是 AI 开关的提交数据
type KefuSessionAiModeForm struct {
	// 会话id
	SessionID int64 `json:"sessionId"`
	// AI 自动回复开关, 0关闭, 1开启
	AiMode int `json:"aiMode"`
}

// KefuSessionAiPromptForm 是会话临时提示词的提交数据
type KefuSessionAiPromptForm struct {
	// 会话id
	SessionID int64 `json:"sessionId"`
	// 会话临时提示词, 空串表示清空
	PromptCustom string `json:"promptCustom"`
}

// KefuSessionAiPrompt 是工作台查看会话 AI 提示词的结果
type KefuSessionAiPrompt struct {
	// 会话id
	SessionID int64 `json:"sessionId"`
	// 会话临时提示词
	PromptCustom string `json:"promptCustom"`
	// 按当前上下文渲染后的提示词
	Prompt string `json:"prompt"`
	// 最近一次 AI 回复实际使用的提示词快照
	PromptSnapshot string `json:"promptSnapshot"`
	// 最近一次 AI 回复id
	LastReplyID int64 `json:"lastReplyId"`
}

// KefuSessionHandoffResult 是访客主动转人工的结果

type KefuSessionHandoffResult struct {
	// 会话id
	SessionID int64 `json:"sessionId"`
	// 会话状态, 转人工后为待接待
	Status int `json:"status"`
	// 是否有人工客服在线, 无人在线时访客端直接展示留言卡片
	OnlineAgent bool `json:"onlineAgent"`
}

// KefuSessionRateForm 是访客评价会话的提交数据
type KefuSessionRateForm struct {
	// 满意度评分, 1-5
	Score int `json:"score"`
	// 满意度标签, 取自字典 kefu_rate_tag
	Tags []string `json:"tags"`
	// 备注, 最长 200 字
	Remark string `json:"remark"`
}

// KefuSessionTagsForm 是设置会话标签的提交数据，覆盖式设置
type KefuSessionTagsForm struct {
	// 会话id
	SessionID int64 `json:"sessionId"`
	// 标签编码, 取自字典 kefu_session_tag
	Tags []string `json:"tags"`
}

// KefuSessionRemarkForm 是保存会话内部备注的提交数据
type KefuSessionRemarkForm struct {
	// 会话id
	SessionID int64 `json:"sessionId"`
	// 内部备注, 空串表示清空
	Remark string `json:"remark"`
}

// KefuSessionAssignForm 是领取接待的提交数据
type KefuSessionAssignForm struct {
	// 会话id
	SessionID int64 `json:"sessionId"`
	// 目标客服id, 0 表示领取到当前客服名下
	AgentID int64 `json:"agentId"`
}

// KefuWorkbenchSummary 是客服工作台的顶部统计
type KefuWorkbenchSummary struct {
	// 待接待会话数
	QueueCount int64 `json:"queueCount"`
	// AI 进行中会话数
	AiCount int64 `json:"aiCount"`
	// 人工进行中会话数
	HumanCount int64 `json:"humanCount"`
	// 未读消息数
	UnreadCount int64 `json:"unreadCount"`
	// AI 回复失败待人工接管的会话数
	AiFailedCount int64 `json:"aiFailedCount"`
	// 全部会话数, 与列表「全部」页签的可见范围一致
	AllCount int64 `json:"allCount"`
	// 已结束会话数, 与列表「已结束」页签的可见范围一致
	ClosedCount int64 `json:"closedCount"`
	// 待接待最长等待秒数, 按最早的排队时间计算, 用于工作台横幅
	QueueMaxWaitSeconds int64 `json:"queueMaxWaitSeconds"`
	// 排队最早的待接待会话id, 0 表示没有待接待
	EarliestQueueID int64 `json:"earliestQueueId"`
	// 当前客服的工作状态, 0离线, 1在线, 2小休
	WorkStatus int `json:"workStatus"`
	// 小休原因
	RestReason string `json:"restReason"`
	// 最大同时接待会话数
	MaxConcurrent int `json:"maxConcurrent"`
	// 当前客服名下人工进行中的会话数, 用于状态条「当前接待 N / M」
	MySessionCount int64 `json:"mySessionCount"`
}
