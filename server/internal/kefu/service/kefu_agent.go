package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"admin/server/internal/kefu/repository"
	"admin/server/model"
	"admin/server/pkg/log"
)

// 客服档案的业务错误
var (
	// ErrAgentUserRequired 是未选择关联的管理员账号
	ErrAgentUserRequired = errors.New("请选择关联的管理员账号")
	// ErrAgentUserNotFound 是关联的管理员账号不存在
	ErrAgentUserNotFound = repository.ErrAgentUserNotFound
	// ErrAgentUserExists 是该管理员已经建立了客服档案
	ErrAgentUserExists = errors.New("该管理员已经建立了客服档案")
	// ErrAgentStatusInvalid 是客服状态取值不正确
	ErrAgentStatusInvalid = errors.New("客服状态取值不正确")
	// ErrAgentAcceptStatusInvalid 是接单开关取值不正确
	ErrAgentAcceptStatusInvalid = errors.New("接单开关取值不正确")
	// ErrAgentWorkStatusInvalid 是工作状态取值不正确
	ErrAgentWorkStatusInvalid = errors.New("工作状态取值不正确")
	// ErrAgentSessionActive 是还有进行中的会话，不允许切离线
	ErrAgentSessionActive = errors.New("请先结束或转接后再切离线")
)

// maxRestReasonLength 是小休原因的最大字数
const maxRestReasonLength = 64

// agentDisabledReason 是客服停用后转出会话的转接原因
const agentDisabledReason = "客服停用转出"

// KefuAgentService 是客服档案的业务处理
type KefuAgentService struct {
	// 客服档案的数据访问
	agentRepository *repository.KefuAgentRepository
	// 会话的数据访问
	sessionRepository *repository.KefuSessionRepository
	// 客服在线状态的业务处理
	onlineService *KefuOnlineService
	// 会话转接的业务处理，客服停用时批量转出
	transferService *KefuTransferService
}

// NewKefuAgentService 创建客服档案的业务处理
func NewKefuAgentService() *KefuAgentService {
	return &KefuAgentService{
		agentRepository:   repository.NewKefuAgentRepository(),
		sessionRepository: repository.NewKefuSessionRepository(),
		onlineService:     NewKefuOnlineService(),
		transferService:   NewKefuTransferService(),
	}
}

// Page 分页查询客服档案，并回填账号、负载与在线状态
func (s *KefuAgentService) Page(
	ctx context.Context, param model.KefuAgentParam,
) ([]model.KefuAgent, int64, error) {
	list, count, err := s.agentRepository.Page(param, param.PageParam)
	if err != nil {
		return nil, 0, err
	}
	s.fillExtra(ctx, list)
	return list, count, nil
}

// List 查询启用中的客服档案
func (s *KefuAgentService) List(ctx context.Context) ([]model.KefuAgent, error) {
	list, err := s.agentRepository.List()
	if err != nil {
		return nil, err
	}
	s.fillExtra(ctx, list)
	return list, nil
}

// Add 新增客服档案，同时给关联的管理员挂上客服角色
func (s *KefuAgentService) Add(form model.KefuAgentForm) error {
	if err := normalizeAgentForm(&form); err != nil {
		return err
	}
	if _, err := s.agentRepository.GetUserByID(form.UserID); err != nil {
		return err
	}
	exists, err := s.agentRepository.ExistsByUserID(form.UserID, 0)
	if err != nil {
		return err
	}
	if exists {
		return ErrAgentUserExists
	}

	now := model.Now()
	agent := &model.KefuAgent{
		UserID:        form.UserID,
		AgentNo:       form.AgentNo,
		MaxConcurrent: form.MaxConcurrent,
		AcceptStatus:  form.AcceptStatus,
		AiDefault:     form.AiDefault,
		PromptID:      form.PromptID,
		WelcomeText:   form.WelcomeText,
		Status:        model.KefuAgentStatusNormal,
		CreateTime:    now,
		UpdateTime:    now,
	}
	if err := s.agentRepository.Create(agent); err != nil {
		return err
	}
	if err := s.agentRepository.AddRole(form.UserID, model.KefuRoleAgent); err != nil {
		// 角色没挂上不算致命错误，管理员仍可在用户管理里手工授权
		log.Logger.Warn("grant kefu role failed",
			zap.Int64("userId", form.UserID), zap.Error(err))
	}
	return nil
}

// Update 修改客服档案
func (s *KefuAgentService) Update(form model.KefuAgentForm) error {
	agent, err := s.agentRepository.GetByID(form.AgentID)
	if err != nil {
		return err
	}
	if err := normalizeAgentForm(&form); err != nil {
		return err
	}
	if form.UserID != agent.UserID {
		if _, err := s.agentRepository.GetUserByID(form.UserID); err != nil {
			return err
		}
	}
	exists, err := s.agentRepository.ExistsByUserID(form.UserID, agent.AgentID)
	if err != nil {
		return err
	}
	if exists {
		return ErrAgentUserExists
	}

	agent.UserID = form.UserID
	agent.AgentNo = form.AgentNo
	agent.MaxConcurrent = form.MaxConcurrent
	agent.AcceptStatus = form.AcceptStatus
	agent.AiDefault = form.AiDefault
	agent.PromptID = form.PromptID
	agent.WelcomeText = form.WelcomeText
	agent.UpdateTime = model.Now()
	return s.agentRepository.Update(agent)
}

// UpdateStatus 修改客服状态或接单开关，停用与停止接单都会把进行中的会话转出
func (s *KefuAgentService) UpdateStatus(
	ctx context.Context, form model.KefuAgentStatusForm,
) error {
	agent, err := s.agentRepository.GetByID(form.AgentID)
	if err != nil {
		return err
	}

	fields := make(map[string]any, 2)
	release := false
	if form.Status != nil {
		if *form.Status != model.KefuAgentStatusNormal &&
			*form.Status != model.KefuAgentStatusDisabled {
			return ErrAgentStatusInvalid
		}
		fields["status"] = *form.Status
		release = release || *form.Status == model.KefuAgentStatusDisabled
	}
	if form.AcceptStatus != nil {
		if *form.AcceptStatus != model.KefuAgentAcceptOn &&
			*form.AcceptStatus != model.KefuAgentAcceptOff {
			return ErrAgentAcceptStatusInvalid
		}
		fields["accept_status"] = *form.AcceptStatus
		release = release || *form.AcceptStatus == model.KefuAgentAcceptOff
	}
	if len(fields) == 0 {
		return ErrAgentStatusInvalid
	}
	if err := s.agentRepository.UpdateFields(agent.AgentID, fields); err != nil {
		return err
	}

	// 停用或停止接单后必须先把会话转出去，再让客服退出候选池
	if release {
		if err := s.transferService.TransferOut(ctx, agent.AgentID, agentDisabledReason); err != nil {
			log.Logger.Warn("transfer out kefu sessions failed",
				zap.Int64("agentId", agent.AgentID), zap.Error(err))
		}
	}
	return nil
}

// UpdateWorkStatus 坐席切换自己的工作状态（在线 / 小休 / 离线）
func (s *KefuAgentService) UpdateWorkStatus(
	userID int64, form model.KefuAgentWorkStatusForm,
) error {
	agent, err := s.agentRepository.GetByUserID(userID)
	if err != nil {
		return err
	}
	if form.WorkStatus != model.KefuAgentWorkOffline &&
		form.WorkStatus != model.KefuAgentWorkOnline &&
		form.WorkStatus != model.KefuAgentWorkRest {
		return ErrAgentWorkStatusInvalid
	}

	// 切离线时先把名下进行中的会话处理完，避免会话无人接手
	if form.WorkStatus == model.KefuAgentWorkOffline {
		loads, err := s.sessionRepository.CountActiveByAgents([]int64{agent.AgentID})
		if err != nil {
			return err
		}
		if total := loads[agent.AgentID]; total > 0 {
			return fmt.Errorf("%w：当前还有 %d 个会话进行中", ErrAgentSessionActive, total)
		}
	}

	reason := strings.TrimSpace(form.RestReason)
	if form.WorkStatus != model.KefuAgentWorkRest {
		reason = ""
	}
	if runes := []rune(reason); len(runes) > maxRestReasonLength {
		reason = string(runes[:maxRestReasonLength])
	}

	return s.agentRepository.UpdateFields(agent.AgentID, map[string]any{
		"work_status": form.WorkStatus,
		"rest_reason": reason,
		"status_time": model.Now(),
	})
}

// Remove 删除客服档案，先把进行中的会话转出再软删除
func (s *KefuAgentService) Remove(ctx context.Context, agentID int64) error {
	agent, err := s.agentRepository.GetByID(agentID)
	if err != nil {
		return err
	}
	if err := s.agentRepository.UpdateFields(agent.AgentID, map[string]any{
		"status":        model.KefuAgentStatusDisabled,
		"accept_status": model.KefuAgentAcceptOff,
	}); err != nil {
		return err
	}
	if err := s.transferService.TransferOut(ctx, agent.AgentID, agentDisabledReason); err != nil {
		log.Logger.Warn("transfer out kefu sessions failed",
			zap.Int64("agentId", agent.AgentID), zap.Error(err))
	}
	return s.agentRepository.Remove(agent.AgentID)
}

// fillExtra 批量回填客服的账号名称、进行中会话数与在线状态
func (s *KefuAgentService) fillExtra(ctx context.Context, list []model.KefuAgent) {
	if len(list) == 0 {
		return
	}

	agentIDs := make([]int64, 0, len(list))
	userIDs := make([]int64, 0, len(list))
	for _, agent := range list {
		agentIDs = append(agentIDs, agent.AgentID)
		userIDs = append(userIDs, agent.UserID)
	}

	loads, err := s.sessionRepository.CountActiveByAgents(agentIDs)
	if err != nil {
		log.Logger.Warn("count kefu agent sessions failed", zap.Error(err))
	}
	online := s.onlineService.OnlineMap(ctx, userIDs)

	userMap := make(map[int64]model.SysUser, len(userIDs))
	users, err := s.agentRepository.ListUsersByIDs(userIDs)
	if err != nil {
		log.Logger.Warn("list kefu agent users failed", zap.Error(err))
	}
	for _, user := range users {
		userMap[user.UserID] = user
	}

	for i := range list {
		list[i].SessionCount = loads[list[i].AgentID]
		list[i].Online = online[list[i].UserID]
		if user, ok := userMap[list[i].UserID]; ok {
			list[i].Username = user.Username
			list[i].Nickname = user.Nickname
		}
	}
}

// normalizeAgentForm 校验并归一化客服档案的提交数据
func normalizeAgentForm(form *model.KefuAgentForm) error {
	form.AgentNo = strings.TrimSpace(form.AgentNo)
	form.WelcomeText = strings.TrimSpace(form.WelcomeText)

	if form.UserID <= 0 {
		return ErrAgentUserRequired
	}

	if form.MaxConcurrent <= 0 {
		form.MaxConcurrent = defaultAgentMaxConcurrent
	}
	if form.AcceptStatus != model.KefuAgentAcceptOn &&
		form.AcceptStatus != model.KefuAgentAcceptOff {
		return ErrAgentAcceptStatusInvalid
	}
	if form.AiDefault != 0 && form.AiDefault != 1 {
		form.AiDefault = 1
	}
	return nil
}
