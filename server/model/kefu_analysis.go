package model

import "admin/server/internal/common"

// KefuAnalysisParam 是分析页的查询参数
type KefuAnalysisParam struct {
	common.PageParam
	// 开始时间, 格式 2006-01-02 15:04:05, 为空时默认最近 7 天
	StartTime string `form:"startTime"`
	// 结束时间, 格式 2006-01-02 15:04:05, 为空时默认取当前时间
	EndTime string `form:"endTime"`
}

// KefuAnalysisOverview 是管理员视角的统计指标
type KefuAnalysisOverview struct {
	// 会话总量
	SessionTotal int64 `json:"sessionTotal"`
	// 今日新增会话数
	SessionToday int64 `json:"sessionToday"`
	// 访客总量
	VisitorTotal int64 `json:"visitorTotal"`
	// 今日新增访客数
	VisitorToday int64 `json:"visitorToday"`
	// 消息总量
	MessageTotal int64 `json:"messageTotal"`
	// AI 回复数
	AiReplyCount int64 `json:"aiReplyCount"`
	// AI 回复成功率, 正常完成的回复占全部回复的百分比
	AiSuccessRate float64 `json:"aiSuccessRate"`
	// 人工会话数, 即会话已分配客服的数量
	HumanSessionCount int64 `json:"humanSessionCount"`
	// 转接次数
	TransferCount int64 `json:"transferCount"`
	// 转人工率, 转接次数占会话总量的百分比
	TransferRate float64 `json:"transferRate"`
	// 平均首次响应时长, 秒
	AvgFirstResponseSeconds int64 `json:"avgFirstResponseSeconds"`
	// 平均会话时长, 分钟
	AvgSessionMinutes int64 `json:"avgSessionMinutes"`
	// 当前待接待会话数
	QueueCount int64 `json:"queueCount"`
	// 当前 AI 进行中会话数
	AiCount int64 `json:"aiCount"`
	// 当前人工进行中会话数
	HumanCount int64 `json:"humanCount"`
	// 当前未读消息数
	UnreadCount int64 `json:"unreadCount"`
	// 知识库命中回复数
	KnowledgeHitCount int64 `json:"knowledgeHitCount"`
	// token 消耗总数, 提示词与回复之和
	TokenTotal int64 `json:"tokenTotal"`
	// 已评价会话数
	RateCount int64 `json:"rateCount"`
	// 满意度均分, 1-5, 没有评价时为 0
	RateScoreAvg float64 `json:"rateScoreAvg"`
	// 评价率, 已评价会话占已结束会话的百分比
	RatePercent float64 `json:"ratePercent"`
}

// KefuAnalysisAgent 是客服视角的统计指标
type KefuAnalysisAgent struct {
	// 我的会话总量
	MySessionTotal int64 `json:"mySessionTotal"`
	// 我的今日新增会话数
	MySessionToday int64 `json:"mySessionToday"`
	// 我的消息数
	MyMessageCount int64 `json:"myMessageCount"`
	// 我的平均首次响应时长, 秒
	MyAvgFirstResponseSeconds int64 `json:"myAvgFirstResponseSeconds"`
	// 当前待接待会话数
	MyQueueCount int64 `json:"myQueueCount"`
	// 当前 AI 进行中会话数
	MyAiCount int64 `json:"myAiCount"`
	// 当前人工进行中会话数
	MyHumanCount int64 `json:"myHumanCount"`
	// 我的已评价会话数
	MyRateCount int64 `json:"myRateCount"`
	// 我的满意度均分, 1-5, 没有评价时为 0
	MyRateScoreAvg float64 `json:"myRateScoreAvg"`
	// 我的评价率, 已评价会话占已结束会话的百分比
	MyRatePercent float64 `json:"myRatePercent"`
}

// KefuAnalysisTrend 是会话趋势的按天统计，缺口日期由业务层补 0
type KefuAnalysisTrend struct {
	// 日期, 格式 2006-01-02
	Date string `json:"date"`
	// 会话数
	SessionCount int64 `json:"sessionCount"`
	// 消息数
	MessageCount int64 `json:"messageCount"`
	// AI 会话数, 即开启 AI 自动回复的会话数
	AiCount int64 `json:"aiCount"`
	// 人工会话数, 即已分配客服的会话数
	HumanCount int64 `json:"humanCount"`
}

// KefuAnalysisRankItem 是客服排行的一行
type KefuAnalysisRankItem struct {
	// 客服id
	AgentID int64 `json:"agentId"`
	// 客服昵称
	Nickname string `json:"nickname"`
	// 会话数
	SessionCount int64 `json:"sessionCount"`
	// 消息数
	MessageCount int64 `json:"messageCount"`
}

// KefuAnalysisDistributionItem 是分布统计的一项，
// 渠道分布、会话状态分布共用该结构
type KefuAnalysisDistributionItem struct {
	// 分布项的标识
	Key string `json:"key"`
	// 分布项的名称
	Name string `json:"name"`
	// 数量
	Count int64 `json:"count"`
}

// KefuAnalysisResult 是分析页的完整返回，
// 管理员视角只有 overview 与排行，客服视角只有 agent
type KefuAnalysisResult struct {
	// 管理员视角统计, 客服视角为 nil
	Overview *KefuAnalysisOverview `json:"overview,omitempty"`
	// 客服视角统计, 管理员视角为 nil
	Agent *KefuAnalysisAgent `json:"agent,omitempty"`
	// 会话趋势
	Trend []KefuAnalysisTrend `json:"trend"`
	// 渠道分布
	ChannelDistribution []KefuAnalysisDistributionItem `json:"channelDistribution"`
	// 会话状态分布
	StatusDistribution []KefuAnalysisDistributionItem `json:"statusDistribution"`
	// 客服排行
	AgentRanking []KefuAnalysisRankItem `json:"agentRanking"`
}
