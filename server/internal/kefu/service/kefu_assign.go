package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"go.uber.org/zap"

	"admin/server/internal/kefu/im"
	"admin/server/internal/kefu/repository"
	"admin/server/model"
	"admin/server/pkg/log"
	"admin/server/pkg/setting"
)

// 分配与待接待使用到的配置项
const (
	// configKeyAssignQueueTimeout 是待接待超时的分钟数
	configKeyAssignQueueTimeout = "kefu.assign.queue_timeout_minutes"
	// configKeyAssignMaxTransferPerDay 是同一会话 24 小时内的转接上限
	configKeyAssignMaxTransferPerDay = "kefu.assign.max_transfer_per_day"
	// configKeyAssignAvgWaitSeconds 是排队预计等待的人平秒数，只用于展示
	configKeyAssignAvgWaitSeconds = "kefu.assign.avg_wait_seconds"
)

// 分配与待接待使用到的常量
const (
	// queueTimeoutScanInterval 是待接待超时扫描的间隔
	queueTimeoutScanInterval = time.Minute
	// queueTimeoutBatch 是单次扫描处理的会话上限
	queueTimeoutBatch = 200
	// defaultAgentMaxConcurrent 是客服未配置并发上限时的默认值
	defaultAgentMaxConcurrent = 5
	// defaultAssignAvgWaitSeconds 是未配置人平等待时长时的默认值，单位秒
	defaultAssignAvgWaitSeconds = 60
	// leaveMessageText 是待接待超时转留言时给客户的话术
	leaveMessageText = "当前客服繁忙，请留下您的联系方式，我们会尽快与您联系。"
)

// 分配相关的业务错误
var (
	// ErrNoAgentAvailable 是当前没有可接待的客服
	ErrNoAgentAvailable = errors.New("当前没有可接待的客服")
)

// KefuAssignService 是客服挑选与待接待超时的业务处理：
// 会话不再自动分配，转人工后进入待接待，由客服在工作台领取
type KefuAssignService struct {
	// 会话的数据访问
	sessionRepository *repository.KefuSessionRepository
	// 客服档案的数据访问
	agentRepository *repository.KefuAgentRepository
	// 转接记录的数据访问，挑选客服时比较最近分配时间
	transferRepository *repository.KefuSessionTransferRepository
	// 会话消息的数据访问，写频道内的系统消息
	messageRepository *repository.KefuMessageRepository
	// 客服在线状态的业务处理
	onlineService *KefuOnlineService
}

// NewKefuAssignService 创建客服挑选与待接待超时的业务处理
func NewKefuAssignService() *KefuAssignService {
	return &KefuAssignService{
		sessionRepository:  repository.NewKefuSessionRepository(),
		agentRepository:    repository.NewKefuAgentRepository(),
		transferRepository: repository.NewKefuSessionTransferRepository(),
		messageRepository:  repository.NewKefuMessageRepository(),
		onlineService:      NewKefuOnlineService(),
	}
}

// PickAgent 挑选一名客服：在线、未满载，按会话数升序、
// 最近分配时间升序、客服id升序取第一个；没有可用客服时返回 nil
func (s *KefuAssignService) PickAgent(
	ctx context.Context,
) (*model.KefuAgent, error) {
	list, err := s.agentRepository.ListCandidates()
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, nil
	}

	agentIDs := make([]int64, 0, len(list))
	userIDs := make([]int64, 0, len(list))
	for _, agent := range list {
		agentIDs = append(agentIDs, agent.AgentID)
		userIDs = append(userIDs, agent.UserID)
	}

	loads, err := s.sessionRepository.CountActiveByAgents(agentIDs)
	if err != nil {
		return nil, err
	}
	lastAssign, err := s.transferRepository.LastTransferTimes(agentIDs)
	if err != nil {
		return nil, err
	}
	online := s.onlineService.OnlineMap(ctx, userIDs)

	available := make([]*model.KefuAgent, 0, len(list))
	for i := range list {
		agent := &list[i]
		if !online[agent.UserID] {
			continue
		}
		if loads[agent.AgentID] >= int64(maxConcurrentOf(agent)) {
			continue
		}
		available = append(available, agent)
	}
	if len(available) == 0 {
		return nil, nil
	}

	sortAgentsForAssign(available, loads, lastAssign)
	return available[0], nil
}

// OnlineAgents 返回在线且可接单的客服，转人工通知用它群发
func (s *KefuAssignService) OnlineAgents(ctx context.Context) []model.KefuAgent {
	list, err := s.agentRepository.ListCandidates()
	if err != nil {
		log.Logger.Warn("list kefu agent candidates failed", zap.Error(err))
		return nil
	}
	if len(list) == 0 {
		return nil
	}

	userIDs := make([]int64, 0, len(list))
	for _, agent := range list {
		userIDs = append(userIDs, agent.UserID)
	}
	online := s.onlineService.OnlineMap(ctx, userIDs)

	result := make([]model.KefuAgent, 0, len(list))
	for _, agent := range list {
		if online[agent.UserID] {
			result = append(result, agent)
		}
	}
	return result
}

// HasOnlineAgent 判断是否有客服在线，用于转人工时回复不同的提示
func (s *KefuAssignService) HasOnlineAgent(ctx context.Context) bool {
	return len(s.OnlineAgents(ctx)) > 0
}

// StartQueueTimeoutTask 启动待接待超时扫描任务，进程内每分钟检查一次
func (s *KefuAssignService) StartQueueTimeoutTask() {
	go func() {
		ticker := time.NewTicker(queueTimeoutScanInterval)
		defer ticker.Stop()
		for range ticker.C {
			s.expireQueue(context.Background())
		}
	}()
	log.Kernel.Info("start kefu queue timeout task")
}

// expireQueue 把待接待超时的会话转成留言态，并给客户发一句留联系方式的话术
func (s *KefuAssignService) expireQueue(ctx context.Context) {
	minutes := setting.GetInt(configKeyAssignQueueTimeout, 10)
	if minutes <= 0 {
		minutes = 10
	}
	deadline := time.Now().Add(-time.Duration(minutes) * time.Minute)
	before := model.Time{Time: deadline}

	list, err := s.sessionRepository.ListQueueTimeout(before, queueTimeoutBatch)
	if err != nil {
		log.Logger.Error("list kefu queue timeout sessions failed", zap.Error(err))
		return
	}

	for _, session := range list {
		if err := s.sessionRepository.UpdateFields(session.SessionID, map[string]any{
			"status": model.KefuSessionStatusLeaveMessage,
		}); err != nil {
			log.Logger.Warn("mark kefu session leave message failed",
				zap.Int64("sessionId", session.SessionID), zap.Error(err))
			continue
		}
		if err := s.sendSystemMessage(ctx, session.SessionID, leaveMessageText); err != nil {
			log.Logger.Warn("send kefu leave message failed",
				zap.Int64("sessionId", session.SessionID), zap.Error(err))
		}
	}
}

// maxConcurrentOf 返回客服的最大并发接待数，未配置时用默认值
func maxConcurrentOf(agent *model.KefuAgent) int {
	if agent == nil || agent.MaxConcurrent <= 0 {
		return defaultAgentMaxConcurrent
	}
	return agent.MaxConcurrent
}

// sortAgentsForAssign 按人工进行中会话数升序、最近分配时间升序、客服id升序排序
func sortAgentsForAssign(
	agents []*model.KefuAgent, loads map[int64]int64, lastAssign map[int64]model.Time,
) {
	sort.SliceStable(agents, func(i, j int) bool {
		left, right := agents[i], agents[j]
		leftLoad, rightLoad := loads[left.AgentID], loads[right.AgentID]
		if leftLoad != rightLoad {
			return leftLoad < rightLoad
		}
		leftTime, rightTime := lastAssign[left.AgentID], lastAssign[right.AgentID]
		if !leftTime.Equal(rightTime.Time) {
			return leftTime.Time.Before(rightTime.Time)
		}
		return left.AgentID < right.AgentID
	})
}

// sendSystemMessage 往会话频道发一条系统消息并落库，
// 转人工提示、留言话术与转接提示都用它，IM 不可用时返回错误由调用方记日志
func (s *KefuAssignService) sendSystemMessage(
	ctx context.Context, sessionID int64, content string,
) error {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}

	now := model.Now()
	message := &model.KefuMessage{
		SessionID:   sessionID,
		ClientMsgNo: generateClientMsgNo("system"),
		FromRole:    model.KefuMessageFromSystem,
		FromID:      "system",
		ContentType: model.KefuMessageTypeSystem,
		Content:     content,
		CreateTime:  now,
		UpdateTime:  now,
	}
	if err := s.messageRepository.Create(message); err != nil {
		return err
	}

	payload := &im.Payload{
		Version:   1,
		Type:      im.PayloadTypeSystem,
		Role:      im.PayloadRoleSystem,
		Content:   content,
		MessageID: message.MessageID,
		SessionID: sessionID,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	message.Payload = string(raw)
	if err := s.messageRepository.UpdateContent(message.MessageID,
		map[string]any{"payload": message.Payload}); err != nil {
		log.Logger.Warn("update kefu system message payload failed",
			zap.Int64("messageId", message.MessageID), zap.Error(err))
	}

	resp, err := im.Default.SendMessage(ctx, &im.MessageSendReq{
		Header:      im.MessageHeader{RedDot: 1},
		ClientMsgNo: message.ClientMsgNo,
		FromUID:     im.AiUID(),
		ChannelID:   im.ChannelIDOfSession(sessionID),
		ChannelType: im.ChannelTypeGroup,
		Payload:     raw,
	})
	if err != nil {
		return err
	}
	if resp != nil {
		if err := s.messageRepository.UpdateIMInfo(message.MessageID,
			fmt.Sprintf("%d", resp.MessageID), int64(resp.MessageSeq)); err != nil {
			log.Logger.Warn("update kefu system message im info failed",
				zap.Int64("messageId", message.MessageID), zap.Error(err))
		}
	}
	if err := s.sessionRepository.UpdateMessageStat(sessionID, now, 0); err != nil {
		log.Logger.Warn("update kefu session stat failed",
			zap.Int64("sessionId", sessionID), zap.Error(err))
	}
	return nil
}
