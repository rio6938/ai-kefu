package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"go.uber.org/zap"

	"admin/server/internal/kefu/repository"
	"admin/server/model"
	"admin/server/pkg/log"
)

// 留言的业务错误
var (
	// ErrMessageLeaveNotFound 是留言不存在
	ErrMessageLeaveNotFound = repository.ErrMessageLeaveNotFound
	// ErrMessageLeaveContactTypeInvalid 是联系方式类型不正确
	ErrMessageLeaveContactTypeInvalid = errors.New("请选择手机号或邮箱作为联系方式")
	// ErrMessageLeaveContactInvalid 是联系方式格式不正确
	ErrMessageLeaveContactInvalid = errors.New("联系方式格式不正确")
	// ErrMessageLeaveContentRequired 是留言内容为空
	ErrMessageLeaveContentRequired = errors.New("留言内容不能为空")
	// ErrMessageLeaveHandled 是该留言已经处理过
	ErrMessageLeaveHandled = errors.New("该留言已处理")
)

// 留言相关的常量
const (
	// maxMessageLeaveContentLength 是留言内容的最大字数
	maxMessageLeaveContentLength = 500
	// maxMessageLeaveRemarkLength 是处理备注的最大字数
	maxMessageLeaveRemarkLength = 200
	// messageLeaveNoticePrefix 是提交留言后补进会话的系统消息前缀
	messageLeaveNoticePrefix = "访客留言："
)

// phonePattern 是手机号的校验规则，只做国内 11 位手机号的格式校验
var phonePattern = regexp.MustCompile(`^1[3-9]\d{9}$`)

// KefuMessageLeaveService 是离线留言的业务处理
type KefuMessageLeaveService struct {
	// 留言的数据访问
	leaveRepository *repository.KefuMessageLeaveRepository
	// 会话的业务处理，校验会话归属并给会话补系统消息
	sessionService *KefuSessionService
	// 会话消息的统一写入，复用分配服务的系统消息通道
	assignService *KefuAssignService
	// 客服档案的数据访问，回填处理人账号
	agentRepository *repository.KefuAgentRepository
}

// NewKefuMessageLeaveService 创建离线留言的业务处理
func NewKefuMessageLeaveService() *KefuMessageLeaveService {
	return &KefuMessageLeaveService{
		leaveRepository: repository.NewKefuMessageLeaveRepository(),
		sessionService:  NewKefuSessionService(),
		assignService:   NewKefuAssignService(),
		agentRepository: repository.NewKefuAgentRepository(),
	}
}

// Submit 访客提交留言：校验联系方式与内容，落库后往会话里补一条系统消息
func (s *KefuMessageLeaveService) Submit(
	ctx context.Context, sessionID int64, visitorKey string, form model.KefuMessageLeaveForm,
) (*model.KefuMessageLeave, error) {
	session, err := s.sessionService.ClientSession(sessionID, visitorKey)
	if err != nil {
		return nil, err
	}

	name := strings.TrimSpace(form.Name)
	contactType := strings.TrimSpace(form.ContactType)
	contact := strings.TrimSpace(form.Contact)
	content := strings.TrimSpace(form.Content)
	if err := validateLeaveContact(contactType, contact); err != nil {
		return nil, err
	}
	if content == "" {
		return nil, ErrMessageLeaveContentRequired
	}
	if runes := []rune(content); len(runes) > maxMessageLeaveContentLength {
		content = string(runes[:maxMessageLeaveContentLength])
	}
	if name == "" {
		name = "访客"
	}
	if runes := []rune(name); len(runes) > 64 {
		name = string(runes[:64])
	}

	now := model.Now()
	leave := &model.KefuMessageLeave{
		SessionID:   session.SessionID,
		VisitorID:   session.VisitorID,
		ChannelCode: session.ChannelCode,
		Name:        name,
		ContactType: contactType,
		Contact:     contact,
		Content:     content,
		Status:      model.KefuMessageLeaveStatusPending,
		CreateTime:  now,
		UpdateTime:  now,
	}
	if err := s.leaveRepository.Create(leave); err != nil {
		return nil, err
	}

	// 同步往会话里补一条系统消息，会话回放时能看到留言上下文
	notice := fmt.Sprintf("%s%s（%s）", messageLeaveNoticePrefix, content, contact)
	if err := s.assignService.sendSystemMessage(ctx, session.SessionID, notice); err != nil {
		log.Logger.Warn("send kefu leave message notice failed",
			zap.Int64("sessionId", session.SessionID), zap.Error(err))
	}
	return leave, nil
}

// Page 分页查询留言，并回填处理人账号
func (s *KefuMessageLeaveService) Page(
	param model.KefuMessageLeaveParam,
) ([]model.KefuMessageLeave, int64, error) {
	list, count, err := s.leaveRepository.Page(param, param.PageParam)
	if err != nil {
		return nil, 0, err
	}
	s.fillHandleUsername(list)
	return list, count, nil
}

// Handle 坐席处理留言：已处理的不再重复处理，可按需给访客补一条消息
func (s *KefuMessageLeaveService) Handle(
	ctx context.Context, userID int64, form model.KefuMessageLeaveHandleForm,
) error {
	leave, err := s.leaveRepository.GetByID(form.LeaveID)
	if err != nil {
		return err
	}
	if leave.Status == model.KefuMessageLeaveStatusHandled {
		return ErrMessageLeaveHandled
	}

	remark := strings.TrimSpace(form.Remark)
	if runes := []rune(remark); len(runes) > maxMessageLeaveRemarkLength {
		remark = string(runes[:maxMessageLeaveRemarkLength])
	}

	ok, err := s.leaveRepository.MarkHandled(leave.LeaveID, userID, remark, model.Now())
	if err != nil {
		return err
	}
	if !ok {
		return ErrMessageLeaveHandled
	}

	// 回复访客时只补一条系统消息，会话已结束或留言已超时不补
	if form.Reply && remark != "" {
		session, err := s.sessionService.ReadableSession(userID, leave.SessionID)
		if err != nil {
			log.Logger.Warn("load kefu leave session failed",
				zap.Int64("sessionId", leave.SessionID), zap.Error(err))
			return nil
		}
		if session.Status == model.KefuSessionStatusClosed {
			return nil
		}
		if err := s.assignService.sendSystemMessage(ctx, leave.SessionID, remark); err != nil {
			log.Logger.Warn("send kefu leave reply failed",
				zap.Int64("sessionId", leave.SessionID), zap.Error(err))
		}
	}
	return nil
}

// fillHandleUsername 批量回填留言处理人的账号
func (s *KefuMessageLeaveService) fillHandleUsername(list []model.KefuMessageLeave) {
	if len(list) == 0 {
		return
	}

	userIDs := make([]int64, 0, len(list))
	seen := make(map[int64]struct{}, len(list))
	for _, item := range list {
		if item.HandleUserID <= 0 {
			continue
		}
		if _, ok := seen[item.HandleUserID]; ok {
			continue
		}
		seen[item.HandleUserID] = struct{}{}
		userIDs = append(userIDs, item.HandleUserID)
	}
	if len(userIDs) == 0 {
		return
	}

	users, err := s.agentRepository.ListUsersByIDs(userIDs)
	if err != nil {
		log.Logger.Warn("list kefu leave handle users failed", zap.Error(err))
		return
	}
	names := make(map[int64]string, len(users))
	for _, user := range users {
		name := strings.TrimSpace(user.Nickname)
		if name == "" {
			name = user.Username
		}
		names[user.UserID] = name
	}
	for i := range list {
		list[i].HandleUsername = names[list[i].HandleUserID]
	}
}

// validateLeaveContact 校验联系方式类型与格式
func validateLeaveContact(contactType, contact string) error {
	if contactType != model.KefuMessageLeaveContactPhone &&
		contactType != model.KefuMessageLeaveContactEmail {
		return ErrMessageLeaveContactTypeInvalid
	}
	if contactType == model.KefuMessageLeaveContactPhone {
		if !phonePattern.MatchString(contact) {
			return ErrMessageLeaveContactInvalid
		}
		return nil
	}
	if !isValidEmail(contact) {
		return ErrMessageLeaveContactInvalid
	}
	return nil
}
