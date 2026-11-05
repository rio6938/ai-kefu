package repository

import (
	"strconv"

	"gorm.io/gorm"

	"admin/server/model"
	"admin/server/pkg/db"
)

// KefuAnalysisRepository 是分析页的聚合查询，
// agentID 为 0 表示全量（管理员），非 0 表示只看该客服名下的会话
type KefuAnalysisRepository struct{}

// NewKefuAnalysisRepository 创建分析页的数据访问
func NewKefuAnalysisRepository() *KefuAnalysisRepository {
	return &KefuAnalysisRepository{}
}

// timeScope 按创建时间过滤，时间区间由业务层保证已填默认值
func (r *KefuAnalysisRepository) timeScope(
	query *gorm.DB, param model.KefuAnalysisParam,
) *gorm.DB {
	if param.StartTime != "" {
		query = query.Where("create_time >= ?", param.StartTime)
	}
	if param.EndTime != "" {
		query = query.Where("create_time <= ?", param.EndTime)
	}
	return query
}

// sessionScope 返回会话查询条件：时间区间 + 客服范围
func (r *KefuAnalysisRepository) sessionScope(
	agentID int64, param model.KefuAnalysisParam,
) *gorm.DB {
	query := r.timeScope(
		db.DB.Model(&model.KefuSession{}).Where("deleted = 0"), param)
	if agentID > 0 {
		query = query.Where("agent_id = ?", agentID)
	}
	return query
}

// sessionIDQuery 返回客服名下的会话id子查询，
// 消息、AI 回复与转接都通过它会话过滤
func (r *KefuAnalysisRepository) sessionIDQuery(agentID int64) *gorm.DB {
	return db.DB.Model(&model.KefuSession{}).Select("session_id").
		Where("deleted = 0 and agent_id = ?", agentID)
}

// CountSessions 统计会话数，statuses 为空时统计全部状态
func (r *KefuAnalysisRepository) CountSessions(
	agentID int64, param model.KefuAnalysisParam, statuses []int,
) (int64, error) {
	query := r.sessionScope(agentID, param)
	if len(statuses) > 0 {
		query = query.Where("status in ?", statuses)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// RateStat 统计已评价会话数与满意度均分，agentID 为 0 表示全量；
// 只统计 rate_score 大于 0 的会话，没有评价时均分返回 0
func (r *KefuAnalysisRepository) RateStat(
	agentID int64, param model.KefuAnalysisParam,
) (int64, float64, error) {
	type rateRow struct {
		// 已评价会话数
		RatedCount int64
		// 满意度均分
		AvgScore float64
	}

	var result rateRow
	if err := r.sessionScope(agentID, param).
		Where("rate_score > 0").
		Select("count(*) as rated_count, coalesce(avg(rate_score), 0) as avg_score").
		Scan(&result).Error; err != nil {
		return 0, 0, err
	}
	return result.RatedCount, result.AvgScore, nil
}

// CountSessionsByDay 按天统计会话数，key 为 2006-01-02 格式的日期
func (r *KefuAnalysisRepository) CountSessionsByDay(
	agentID int64, param model.KefuAnalysisParam,
) (map[string]int64, error) {
	return r.countByDay(r.sessionScope(agentID, param))
}

// CountAiSessionsByDay 按天统计开启 AI 自动回复的会话数
func (r *KefuAnalysisRepository) CountAiSessionsByDay(
	agentID int64, param model.KefuAnalysisParam,
) (map[string]int64, error) {
	return r.countByDay(r.sessionScope(agentID, param).
		Where("ai_mode = ?", model.KefuSessionAiModeOn))
}

// CountHumanSessionsByDay 按天统计已分配客服的会话数
func (r *KefuAnalysisRepository) CountHumanSessionsByDay(
	agentID int64, param model.KefuAnalysisParam,
) (map[string]int64, error) {
	return r.countByDay(r.sessionScope(agentID, param).Where("agent_id > 0"))
}

// CountMessagesByDay 按天统计消息数
func (r *KefuAnalysisRepository) CountMessagesByDay(
	agentID int64, param model.KefuAnalysisParam,
) (map[string]int64, error) {
	return r.countByDay(r.messageScope(agentID, param))
}

// countByDay 按 create_time 的年月日分组统计数量
func (r *KefuAnalysisRepository) countByDay(query *gorm.DB) (map[string]int64, error) {
	type dayCount struct {
		// 日期
		Date string
		// 数量
		Count int64
	}

	var list []dayCount
	err := query.
		Select("to_char(create_time, 'YYYY-MM-DD') as date, count(*) as count").
		Group("to_char(create_time, 'YYYY-MM-DD')").
		Scan(&list).Error
	if err != nil {
		return nil, err
	}

	result := make(map[string]int64, len(list))
	for _, item := range list {
		result[item.Date] = item.Count
	}
	return result, nil
}

// CountAssignedSessions 统计已分配客服的会话数，
// 管理员视角用它统计人工会话数，客服视角与 CountSessions 结果一致
func (r *KefuAnalysisRepository) CountAssignedSessions(
	agentID int64, param model.KefuAnalysisParam,
) (int64, error) {
	var count int64
	err := r.sessionScope(agentID, param).Where("agent_id > 0").Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

// CountCurrent 统计指定状态的会话数，不受时间区间限制（当前进行中的会话）
func (r *KefuAnalysisRepository) CountCurrent(
	agentID int64, status int,
) (int64, error) {
	query := db.DB.Model(&model.KefuSession{}).Where("deleted = 0 and status = ?", status)
	if agentID > 0 {
		query = query.Where("agent_id = ?", agentID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// SumUnread 统计未读消息数，不受时间区间限制（当前未读）
func (r *KefuAnalysisRepository) SumUnread(agentID int64) (int64, error) {
	query := db.DB.Model(&model.KefuSession{}).Where("deleted = 0")
	if agentID > 0 {
		query = query.Where("agent_id = ?", agentID)
	}
	var total int64
	if err := query.Select("coalesce(sum(unread_count), 0)").Scan(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

// CountSessionsByStatus 按会话状态分组统计
func (r *KefuAnalysisRepository) CountSessionsByStatus(
	agentID int64, param model.KefuAnalysisParam,
) ([]model.KefuAnalysisDistributionItem, error) {
	type statusCount struct {
		// 会话状态
		Status int
		// 数量
		Count int64
	}

	var list []statusCount
	err := r.sessionScope(agentID, param).
		Select("status, count(*) as count").
		Group("status").Scan(&list).Error
	if err != nil {
		return nil, err
	}

	result := make([]model.KefuAnalysisDistributionItem, 0, len(list))
	for _, item := range list {
		result = append(result, model.KefuAnalysisDistributionItem{
			Key:   strconv.Itoa(item.Status),
			Count: item.Count,
		})
	}
	return result, nil
}

// CountSessionsByChannel 按渠道分组统计会话数
func (r *KefuAnalysisRepository) CountSessionsByChannel(
	agentID int64, param model.KefuAnalysisParam,
) ([]model.KefuAnalysisDistributionItem, error) {
	type channelCount struct {
		// 渠道标识
		ChannelCode string
		// 数量
		Count int64
	}

	var list []channelCount
	err := r.sessionScope(agentID, param).
		Select("channel_code, count(*) as count").
		Group("channel_code").Scan(&list).Error
	if err != nil {
		return nil, err
	}

	result := make([]model.KefuAnalysisDistributionItem, 0, len(list))
	for _, item := range list {
		result = append(result, model.KefuAnalysisDistributionItem{
			Key:   item.ChannelCode,
			Count: item.Count,
		})
	}
	return result, nil
}

// CountVisitors 统计访客数，访客表没有客服归属，只看时间区间
func (r *KefuAnalysisRepository) CountVisitors(
	param model.KefuAnalysisParam,
) (int64, error) {
	query := r.timeScope(
		db.DB.Model(&model.KefuVisitor{}).Where("deleted = 0"), param)
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// CountMessages 统计消息数，fromRole 为空时统计全部角色
func (r *KefuAnalysisRepository) CountMessages(
	agentID int64, param model.KefuAnalysisParam, fromRole string,
) (int64, error) {
	query := r.messageScope(agentID, param)
	if fromRole != "" {
		query = query.Where("from_role = ?", fromRole)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// messageScope 返回会话消息的查询条件，客服视角走会话子查询
func (r *KefuAnalysisRepository) messageScope(
	agentID int64, param model.KefuAnalysisParam,
) *gorm.DB {
	query := r.timeScope(
		db.DB.Model(&model.KefuMessage{}).Where("deleted = 0"), param)
	if agentID > 0 {
		query = query.Where("session_id in (?)", r.sessionIDQuery(agentID))
	}
	return query
}

// CountAiReplies 统计 AI 回复数，statuses 为空时统计全部状态
func (r *KefuAnalysisRepository) CountAiReplies(
	agentID int64, param model.KefuAnalysisParam, statuses []string,
) (int64, error) {
	query := r.aiReplyScope(agentID, param)
	if len(statuses) > 0 {
		query = query.Where("status in ?", statuses)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// CountKnowledgeHits 统计命中知识库的 AI 回复数，rag_hits 非空即视为命中
func (r *KefuAnalysisRepository) CountKnowledgeHits(
	agentID int64, param model.KefuAnalysisParam,
) (int64, error) {
	var count int64
	err := r.aiReplyScope(agentID, param).
		Where("rag_hits is not null and rag_hits <> '' and rag_hits <> '[]'").
		Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

// SumAiTokens 统计 AI 回复消耗的 token 总数（提示词 + 回复）
func (r *KefuAnalysisRepository) SumAiTokens(
	agentID int64, param model.KefuAnalysisParam,
) (int64, error) {
	var total int64
	err := r.aiReplyScope(agentID, param).
		Select("coalesce(sum(prompt_tokens + completion_tokens), 0)").
		Scan(&total).Error
	if err != nil {
		return 0, err
	}
	return total, nil
}

// aiReplyScope 返回 AI 回复的查询条件
func (r *KefuAnalysisRepository) aiReplyScope(
	agentID int64, param model.KefuAnalysisParam,
) *gorm.DB {
	query := r.timeScope(
		db.DB.Model(&model.KefuAiReply{}).Where("deleted = 0"), param)
	if agentID > 0 {
		query = query.Where("session_id in (?)", r.sessionIDQuery(agentID))
	}
	return query
}

// CountTransfers 统计转接次数
func (r *KefuAnalysisRepository) CountTransfers(
	agentID int64, param model.KefuAnalysisParam,
) (int64, error) {
	query := r.timeScope(
		db.DB.Model(&model.KefuSessionTransfer{}).Where("deleted = 0"), param)
	if agentID > 0 {
		query = query.Where("session_id in (?)", r.sessionIDQuery(agentID))
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// AvgFirstResponse 统计平均首次响应时长（秒），口径为 start_time - queue_time，
// 只统计已接待的会话，四舍五入到秒
func (r *KefuAnalysisRepository) AvgFirstResponse(
	agentID int64, param model.KefuAnalysisParam,
) (int64, error) {
	var seconds float64
	err := r.sessionScope(agentID, param).
		Where("start_time is not null and queue_time is not null").
		Select("coalesce(avg(extract(epoch from (start_time - queue_time))), 0)").
		Scan(&seconds).Error
	if err != nil {
		return 0, err
	}
	return int64(seconds + 0.5), nil
}

// AvgSessionMinutes 统计平均会话时长（分钟），只统计起止时间都完整的会话
func (r *KefuAnalysisRepository) AvgSessionMinutes(
	agentID int64, param model.KefuAnalysisParam,
) (int64, error) {
	var minutes float64
	err := r.sessionScope(agentID, param).
		Where("start_time is not null and end_time is not null").
		Select("coalesce(avg(extract(epoch from (end_time - start_time)) / 60), 0)").
		Scan(&minutes).Error
	if err != nil {
		return 0, err
	}
	return int64(minutes + 0.5), nil
}

// RankAgents 按会话数统计客服排行，只返回区间内有会话的客服
func (r *KefuAnalysisRepository) RankAgents(
	param model.KefuAnalysisParam, limit int,
) ([]model.KefuAnalysisRankItem, error) {
	if limit <= 0 {
		return nil, nil
	}

	type agentRank struct {
		// 客服id
		AgentID int64
		// 会话数
		SessionCount int64
		// 消息数
		MessageCount int64
	}

	var list []agentRank
	err := r.timeScope(db.DB.Model(&model.KefuSession{}).
		Where("deleted = 0 and agent_id > 0"), param).
		Select("agent_id, count(*) as session_count, coalesce(sum(message_count), 0) as message_count").
		Group("agent_id").
		Order("session_count desc, agent_id asc").
		Limit(limit).
		Scan(&list).Error
	if err != nil {
		return nil, err
	}

	result := make([]model.KefuAnalysisRankItem, 0, len(list))
	for _, item := range list {
		result = append(result, model.KefuAnalysisRankItem{
			AgentID:      item.AgentID,
			SessionCount: item.SessionCount,
			MessageCount: item.MessageCount,
		})
	}
	return result, nil
}

// ListChannelNames 查询全部未删除渠道的名称，key 为渠道标识
func (r *KefuAnalysisRepository) ListChannelNames() (map[string]string, error) {
	var list []model.KefuChannel
	if err := db.DB.Model(&model.KefuChannel{}).Where("deleted = 0").
		Select("channel_code, name").Find(&list).Error; err != nil {
		return nil, err
	}
	result := make(map[string]string, len(list))
	for _, item := range list {
		result[item.ChannelCode] = item.Name
	}
	return result, nil
}
