package repository

import (
	"admin/server/model"
	"admin/server/pkg/db"
)

// MailAnalysisFilter 是邮件统计的查询条件
type MailAnalysisFilter struct {
	// 开始时间
	Start model.Time
	// 结束时间
	End model.Time
	// 渠道id
	ChannelID *int64
	// 邮箱配置id
	MailboxID *int64
	// 客服id
	AgentID *int64
	// 分组维度, day/mailbox/agent, 为空不分组
	GroupBy string
}

// KefuMailAnalysisRepository 是邮件统计的数据访问
type KefuMailAnalysisRepository struct{}

// NewKefuMailAnalysisRepository 创建邮件统计的数据访问
func NewKefuMailAnalysisRepository() *KefuMailAnalysisRepository {
	return &KefuMailAnalysisRepository{}
}

// MailLogStats 按条件聚合收发记录，groupBy 决定分组维度
func (r *KefuMailAnalysisRepository) MailLogStats(
	filter MailAnalysisFilter,
) ([]model.KefuMailAnalysisItem, error) {
	label, joins := mailAnalysisGroup(filter.GroupBy)
	query := db.DB.Table("kefu_mail_log")
	if joins != "" {
		query = query.Joins(joins)
	}
	query = query.Where("kefu_mail_log.deleted = 0").
		Where("kefu_mail_log.sent_time >= ? and kefu_mail_log.sent_time <= ?",
			filter.Start, filter.End)
	if filter.ChannelID != nil {
		query = query.Where("kefu_mail_log.channel_id = ?", *filter.ChannelID)
	}
	if filter.MailboxID != nil {
		query = query.Where("kefu_mail_log.mailbox_id = ?", *filter.MailboxID)
	}
	if filter.AgentID != nil {
		query = query.Joins("join kefu_session on kefu_session.session_id = kefu_mail_log.session_id").
			Where("kefu_session.agent_id = ?", *filter.AgentID)
	}

	selectSQL := `
		` + label + ` as label,
		count(*) filter (where kefu_mail_log.direction = 'in' and kefu_mail_log.status = 'received') as inbound_count,
		count(*) filter (where kefu_mail_log.direction = 'out' and kefu_mail_log.status = 'sent') as outbound_count,
		count(*) filter (where kefu_mail_log.direction = 'out' and kefu_mail_log.status = 'failed') as failed_count,
		count(*) filter (where kefu_mail_log.direction = 'in' and kefu_mail_log.status = 'skipped') as skipped_count,
		count(*) filter (where kefu_mail_log.direction = 'in' and kefu_mail_log.status = 'received'
			and kefu_mail_log.subject ~ '[一-龥]') as chinese_count,
		count(*) filter (where kefu_mail_log.direction = 'in' and kefu_mail_log.status = 'received'
			and kefu_mail_log.subject !~ '[一-龥]') as english_count`

	query = query.Select(selectSQL)
	if filter.GroupBy != "" {
		query = query.Group(label).Order(label + " asc")
	}

	var list []model.KefuMailAnalysisItem
	if err := query.Scan(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// mailAnalysisGroup 返回分组表达式与需要的关联表
func mailAnalysisGroup(groupBy string) (string, string) {
	switch groupBy {
	case "day":
		return "to_char(kefu_mail_log.sent_time, 'YYYY-MM-DD')", ""
	case "mailbox":
		return "kefu_mail_log.mailbox_id::text", ""
	case "agent":
		return "kefu_session.agent_id::text",
			"join kefu_session on kefu_session.session_id = kefu_mail_log.session_id"
	default:
		return "''", ""
	}
}

// MailAiStats 聚合 AI 回复数据：自动发送数与草稿采用数
func (r *KefuMailAnalysisRepository) MailAiStats(
	filter MailAnalysisFilter,
) (aiSent int64, draftUsed int64, err error) {
	query := db.DB.Table("kefu_ai_reply").
		Where("kefu_ai_reply.deleted = 0 and kefu_ai_reply.channel = ?",
			model.KefuAiReplyChannelMail).
		Where("kefu_ai_reply.update_time >= ? and kefu_ai_reply.update_time <= ?",
			filter.Start, filter.End).
		Where("kefu_ai_reply.send_status = ?", model.KefuAiReplySendSent)
	if filter.MailboxID != nil || filter.ChannelID != nil {
		query = query.Joins("join kefu_mail_thread on kefu_mail_thread.thread_id = kefu_ai_reply.mail_thread_id")
	}
	if filter.MailboxID != nil {
		query = query.Joins("join kefu_mailbox on kefu_mailbox.channel_id = kefu_mail_thread.channel_id").
			Where("kefu_mailbox.mailbox_id = ?", *filter.MailboxID)
	}
	if filter.ChannelID != nil {
		query = query.Where("kefu_mail_thread.channel_id = ?", *filter.ChannelID)
	}
	if filter.AgentID != nil {
		query = query.Joins("join kefu_session on kefu_session.session_id = kefu_ai_reply.session_id").
			Where("kefu_session.agent_id = ?", *filter.AgentID)
	}
	if err = query.Count(&aiSent).Error; err != nil {
		return 0, 0, err
	}

	usedQuery := db.DB.Table("kefu_ai_reply").
		Joins("join kefu_message on kefu_message.ai_reply_id = kefu_ai_reply.reply_id").
		Where("kefu_ai_reply.deleted = 0 and kefu_ai_reply.channel = ?",
			model.KefuAiReplyChannelMail).
		Where("kefu_ai_reply.send_status = ?", model.KefuAiReplySendSent).
		Where("kefu_ai_reply.update_time >= ? and kefu_ai_reply.update_time <= ?",
			filter.Start, filter.End).
		Where("kefu_message.deleted = 0 and kefu_message.ai_generated = 1")
	if filter.AgentID != nil {
		usedQuery = usedQuery.Joins("join kefu_session on kefu_session.session_id = kefu_ai_reply.session_id").
			Where("kefu_session.agent_id = ?", *filter.AgentID)
	}
	if err = usedQuery.Count(&draftUsed).Error; err != nil {
		return aiSent, 0, err
	}
	return aiSent, draftUsed, nil
}

// MailFirstResponseSeconds 计算平均首次响应时长，单位为秒
func (r *KefuMailAnalysisRepository) MailFirstResponseSeconds(
	filter MailAnalysisFilter,
) (int64, error) {
	query := db.DB.Table("kefu_mail_log").
		Select(`coalesce(avg(extract(epoch from (outbound.first_out - inbound.first_in))), 0)`).
		Joins(`join (
			select session_id, min(sent_time) as first_in from kefu_mail_log
			where deleted = 0 and direction = 'in' and status = 'received'
			group by session_id
		) inbound on inbound.session_id = kefu_mail_log.session_id`).
		Joins(`join (
			select session_id, min(sent_time) as first_out from kefu_mail_log
			where deleted = 0 and direction = 'out' and status = 'sent'
			group by session_id
		) outbound on outbound.session_id = kefu_mail_log.session_id`).
		Where("kefu_mail_log.deleted = 0").
		Where("kefu_mail_log.sent_time >= ? and kefu_mail_log.sent_time <= ?",
			filter.Start, filter.End)
	if filter.ChannelID != nil {
		query = query.Where("kefu_mail_log.channel_id = ?", *filter.ChannelID)
	}
	if filter.MailboxID != nil {
		query = query.Where("kefu_mail_log.mailbox_id = ?", *filter.MailboxID)
	}
	if filter.AgentID != nil {
		query = query.Joins("join kefu_session on kefu_session.session_id = kefu_mail_log.session_id").
			Where("kefu_session.agent_id = ?", *filter.AgentID)
	}
	var seconds float64
	if err := query.Scan(&seconds).Error; err != nil {
		return 0, err
	}
	return int64(seconds), nil
}

// MailSessionAvgSeconds 计算邮件会话平均时长，单位为秒
func (r *KefuMailAnalysisRepository) MailSessionAvgSeconds(
	filter MailAnalysisFilter,
) (int64, error) {
	query := db.DB.Table("kefu_session").
		Select("coalesce(avg(extract(epoch from (kefu_session.end_time - kefu_session.create_time))), 0)").
		Joins("join kefu_channel on kefu_channel.channel_code = kefu_session.channel_code").
		Where("kefu_session.deleted = 0 and kefu_channel.channel_type = ?",
			model.KefuChannelTypeEmail).
		Where("kefu_session.status = ?", model.KefuSessionStatusClosed).
		Where("kefu_session.create_time >= ? and kefu_session.create_time <= ?",
			filter.Start, filter.End)
	if filter.ChannelID != nil {
		query = query.Where("kefu_channel.channel_id = ?", *filter.ChannelID)
	}
	if filter.AgentID != nil {
		query = query.Where("kefu_session.agent_id = ?", *filter.AgentID)
	}
	var seconds float64
	if err := query.Scan(&seconds).Error; err != nil {
		return 0, err
	}
	return int64(seconds), nil
}
