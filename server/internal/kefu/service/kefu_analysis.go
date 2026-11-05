package service

import (
	"math"
	"strconv"
	"strings"
	"time"

	"admin/server/internal/kefu/repository"
	"admin/server/model"
)

// 分析页使用到的常量
const (
	// analysisDefaultDays 是未指定时间区间时默认统计的天数
	analysisDefaultDays = 7
	// analysisDateLayout 是趋势数据的日期格式
	analysisDateLayout = "2006-01-02"
	// analysisRankLimit 是客服排行的返回条数
	analysisRankLimit = 10
)

// KefuAnalysisService 是分析页的业务处理：
// 组装管理员视角与客服视角的统计指标
type KefuAnalysisService struct {
	// 分析页的聚合查询
	analysisRepository *repository.KefuAnalysisRepository
	// 会话的业务处理，复用客服范围的判断
	sessionService *KefuSessionService
	// 客服档案的数据访问，回填排行里的昵称
	agentRepository *repository.KefuAgentRepository
}

// NewKefuAnalysisService 创建分析页的业务处理
func NewKefuAnalysisService() *KefuAnalysisService {
	return &KefuAnalysisService{
		analysisRepository: repository.NewKefuAnalysisRepository(),
		sessionService:     NewKefuSessionService(),
		agentRepository:    repository.NewKefuAgentRepository(),
	}
}

// Overview 组装管理员视角的统计数据
func (s *KefuAnalysisService) Overview(
	param model.KefuAnalysisParam,
) (*model.KefuAnalysisResult, error) {
	param = normalizeAnalysisParam(param)
	overview, err := s.overview(param)
	if err != nil {
		return nil, err
	}

	result := &model.KefuAnalysisResult{Overview: overview}
	if err := s.fillCommon(0, param, result); err != nil {
		return nil, err
	}
	return result, nil
}

// AgentView 组装客服视角的统计数据；
// 主管与管理员按全量返回（与工作台口径一致），非客服账号返回 ErrNotAgent
func (s *KefuAnalysisService) AgentView(
	userID int64, param model.KefuAnalysisParam,
) (*model.KefuAnalysisResult, error) {
	agentID, err := s.sessionService.scopeAgentID(userID)
	if err != nil {
		return nil, err
	}
	param = normalizeAnalysisParam(param)
	agent, err := s.agentOverview(agentID, param)
	if err != nil {
		return nil, err
	}

	result := &model.KefuAnalysisResult{Agent: agent}
	if err := s.fillCommon(agentID, param, result); err != nil {
		return nil, err
	}
	return result, nil
}

// overview 统计管理员视角的指标，口径为全量数据
func (s *KefuAnalysisService) overview(
	param model.KefuAnalysisParam,
) (*model.KefuAnalysisOverview, error) {
	repo := s.analysisRepository
	result := &model.KefuAnalysisOverview{}
	today := todayParam(param)

	var err error
	if result.SessionTotal, err = repo.CountSessions(0, param, nil); err != nil {
		return nil, err
	}
	if result.SessionToday, err = repo.CountSessions(0, today, nil); err != nil {
		return nil, err
	}
	if result.VisitorTotal, err = repo.CountVisitors(param); err != nil {
		return nil, err
	}
	if result.VisitorToday, err = repo.CountVisitors(today); err != nil {
		return nil, err
	}
	if result.MessageTotal, err = repo.CountMessages(0, param, ""); err != nil {
		return nil, err
	}
	if result.HumanSessionCount, err = repo.CountAssignedSessions(0, param); err != nil {
		return nil, err
	}
	if result.AiReplyCount, err = repo.CountAiReplies(0, param, nil); err != nil {
		return nil, err
	}
	if result.TransferCount, err = repo.CountTransfers(0, param); err != nil {
		return nil, err
	}
	if result.AvgFirstResponseSeconds, err = repo.AvgFirstResponse(0, param); err != nil {
		return nil, err
	}
	if result.AvgSessionMinutes, err = repo.AvgSessionMinutes(0, param); err != nil {
		return nil, err
	}
	if result.KnowledgeHitCount, err = repo.CountKnowledgeHits(0, param); err != nil {
		return nil, err
	}
	if result.TokenTotal, err = repo.SumAiTokens(0, param); err != nil {
		return nil, err
	}

	// AI 成功率按正常完成的回复数占全部回复数计算
	var successCount int64
	if successCount, err = repo.CountAiReplies(0, param,
		[]string{model.KefuAiReplyStatusFinished}); err != nil {
		return nil, err
	}
	result.AiSuccessRate = percent(successCount, result.AiReplyCount)
	result.TransferRate = percent(result.TransferCount, result.SessionTotal)

	// 满意度只统计已结束会话，评价率 = 已评价数 / 已结束会话数
	if err := s.fillRate(0, param, &result.RateCount,
		&result.RateScoreAvg, &result.RatePercent); err != nil {
		return nil, err
	}

	// 当前进行中的会话与未读不受时间区间影响
	if result.QueueCount, err = repo.CountCurrent(0, model.KefuSessionStatusQueueing); err != nil {
		return nil, err
	}
	if result.AiCount, err = repo.CountCurrent(0, model.KefuSessionStatusAi); err != nil {
		return nil, err
	}
	if result.HumanCount, err = repo.CountCurrent(0, model.KefuSessionStatusHuman); err != nil {
		return nil, err
	}
	if result.UnreadCount, err = repo.SumUnread(0); err != nil {
		return nil, err
	}
	return result, nil
}

// agentOverview 统计客服视角的指标，agentID 为 0 表示主管按全量统计
func (s *KefuAnalysisService) agentOverview(
	agentID int64, param model.KefuAnalysisParam,
) (*model.KefuAnalysisAgent, error) {
	repo := s.analysisRepository
	result := &model.KefuAnalysisAgent{}
	today := todayParam(param)

	var err error
	if result.MySessionTotal, err = repo.CountSessions(agentID, param, nil); err != nil {
		return nil, err
	}
	if result.MySessionToday, err = repo.CountSessions(agentID, today, nil); err != nil {
		return nil, err
	}
	if result.MyMessageCount, err = repo.CountMessages(agentID, param, ""); err != nil {
		return nil, err
	}
	if result.MyAvgFirstResponseSeconds, err = repo.AvgFirstResponse(agentID, param); err != nil {
		return nil, err
	}

	if result.MyQueueCount, err = repo.CountCurrent(agentID, model.KefuSessionStatusQueueing); err != nil {
		return nil, err
	}
	if result.MyAiCount, err = repo.CountCurrent(agentID, model.KefuSessionStatusAi); err != nil {
		return nil, err
	}
	if result.MyHumanCount, err = repo.CountCurrent(agentID, model.KefuSessionStatusHuman); err != nil {
		return nil, err
	}
	if err := s.fillRate(agentID, param, &result.MyRateCount,
		&result.MyRateScoreAvg, &result.MyRatePercent); err != nil {
		return nil, err
	}
	return result, nil
}

// fillRate 统计满意度：已评价会话数、均分与评价率，
// 分母取同一区间内的已结束会话数，没有评价时均分为 0
func (s *KefuAnalysisService) fillRate(
	agentID int64, param model.KefuAnalysisParam,
	count *int64, scoreAvg, ratePercent *float64,
) error {
	rated, avg, err := s.analysisRepository.RateStat(agentID, param)
	if err != nil {
		return err
	}
	closed, err := s.analysisRepository.CountSessions(agentID, param,
		[]int{model.KefuSessionStatusClosed})
	if err != nil {
		return err
	}
	*count = rated
	*scoreAvg = math.Round(avg*100) / 100
	*ratePercent = percent(rated, closed)
	return nil
}

// fillCommon 填充两个视角共用的趋势、分布与排行
func (s *KefuAnalysisService) fillCommon(
	agentID int64, param model.KefuAnalysisParam, result *model.KefuAnalysisResult,
) error {
	trend, err := s.trend(agentID, param)
	if err != nil {
		return err
	}
	result.Trend = trend

	channels, err := s.analysisRepository.CountSessionsByChannel(agentID, param)
	if err != nil {
		return err
	}
	result.ChannelDistribution = s.channelDistribution(channels)

	statuses, err := s.analysisRepository.CountSessionsByStatus(agentID, param)
	if err != nil {
		return err
	}
	result.StatusDistribution = s.statusDistribution(statuses)

	ranking, err := s.agentRanking(param)
	if err != nil {
		return err
	}
	result.AgentRanking = ranking
	return nil
}

// trend 组装按天补齐的趋势数据，缺口日期补 0，避免图表断裂
func (s *KefuAnalysisService) trend(
	agentID int64, param model.KefuAnalysisParam,
) ([]model.KefuAnalysisTrend, error) {
	repo := s.analysisRepository
	sessions, err := repo.CountSessionsByDay(agentID, param)
	if err != nil {
		return nil, err
	}
	messages, err := repo.CountMessagesByDay(agentID, param)
	if err != nil {
		return nil, err
	}
	aiSessions, err := repo.CountAiSessionsByDay(agentID, param)
	if err != nil {
		return nil, err
	}
	humanSessions, err := repo.CountHumanSessionsByDay(agentID, param)
	if err != nil {
		return nil, err
	}

	start, end := analysisRange(param)
	list := make([]model.KefuAnalysisTrend, 0, analysisDays(start, end))
	for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
		key := day.Format(analysisDateLayout)
		list = append(list, model.KefuAnalysisTrend{
			Date:         key,
			SessionCount: sessions[key],
			MessageCount: messages[key],
			AiCount:      aiSessions[key],
			HumanCount:   humanSessions[key],
		})
	}
	return list, nil
}

// channelDistribution 把渠道分组结果补上渠道名称
func (s *KefuAnalysisService) channelDistribution(
	list []model.KefuAnalysisDistributionItem,
) []model.KefuAnalysisDistributionItem {
	if len(list) == 0 {
		return []model.KefuAnalysisDistributionItem{}
	}

	names, err := s.analysisRepository.ListChannelNames()
	if err != nil {
		names = nil
	}
	for i := range list {
		list[i].Name = list[i].Key
		if name, ok := names[list[i].Key]; ok && strings.TrimSpace(name) != "" {
			list[i].Name = name
		}
	}
	return list
}

// statusDistribution 把会话状态分组结果补上状态名称，并按固定顺序补齐全部状态
func (s *KefuAnalysisService) statusDistribution(
	list []model.KefuAnalysisDistributionItem,
) []model.KefuAnalysisDistributionItem {
	counts := make(map[string]int64, len(list))
	for _, item := range list {
		counts[item.Key] = item.Count
	}

	names := sessionStatusNames()
	statuses := []int{
		model.KefuSessionStatusQueueing,
		model.KefuSessionStatusAi,
		model.KefuSessionStatusClosed,
		model.KefuSessionStatusLeaveMessage,
		model.KefuSessionStatusHuman,
	}
	result := make([]model.KefuAnalysisDistributionItem, 0, len(statuses))
	for _, status := range statuses {
		key := strconv.Itoa(status)
		result = append(result, model.KefuAnalysisDistributionItem{
			Key:   key,
			Name:  names[key],
			Count: counts[key],
		})
	}
	return result
}

// agentRanking 组装客服排行，回填昵称
func (s *KefuAnalysisService) agentRanking(
	param model.KefuAnalysisParam,
) ([]model.KefuAnalysisRankItem, error) {
	list, err := s.analysisRepository.RankAgents(param, analysisRankLimit)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return []model.KefuAnalysisRankItem{}, nil
	}

	agentIDs := make([]int64, 0, len(list))
	for _, item := range list {
		agentIDs = append(agentIDs, item.AgentID)
	}

	names := s.agentNicknames(agentIDs)
	for i := range list {
		list[i].Nickname = names[list[i].AgentID]
	}
	return list, nil
}

// agentNicknames 批量查询客服的展示昵称，取不到昵称时用工号
func (s *KefuAnalysisService) agentNicknames(agentIDs []int64) map[int64]string {
	result := make(map[int64]string, len(agentIDs))
	agents, err := s.agentRepository.ListByIDs(agentIDs)
	if err != nil || len(agents) == 0 {
		return result
	}

	userIDs := make([]int64, 0, len(agents))
	agentUsers := make(map[int64]int64, len(agents))
	for _, agent := range agents {
		userIDs = append(userIDs, agent.UserID)
		agentUsers[agent.AgentID] = agent.UserID
		if no := strings.TrimSpace(agent.AgentNo); no != "" {
			result[agent.AgentID] = no
		}
	}
	users, err := s.agentRepository.ListUsersByIDs(userIDs)
	if err != nil {
		return result
	}
	userNames := make(map[int64]string, len(users))
	for _, user := range users {
		if nickname := strings.TrimSpace(user.Nickname); nickname != "" {
			userNames[user.UserID] = nickname
		} else if username := strings.TrimSpace(user.Username); username != "" {
			userNames[user.UserID] = username
		}
	}
	for agentID, userID := range agentUsers {
		if name, ok := userNames[userID]; ok {
			result[agentID] = name
		}
	}
	return result
}

// normalizeAnalysisParam 补齐时间区间：缺省最近 7 天，格式统一并按时间正序
func normalizeAnalysisParam(param model.KefuAnalysisParam) model.KefuAnalysisParam {
	end := parseAnalysisTime(param.EndTime)
	start := parseAnalysisTime(param.StartTime)
	if end.IsZero() {
		end = time.Now()
	}
	if start.IsZero() {
		start = truncateDay(end).AddDate(0, 0, -analysisDefaultDays+1)
	}
	if start.After(end) {
		start, end = end, start
	}
	param.StartTime = start.Format(model.TimeLayout)
	param.EndTime = end.Format(model.TimeLayout)
	return param
}

// analysisRange 解析已补齐的查询区间，用于按天补齐趋势
func analysisRange(param model.KefuAnalysisParam) (time.Time, time.Time) {
	start, _ := time.ParseInLocation(model.TimeLayout, param.StartTime, time.Local)
	end, _ := time.ParseInLocation(model.TimeLayout, param.EndTime, time.Local)
	return truncateDay(start), truncateDay(end)
}

// analysisDays 返回区间内需要补齐的天数
func analysisDays(start, end time.Time) int {
	days := int(end.Sub(start).Hours()/24) + 1
	if days < 1 {
		return 1
	}
	return days
}

// todayParam 返回「今日」的时间区间，今日指查询区间结束时间所在的那一天
func todayParam(param model.KefuAnalysisParam) model.KefuAnalysisParam {
	end, err := time.ParseInLocation(model.TimeLayout, param.EndTime, time.Local)
	if err != nil {
		end = time.Now()
	}
	day := truncateDay(end)
	return model.KefuAnalysisParam{
		StartTime: day.Format(model.TimeLayout),
		EndTime:   day.AddDate(0, 0, 1).Add(-time.Second).Format(model.TimeLayout),
	}
}

// parseAnalysisTime 解析时间文本，解析失败返回零值
func parseAnalysisTime(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	parsed, err := time.ParseInLocation(model.TimeLayout, value, time.Local)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

// truncateDay 截断到当天零点
func truncateDay(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(),
		0, 0, 0, 0, value.Location())
}

// sessionStatusNames 返回会话状态标识到名称的映射，与迁移种子里的字典一致
func sessionStatusNames() map[string]string {
	return map[string]string{
		"0": "待接待",
		"1": "AI 进行中",
		"2": "已结束",
		"3": "留言",
		"4": "人工进行中",
	}
}

// percent 计算占比，返回百分比且保留两位小数，分母为 0 时返回 0
func percent(part, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return math.Round(float64(part)*10000/float64(total)) / 100
}
