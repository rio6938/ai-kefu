package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"admin/server/internal/kefu/im"
	"admin/server/internal/kefu/repository"
	"admin/server/model"
	"admin/server/pkg/log"
	"admin/server/pkg/setting"
)

// 转接使用到的常量
const (
	// transferKeywordDictCode 是转接关键词字典标识
	transferKeywordDictCode = "kefu_transfer_keyword"
	// transferKeywordScanLimit 是建议转接时回看的最近消息条数
	transferKeywordScanLimit = 20
	// transferNoticeTitle 是转接通知的站内消息标题
	transferNoticeTitle = "会话转接提醒"
)

// 转接相关的业务错误
var (
	// ErrTransferReasonRequired 是转接原因不能为空
	ErrTransferReasonRequired = errors.New("请填写转接原因")
	// ErrTransferTargetInvalid 是转接目标不可用
	ErrTransferTargetInvalid = errors.New("转接目标不可用")
	// ErrTransferSelf 是转接目标就是当前客服
	ErrTransferSelf = errors.New("不能转接给当前接待的客服")
	// ErrTransferLimitReached 是 24 小时内的转接次数达到上限
	ErrTransferLimitReached = errors.New("该会话 24 小时内的转接次数已达上限")
)

// KefuTransferService 是会话转接的业务处理：
// 手动转接、规则建议转接与客服停用后的批量转出都走这里
type KefuTransferService struct {
	// 会话的数据访问
	sessionRepository *repository.KefuSessionRepository
	// 客服档案的数据访问
	agentRepository *repository.KefuAgentRepository
	// 转接记录的数据访问
	transferRepository *repository.KefuSessionTransferRepository
	// 站内消息的数据访问
	noticeRepository *repository.KefuNoticeRepository
	// 会话消息的数据访问，建议转接需要回看客户消息
	messageRepository *repository.KefuMessageRepository
	// 字典的数据访问，读取转接关键词
	dictionaryRepository *repository.KefuDictionaryRepository
	// 分配的业务处理，停用转出时复用分配算法
	assignService *KefuAssignService
}

// NewKefuTransferService 创建会话转接的业务处理
func NewKefuTransferService() *KefuTransferService {
	return &KefuTransferService{
		sessionRepository:    repository.NewKefuSessionRepository(),
		agentRepository:      repository.NewKefuAgentRepository(),
		transferRepository:   repository.NewKefuSessionTransferRepository(),
		noticeRepository:     repository.NewKefuNoticeRepository(),
		messageRepository:    repository.NewKefuMessageRepository(),
		dictionaryRepository: repository.NewKefuDictionaryRepository(),
		assignService:        NewKefuAssignService(),
	}
}

// Transfer 手动转接会话：校验目标可用与转接次数上限后换绑客服，
// 会话本身不重建，历史消息与 IM 频道保持不变
func (s *KefuTransferService) Transfer(
	ctx context.Context, userID int64, form model.KefuSessionTransferForm,
) error {
	session, err := s.operable(userID, form.SessionID)
	if err != nil {
		return err
	}
	if session.Status != model.KefuSessionStatusHuman {
		return ErrSessionStatusInvalid
	}

	reason := strings.TrimSpace(form.Reason)
	if reason == "" {
		return ErrTransferReasonRequired
	}

	target, err := s.resolveTarget(ctx, session, form.ToAgentID)
	if err != nil {
		return err
	}
	if target.AgentID == session.AgentID {
		return ErrTransferSelf
	}

	if err := s.checkTransferLimit(session.SessionID); err != nil {
		return err
	}
	return s.move(ctx, session, target,
		model.KefuTransferTypeManual, reason, userID)
}

// TransferOut 客服停用或停止接单后批量转出其人工进行中的会话，
// 找不到可接待的客服时会话回到待接待
func (s *KefuTransferService) TransferOut(
	ctx context.Context, agentID int64, reason string,
) error {
	sessions, err := s.sessionRepository.ListActiveByAgent(agentID)
	if err != nil {
		return err
	}
	for _, session := range sessions {
		target, err := s.assignService.PickAgent(ctx)
		if err != nil {
			return err
		}
		if target != nil && target.AgentID != session.AgentID {
			if err := s.move(ctx, &session, target,
				model.KefuTransferTypeDisable, reason, 0); err != nil {
				log.Logger.Warn("transfer out kefu session failed",
					zap.Int64("sessionId", session.SessionID), zap.Error(err))
			}
			continue
		}

		// 没有可接待的客服，会话回到待接待，等待其它客服领取
		if err := s.sessionRepository.ReturnToQueue(session.SessionID, model.Now()); err != nil {
			log.Logger.Warn("return kefu session to queue failed",
				zap.Int64("sessionId", session.SessionID), zap.Error(err))
		}
	}
	return nil
}

// suggestFor 按转接关键词字典判断是否需要建议转接：
// 客户最近的消息命中关键词时给出建议
func (s *KefuTransferService) suggestFor(
	session *model.KefuSession,
) *model.KefuTransferSuggestion {
	if session == nil || session.Status != model.KefuSessionStatusHuman {
		return nil
	}

	keywords, err := s.dictionaryRepository.ListCodes(transferKeywordDictCode)
	if err != nil {
		log.Logger.Warn("load kefu transfer keywords failed", zap.Error(err))
		return nil
	}
	if len(keywords) == 0 {
		return nil
	}

	messages, err := s.messageRepository.ListRecent(session.SessionID, transferKeywordScanLimit)
	if err != nil {
		log.Logger.Warn("load kefu session messages failed",
			zap.Int64("sessionId", session.SessionID), zap.Error(err))
		return nil
	}

	keyword := ""
	for _, message := range messages {
		if message.FromRole != model.KefuMessageFromVisitor {
			continue
		}
		if hit := matchTransferKeyword(message.Content, keywords); hit != "" {
			keyword = hit
			break
		}
	}
	if keyword == "" {
		return nil
	}

	return &model.KefuTransferSuggestion{
		Keyword: keyword,
		Reason:  fmt.Sprintf("客户提到「%s」，建议转接给其他客服", keyword),
	}
}

// resolveTarget 解析转接目标：指定客服时校验该客服可接待，
// 未指定时按分配算法挑一名可接待的客服
func (s *KefuTransferService) resolveTarget(
	ctx context.Context, session *model.KefuSession, toAgentID int64,
) (*model.KefuAgent, error) {
	if toAgentID > 0 {
		agent, err := s.agentRepository.GetByID(toAgentID)
		if err != nil {
			if errors.Is(err, repository.ErrAgentNotFound) {
				return nil, ErrTransferTargetInvalid
			}
			return nil, err
		}
		// 目标客服必须正常且开启接单，否则转过去也没人接待
		if agent.Status != model.KefuAgentStatusNormal ||
			agent.AcceptStatus != model.KefuAgentAcceptOn {
			return nil, ErrTransferTargetInvalid
		}
		return agent, nil
	}

	agent, err := s.assignService.PickAgent(ctx)
	if err != nil {
		return nil, err
	}
	if agent == nil {
		return nil, ErrNoAgentAvailable
	}
	if agent.AgentID == session.AgentID {
		return nil, ErrTransferSelf
	}
	return agent, nil
}

// move 把会话换绑到目标客服并完成全部收尾：
// 事务内改会话与写转接记录 → IM 频道换成员 → 频道系统消息 → 双方站内消息
func (s *KefuTransferService) move(
	ctx context.Context, session *model.KefuSession, target *model.KefuAgent,
	transferType, reason string, operatorID int64,
) error {
	fromAgentID := session.AgentID
	now := model.Now()

	record := &model.KefuSessionTransfer{
		SessionID:    session.SessionID,
		TransferType: transferType,
		FromAgentID:  fromAgentID,
		ToAgentID:    target.AgentID,
		Reason:       reason,
		OperatorID:   operatorID,
		CreateTime:   now,
		UpdateTime:   now,
	}
	if err := s.transferRepository.TransferSession(session.SessionID, map[string]any{
		"agent_id": target.AgentID,
	}, record); err != nil {
		return err
	}

	s.syncChannelSubscribers(ctx, session.SessionID, fromAgentID, target)

	if err := s.assignService.sendSystemMessage(ctx, session.SessionID,
		fmt.Sprintf("会话已转接至%s", s.agentDisplayName(target))); err != nil {
		log.Logger.Warn("send kefu transfer system message failed",
			zap.Int64("sessionId", session.SessionID), zap.Error(err))
	}

	s.notifyAgents(session, fromAgentID, target, transferType == model.KefuTransferTypeDisable)
	return nil
}

// syncChannelSubscribers 把 IM 频道成员换成新客服，旧客服不再收到该会话消息；
// IM 不可用只记日志，重连时会重新对齐成员
func (s *KefuTransferService) syncChannelSubscribers(
	ctx context.Context, sessionID int64, fromAgentID int64, target *model.KefuAgent,
) {
	channelID := im.ChannelIDOfSession(sessionID)
	if fromAgentID > 0 && fromAgentID != target.AgentID {
		if oldAgent, err := s.agentRepository.GetByID(fromAgentID); err == nil {
			if err := im.Default.RemoveSubscribers(ctx, channelID, im.ChannelTypeGroup,
				[]string{agentUID(oldAgent.UserID)}); err != nil {
				log.Logger.Warn("remove kefu session subscriber failed",
					zap.Int64("sessionId", sessionID), zap.Error(err))
			}
		}
	}
	if err := im.Default.AddSubscribers(ctx, channelID, im.ChannelTypeGroup,
		[]string{agentUID(target.UserID)}); err != nil {
		log.Logger.Warn("add kefu session subscriber failed",
			zap.Int64("sessionId", sessionID), zap.Error(err))
	}
}

// notifyAgents 给转出与转入的客服发站内消息，停用转出时只通知新客服
func (s *KefuTransferService) notifyAgents(
	session *model.KefuSession, fromAgentID int64, target *model.KefuAgent, disabled bool,
) {
	if !disabled && fromAgentID > 0 && fromAgentID != target.AgentID {
		if oldAgent, err := s.agentRepository.GetByID(fromAgentID); err == nil {
			if err := s.noticeRepository.Send([]int64{oldAgent.UserID}, transferNoticeTitle,
				fmt.Sprintf("会话 %s 已转接给其他客服", session.SessionNo)); err != nil {
				log.Logger.Warn("send kefu transfer notice failed",
					zap.Int64("agentId", oldAgent.AgentID), zap.Error(err))
			}
		}
	}
	if err := s.noticeRepository.Send([]int64{target.UserID}, transferNoticeTitle,
		fmt.Sprintf("会话 %s 已转接给您，请及时接待", session.SessionNo)); err != nil {
		log.Logger.Warn("send kefu transfer notice failed",
			zap.Int64("agentId", target.AgentID), zap.Error(err))
	}
}

// checkTransferLimit 校验同一会话 24 小时内的转接次数是否超限
func (s *KefuTransferService) checkTransferLimit(sessionID int64) error {
	limit := setting.GetInt(configKeyAssignMaxTransferPerDay, 3)
	if limit <= 0 {
		return nil
	}
	since := model.Time{Time: time.Now().Add(-24 * time.Hour)}
	count, err := s.transferRepository.CountSince(sessionID, since)
	if err != nil {
		return err
	}
	if count >= int64(limit) {
		return ErrTransferLimitReached
	}
	return nil
}

// operable 查询会话并校验当前用户能否操作：
// 主管可以操作全部会话，普通客服只能操作自己接待的会话
func (s *KefuTransferService) operable(
	userID int64, sessionID int64,
) (*model.KefuSession, error) {
	session, err := s.sessionRepository.GetByID(sessionID)
	if err != nil {
		return nil, err
	}
	supervisor, err := s.agentRepository.HasRoleCode(userID,
		[]string{model.KefuRoleSupervisor, model.RoleCodeAdmin})
	if err != nil {
		return nil, err
	}
	if supervisor {
		return session, nil
	}
	agent, err := s.agentRepository.GetByUserID(userID)
	if err != nil {
		if errors.Is(err, repository.ErrAgentNotFound) {
			return nil, ErrNotAgent
		}
		return nil, err
	}
	if session.AgentID != agent.AgentID {
		return nil, ErrSessionNoPermission
	}
	return session, nil
}

// agentDisplayName 返回客服的展示名称，取不到昵称时用工号或账号
func (s *KefuTransferService) agentDisplayName(agent *model.KefuAgent) string {
	if agent == nil {
		return "其他客服"
	}
	if users, err := s.agentRepository.ListUsersByIDs([]int64{agent.UserID}); err == nil &&
		len(users) > 0 {
		if nickname := strings.TrimSpace(users[0].Nickname); nickname != "" {
			return nickname
		}
		if username := strings.TrimSpace(users[0].Username); username != "" {
			return username
		}
	}
	if agentNo := strings.TrimSpace(agent.AgentNo); agentNo != "" {
		return agentNo
	}
	return "其他客服"
}

// matchTransferKeyword 判断消息是否命中转接关键词，返回命中的第一个关键词
func matchTransferKeyword(content string, keywords []string) string {
	text := strings.TrimSpace(content)
	if text == "" {
		return ""
	}
	for _, keyword := range keywords {
		keyword = strings.TrimSpace(keyword)
		if keyword == "" {
			continue
		}
		if strings.Contains(text, keyword) {
			return keyword
		}
	}
	return ""
}
