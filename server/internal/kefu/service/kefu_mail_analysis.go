package service

import (
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"admin/server/internal/kefu/repository"
	"admin/server/model"
	"admin/server/pkg/log"
)

// KefuMailAnalysisService 是邮件统计的业务处理
type KefuMailAnalysisService struct {
	// 邮件统计的数据访问
	analysisRepository *repository.KefuMailAnalysisRepository
	// 邮箱配置的数据访问，分组回填邮箱地址
	mailboxRepository *repository.KefuMailboxRepository
	// 客服档案的数据访问，分组回填客服名称
	agentRepository *repository.KefuAgentRepository
}

// NewKefuMailAnalysisService 创建邮件统计的业务处理
func NewKefuMailAnalysisService() *KefuMailAnalysisService {
	return &KefuMailAnalysisService{
		analysisRepository: repository.NewKefuMailAnalysisRepository(),
		mailboxRepository:  repository.NewKefuMailboxRepository(),
		agentRepository:    repository.NewKefuAgentRepository(),
	}
}

// Overview 返回邮件统计的汇总与分组明细，无数据时返回 0 与空列表
func (s *KefuMailAnalysisService) Overview(
	param model.KefuMailAnalysisParam,
) (*model.KefuMailAnalysisResult, error) {
	start, end := mailAnalysisRange(param.StartTime, param.EndTime)
	filter := repository.MailAnalysisFilter{
		Start:     start,
		End:       end,
		ChannelID: param.ChannelID,
		MailboxID: param.MailboxID,
		AgentID:   param.AgentID,
		GroupBy:   strings.TrimSpace(param.GroupBy),
	}

	result := &model.KefuMailAnalysisResult{Items: []model.KefuMailAnalysisItem{}}
	summaryList, err := s.analysisRepository.MailLogStats(repository.MailAnalysisFilter{
		Start: start, End: end, ChannelID: param.ChannelID,
		MailboxID: param.MailboxID, AgentID: param.AgentID,
	})
	if err != nil {
		return nil, err
	}
	if len(summaryList) > 0 {
		result.Summary = summaryList[0]
	}
	aiSent, draftUsed, err := s.analysisRepository.MailAiStats(filter)
	if err != nil {
		log.Logger.Warn("query kefu mail ai stats failed", zap.Error(err))
	}
	result.Summary.AiSentCount = aiSent
	result.Summary.AiDraftUsedCount = draftUsed
	if seconds, err := s.analysisRepository.MailFirstResponseSeconds(filter); err == nil {
		result.Summary.AvgFirstResponseSeconds = seconds
	}
	if seconds, err := s.analysisRepository.MailSessionAvgSeconds(filter); err == nil {
		result.Summary.AvgSessionSeconds = seconds
	}

	if filter.GroupBy == "" {
		return result, nil
	}
	items, err := s.analysisRepository.MailLogStats(filter)
	if err != nil {
		return nil, err
	}
	s.fillLabels(items, filter.GroupBy)
	result.Items = items
	return result, nil
}

// fillLabels 按分组维度把 id 换成可读名称，取不到时保留原值
func (s *KefuMailAnalysisService) fillLabels(
	items []model.KefuMailAnalysisItem, groupBy string,
) {
	switch groupBy {
	case "mailbox":
		for i := range items {
			mailboxID, err := strconv.ParseInt(items[i].Label, 10, 64)
			if err != nil {
				continue
			}
			mailbox, err := s.mailboxRepository.GetByID(mailboxID)
			if err != nil {
				continue
			}
			items[i].Label = mailbox.EmailAddress
		}
	case "agent":
		userIDs := make([]int64, 0, len(items))
		agentIDs := make([]int64, 0, len(items))
		agentUserMap := make(map[int64]int64, len(items))
		for i := range items {
			agentID, err := strconv.ParseInt(items[i].Label, 10, 64)
			if err != nil || agentID <= 0 {
				items[i].Label = "未分配"
				continue
			}
			agentIDs = append(agentIDs, agentID)
		}
		agents, err := s.agentRepository.ListByIDs(agentIDs)
		if err != nil {
			log.Logger.Warn("load kefu agents failed", zap.Error(err))
			return
		}
		for _, agent := range agents {
			agentUserMap[agent.AgentID] = agent.UserID
			userIDs = append(userIDs, agent.UserID)
		}
		users, err := s.agentRepository.ListUsersByIDs(userIDs)
		if err != nil {
			return
		}
		userMap := make(map[int64]model.SysUser, len(users))
		for _, user := range users {
			userMap[user.UserID] = user
		}
		for i := range items {
			agentID, err := strconv.ParseInt(items[i].Label, 10, 64)
			if err != nil || agentID <= 0 {
				continue
			}
			userID := agentUserMap[agentID]
			if user, ok := userMap[userID]; ok {
				if strings.TrimSpace(user.Nickname) != "" {
					items[i].Label = user.Nickname
				} else {
					items[i].Label = user.Username
				}
			}
		}
	}
}

// mailAnalysisRange 解析统计时间范围，为空时默认最近 7 天
func mailAnalysisRange(startValue, endValue string) (model.Time, model.Time) {
	end := time.Now()
	if value := strings.TrimSpace(endValue); value != "" {
		if parsed, err := time.ParseInLocation(model.TimeLayout, value, time.Local); err == nil {
			end = parsed
		}
	}
	start := end.AddDate(0, 0, -7)
	if value := strings.TrimSpace(startValue); value != "" {
		if parsed, err := time.ParseInLocation(model.TimeLayout, value, time.Local); err == nil {
			start = parsed
		}
	}
	return model.Time{Time: start}, model.Time{Time: end}
}
