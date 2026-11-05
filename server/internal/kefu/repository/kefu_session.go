package repository

import (
	"errors"

	"gorm.io/gorm"

	"admin/server/internal/common"
	"admin/server/model"
	"admin/server/pkg/db"
)

// 常见的业务错误
var (
	// ErrSessionNotFound 是会话不存在
	ErrSessionNotFound = errors.New("会话不存在")
)

// KefuSessionRepository 是会话的数据访问
type KefuSessionRepository struct{}

// NewKefuSessionRepository 创建会话的数据访问
func NewKefuSessionRepository() *KefuSessionRepository {
	return &KefuSessionRepository{}
}

// db 返回带 kefu_session 条件的查询
func (r *KefuSessionRepository) db() *gorm.DB {
	return db.DB.Model(&model.KefuSession{}).Where("deleted = 0")
}

// GetByID 按 id 查询会话
func (r *KefuSessionRepository) GetByID(sessionID int64) (*model.KefuSession, error) {
	session := new(model.KefuSession)
	err := r.db().Where("session_id = ?", sessionID).First(session).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, err
	}
	return session, nil
}

// GetActiveByVisitor 查询访客当前未结束的会话，用于 bootstrap 复用会话
func (r *KefuSessionRepository) GetActiveByVisitor(visitorID int64) (*model.KefuSession, error) {
	session := new(model.KefuSession)
	err := r.db().
		Where("visitor_id = ? and status in ?", visitorID,
			[]int{model.KefuSessionStatusQueueing, model.KefuSessionStatusAi,
				model.KefuSessionStatusHuman}).
		Order("session_id desc").First(session).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, err
	}
	return session, nil
}

// Create 新增会话
func (r *KefuSessionRepository) Create(session *model.KefuSession) error {
	return db.DB.Create(session).Error
}

// Update 修改会话
func (r *KefuSessionRepository) Update(session *model.KefuSession) error {
	return db.DB.Save(session).Error
}

// Page 分页查询会话，agentID 大于 0 时按客服的可见范围过滤：
// 查全部时是「自己名下的会话 + 还没有客服接待的会话」，
// 待接待与 AI 进行中的会话没有接待客服，所有客服都能看到，便于领取与查看
func (r *KefuSessionRepository) Page(
	param model.KefuSessionParam, page common.PageParam, agentID int64,
) ([]model.KefuSession, int64, error) {
	query := r.visibleScope(r.scope(param), param.Status, agentID)

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	var list []model.KefuSession
	query = query.Offset(page.Offset()).Limit(page.PageSize())
	for _, order := range orderClausesWith(page, "last_message_time desc") {
		query = query.Order(order)
	}
	if err := query.Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, count, nil
}

// isUnassignedStatus 判断查询的是不是没有接待客服的会话状态：
// 待接待与 AI 进行中的会话 agent_id 恒为 0
func isUnassignedStatus(status *int) bool {
	if status == nil {
		return false
	}
	return *status == model.KefuSessionStatusQueueing || *status == model.KefuSessionStatusAi
}

// visibleScope 追加会话的可见范围：agentID 大于 0 时只看自己名下的会话与还没有客服接待的会话，
// 页签计数与分页查询共用，保证数字与列表总数一致
func (r *KefuSessionRepository) visibleScope(
	query *gorm.DB, status *int, agentID int64,
) *gorm.DB {
	if agentID <= 0 || isUnassignedStatus(status) {
		return query
	}
	if status == nil {
		return query.Where("(agent_id = ? or agent_id = 0)", agentID)
	}
	return query.Where("agent_id = ?", agentID)
}

// CountVisible 按可见范围统计指定状态的会话数，status 为 nil 时统计全部
func (r *KefuSessionRepository) CountVisible(status *int, agentID int64) (int64, error) {
	query := r.db()
	if status != nil {
		query = query.Where("status = ?", *status)
	}
	query = r.visibleScope(query, status, agentID)

	var count int64
	err := query.Count(&count).Error
	return count, err
}

// ListQueueOrder 按排队顺序返回待接待会话的 id，下标 + 1 即排队名次；
// 名次按 queue_time 升序，同一时刻按 session_id 升序保证结果稳定
func (r *KefuSessionRepository) ListQueueOrder() ([]int64, error) {
	var ids []int64
	err := r.db().Where("status = ?", model.KefuSessionStatusQueueing).
		Order("queue_time asc, session_id asc").Pluck("session_id", &ids).Error
	if err != nil {
		return nil, err
	}
	return ids, nil
}

// RateSession 写入访客评价，已有评价时不覆盖，返回是否写入成功；
// 条件更新保证同一会话只会被评价一次，并发提交只有一个能成功
func (r *KefuSessionRepository) RateSession(
	sessionID int64, score int, tags, remark string, now model.Time,
) (bool, error) {
	result := db.DB.Model(&model.KefuSession{}).
		Where("session_id = ? and deleted = 0 and status = ? and rate_score = 0",
			sessionID, model.KefuSessionStatusClosed).
		Updates(map[string]any{
			"rate_score":  score,
			"rate_tags":   tags,
			"rate_remark": remark,
			"rate_time":   now,
			"update_time": now,
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

// EarliestQueue 查询排队最早的待接待会话，工作台「接待最早的一位」用它
func (r *KefuSessionRepository) EarliestQueue() (*model.KefuSession, error) {
	session := new(model.KefuSession)
	err := r.db().Where("status = ?", model.KefuSessionStatusQueueing).
		Order("queue_time asc, session_id asc").First(session).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, err
	}
	return session, nil
}

// scope 按查询参数拼装筛选条件
func (r *KefuSessionRepository) scope(param model.KefuSessionParam) *gorm.DB {
	query := r.db()
	if param.SessionNo != "" {
		query = query.Where("session_no like ?", "%"+param.SessionNo+"%")
	}
	if param.ChannelCode != "" {
		query = query.Where("channel_code = ?", param.ChannelCode)
	}
	if param.VisitorID != nil {
		query = query.Where("visitor_id = ?", *param.VisitorID)
	}
	if param.AgentID != nil {
		query = query.Where("agent_id = ?", *param.AgentID)
	}
	if param.Status != nil {
		query = query.Where("status = ?", *param.Status)
	}
	if param.AiMode != nil {
		query = query.Where("ai_mode = ?", *param.AiMode)
	}
	if param.AiFailed != nil {
		query = query.Where("ai_failed = ?", *param.AiFailed)
	}
	if param.StartTime != "" {
		query = query.Where("create_time >= ?", param.StartTime)
	}
	if param.EndTime != "" {
		query = query.Where("create_time <= ?", param.EndTime)
	}
	if param.Keyword != "" {
		// 关键词同时匹配会话编号与访客昵称
		keyword := "%" + param.Keyword + "%"
		query = query.Where(
			"session_no like ? or visitor_id in (select visitor_id from kefu_visitor where deleted = 0 and nickname like ?)",
			keyword, keyword)
	}
	if param.TagCode != "" {
		query = query.Where(
			"session_id in (select session_id from kefu_session_tag where tag_code = ?)",
			param.TagCode)
	}
	return query
}

// UpdateMessageStat 更新会话的最后消息时间与消息计数
func (r *KefuSessionRepository) UpdateMessageStat(sessionID int64, lastTime model.Time, unreadDelta int) error {
	return db.DB.Model(&model.KefuSession{}).Where("session_id = ?", sessionID).
		Updates(map[string]any{
			"last_message_time": lastTime,
			"message_count":     gorm.Expr("message_count + 1"),
			"unread_count":      gorm.Expr("unread_count + ?", unreadDelta),
			"update_time":       lastTime,
		}).Error
}

// UpdateFields 按字段更新会话，只更新传入的字段
func (r *KefuSessionRepository) UpdateFields(sessionID int64, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	fields["update_time"] = model.Now()
	return db.DB.Model(&model.KefuSession{}).Where("session_id = ?", sessionID).
		Updates(fields).Error
}

// Close 批量结束会话
func (r *KefuSessionRepository) Close(sessionIDs []int64, closeReason string) error {
	if len(sessionIDs) == 0 {
		return nil
	}
	now := model.Now()
	return db.DB.Model(&model.KefuSession{}).Where("session_id in ?", sessionIDs).
		Updates(map[string]any{
			"status":       model.KefuSessionStatusClosed,
			"end_time":     now,
			"close_reason": closeReason,
			"update_time":  now,
		}).Error
}

// SumUnread 统计客服名下会话的未读消息数
func (r *KefuSessionRepository) SumUnread(agentID int64) (int64, error) {
	var total int64
	err := r.db().Where("agent_id = ?", agentID).
		Select("coalesce(sum(unread_count), 0)").Scan(&total).Error
	return total, err
}

// CountAiFailed 统计客服名下 AI 回复失败的会话数
func (r *KefuSessionRepository) CountAiFailed(agentID int64) (int64, error) {
	var count int64
	err := r.db().Where("agent_id = ? and ai_failed = 1 and status = ?", agentID,
		model.KefuSessionStatusHuman).Count(&count).Error
	return count, err
}

// BindAgent 把待接待或 AI 进行中的会话原子地绑定到客服名下，转成人工进行中，
// 已被别人接待时返回 false；接待后不再触发 AI 回复，因此同时关掉 AI 开关
func (r *KefuSessionRepository) BindAgent(sessionID, agentID int64) (bool, error) {
	now := model.Now()
	result := db.DB.Model(&model.KefuSession{}).
		Where("session_id = ? and agent_id = 0 and status in ?", sessionID,
			[]int{model.KefuSessionStatusQueueing, model.KefuSessionStatusAi}).
		Updates(map[string]any{
			"agent_id":    agentID,
			"status":      model.KefuSessionStatusHuman,
			"ai_mode":     model.KefuSessionAiModeOff,
			"start_time":  now,
			"update_time": now,
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

// ResetUnread 清空会话的未读消息数
func (r *KefuSessionRepository) ResetUnread(sessionID int64) error {
	return r.UpdateFields(sessionID, map[string]any{"unread_count": 0})
}

// CountActiveByAgents 批量统计客服名下人工进行中的会话数，分配时用于比较负载
func (r *KefuSessionRepository) CountActiveByAgents(
	agentIDs []int64,
) (map[int64]int64, error) {
	result := make(map[int64]int64, len(agentIDs))
	if len(agentIDs) == 0 {
		return result, nil
	}

	type agentCount struct {
		// 客服id
		AgentID int64
		// 进行中的会话数
		Total int64
	}
	var list []agentCount
	err := r.db().
		Select("agent_id, count(*) as total").
		Where("agent_id in ? and status = ?", agentIDs, model.KefuSessionStatusHuman).
		Group("agent_id").Scan(&list).Error
	if err != nil {
		return nil, err
	}
	for _, item := range list {
		result[item.AgentID] = item.Total
	}
	return result, nil
}

// ListActiveByAgent 查询客服名下人工进行中的会话，客服停用时据此批量转出
func (r *KefuSessionRepository) ListActiveByAgent(agentID int64) ([]model.KefuSession, error) {
	var list []model.KefuSession
	if err := r.db().
		Where("agent_id = ? and status = ?", agentID, model.KefuSessionStatusHuman).
		Order("session_id asc").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// ListQueueTimeout 查询待接待超过指定时间点的会话，定时任务据此转留言
func (r *KefuSessionRepository) ListQueueTimeout(
	before model.Time, limit int,
) ([]model.KefuSession, error) {
	query := r.db().
		Where("agent_id = 0 and status = ? and queue_time <= ?",
			model.KefuSessionStatusQueueing, before.Time).
		Order("session_id asc")
	if limit > 0 {
		query = query.Limit(limit)
	}
	var list []model.KefuSession
	if err := query.Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// ReturnToQueue 把会话退回待接待，没有可用客服转出时使用
func (r *KefuSessionRepository) ReturnToQueue(sessionID int64, queueTime model.Time) error {
	return db.DB.Model(&model.KefuSession{}).Where("session_id = ?", sessionID).
		Updates(map[string]any{
			"agent_id":    0,
			"status":      model.KefuSessionStatusQueueing,
			"queue_time":  queueTime,
			"start_time":  model.Time{},
			"update_time": model.Now(),
		}).Error
}
