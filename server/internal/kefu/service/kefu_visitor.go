package service

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/mail"
	"strings"

	"go.uber.org/zap"

	"admin/server/internal/kefu/im"
	"admin/server/internal/kefu/repository"
	"admin/server/model"
	"admin/server/pkg/log"
	"admin/server/pkg/setting"
)

// 常见的业务错误
var (
	// ErrVisitorNotFound 是访客不存在
	ErrVisitorNotFound = repository.ErrVisitorNotFound
	// ErrImUnavailable 是连接 IM 失败
	ErrImUnavailable = errors.New("连接客服服务失败，请稍后重试")
	// ErrVisitorEmailInvalid 是邮箱格式不正确
	ErrVisitorEmailInvalid = errors.New("邮箱格式不正确")
)

// 浏览轨迹相关的配置与常量
const (
	// configKeyBrowseEnabled 是访客浏览轨迹的开关，默认关闭，需要运营显式开启
	configKeyBrowseEnabled = "kefu.browse.enabled"
	// maxBrowseTrail 是浏览轨迹保留的最大条数
	maxBrowseTrail = 10
)

// browseItem 是一条浏览轨迹，只记录页面标题与地址，不采集表单内容

type browseItem struct {
	// 页面标题
	Title string `json:"title"`
	// 页面地址
	URL string `json:"url"`
	// 记录时间
	Time model.Time `json:"time"`
}

// KefuVisitorService 是访客的业务处理
type KefuVisitorService struct {
	// 访客的数据访问
	visitorRepository *repository.KefuVisitorRepository
	// 客服档案的数据访问，取客服的欢迎语与名称
	agentRepository *repository.KefuAgentRepository
	// 渠道的业务处理
	channelService *KefuChannelService
	// 会话的业务处理
	sessionService *KefuSessionService
}

// NewKefuVisitorService 创建访客的业务处理
func NewKefuVisitorService() *KefuVisitorService {
	return &KefuVisitorService{
		visitorRepository: repository.NewKefuVisitorRepository(),
		agentRepository:   repository.NewKefuAgentRepository(),
		channelService:    NewKefuChannelService(),
		sessionService:    NewKefuSessionService(),
	}
}

// Bootstrap 处理访客接入：取建访客、下发 IM token、返回当前会话
func (s *KefuVisitorService) Bootstrap(
	ctx context.Context, form model.KefuBootstrapForm, ip string,
) (*model.KefuBootstrapResult, error) {
	form.ChannelCode = strings.TrimSpace(form.ChannelCode)
	if form.ChannelCode == "" {
		return nil, ErrChannelCodeRequired
	}
	form.Nickname = strings.TrimSpace(form.Nickname)
	form.Email = strings.TrimSpace(form.Email)
	if form.Email != "" && !isValidEmail(form.Email) {
		return nil, ErrVisitorEmailInvalid
	}
	channel, err := s.channelService.getEnabled(form.ChannelCode)
	if err != nil {
		return nil, err
	}

	visitor, err := s.getOrCreateVisitor(form, ip)
	if err != nil {
		return nil, err
	}

	token, err := randomToken()
	if err != nil {
		return nil, err
	}
	uid := visitorUID(visitor.VisitorID)
	if err := im.Default.EnsureUserToken(ctx, uid, token); err != nil {
		log.Logger.Error("ensure kefu visitor im token failed",
			zap.Int64("visitorId", visitor.VisitorID), zap.Error(err))
		return nil, ErrImUnavailable
	}

	session, err := s.sessionService.EnsureSession(ctx, visitor, channel)
	if err != nil {
		return nil, err
	}

	result := &model.KefuBootstrapResult{
		UID:           uid,
		Token:         token,
		WsURL:         im.WsURL(),
		AiUID:         im.AiUID(),
		VisitorID:     visitor.VisitorID,
		VisitorKey:    visitor.VisitorKey,
		Nickname:      visitor.Nickname,
		Email:         visitor.Email,
		SessionID:     session.SessionID,
		SessionStatus: session.Status,
		AiMode:        session.AiMode,
		WelcomeText:   channel.WelcomeText,
	}
	s.fillAgent(result, session)
	return result, nil
}

// getOrCreateVisitor 按访客标识取访客，不存在时新建；每次接入都刷新活跃信息
func (s *KefuVisitorService) getOrCreateVisitor(
	form model.KefuBootstrapForm, ip string,
) (*model.KefuVisitor, error) {
	now := model.Now()
	visitorKey := strings.TrimSpace(form.VisitorKey)
	if visitorKey == "" {
		visitorKey = randomHex(16)
	}

	visitor, err := s.visitorRepository.GetByKey(visitorKey)
	if err == nil {
		if nickname := strings.TrimSpace(form.Nickname); nickname != "" {
			visitor.Nickname = nickname
		}
		// 空串视为未填，不能用它覆盖已有邮箱
		if email := strings.TrimSpace(form.Email); email != "" {
			visitor.Email = email
		}
		visitor.IP = ip
		if sourceURL := strings.TrimSpace(form.SourceURL); sourceURL != "" {
			visitor.SourceURL = sourceURL
		}
		visitor.LastActiveTime = now
		visitor.UpdateTime = now
		if err := s.visitorRepository.Update(visitor); err != nil {
			return nil, err
		}
		return visitor, nil
	}
	if !errors.Is(err, repository.ErrVisitorNotFound) {
		return nil, err
	}

	nickname := strings.TrimSpace(form.Nickname)
	if nickname == "" {
		nickname = "访客" + lastChars(visitorKey, 4)
	}
	visitor = &model.KefuVisitor{
		VisitorKey:     visitorKey,
		ChannelCode:    form.ChannelCode,
		Nickname:       nickname,
		Email:          strings.TrimSpace(form.Email),
		IP:             ip,
		SourceURL:      strings.TrimSpace(form.SourceURL),
		LastActiveTime: now,
		CreateTime:     now,
		UpdateTime:     now,
	}
	if err := s.visitorRepository.Create(visitor); err != nil {
		return nil, err
	}
	return visitor, nil
}

// TrackBrowse 记录访客浏览的页面标题与地址，最多保留 10 条；
// 开关未开启时直接忽略，不落库也不报错
func (s *KefuVisitorService) TrackBrowse(form model.KefuBrowseTrackForm) error {
	if !setting.GetBool(configKeyBrowseEnabled, false) {
		return nil
	}
	url := strings.TrimSpace(form.URL)
	if url == "" {
		return nil
	}
	visitor, err := s.visitorRepository.GetByKey(strings.TrimSpace(form.VisitorKey))
	if err != nil {
		return err
	}

	title := strings.TrimSpace(form.Title)
	items := decodeBrowseTrail(visitor.BrowseTrail)
	// 与上一条相同的页面不重复记，避免刷新页面刷屏
	if n := len(items); n > 0 && items[n-1].URL == url && items[n-1].Title == title {
		return nil
	}
	items = append(items, browseItem{Title: title, URL: url, Time: model.Now()})
	if len(items) > maxBrowseTrail {
		items = items[len(items)-maxBrowseTrail:]
	}

	raw, err := json.Marshal(items)
	if err != nil {
		return err
	}
	visitor.BrowseTrail = string(raw)
	visitor.UpdateTime = model.Now()
	return s.visitorRepository.Update(visitor)
}

// decodeBrowseTrail 解析已存的浏览轨迹，格式异常时返回空列表
func decodeBrowseTrail(value string) []browseItem {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	var items []browseItem
	if err := json.Unmarshal([]byte(value), &items); err != nil {
		log.Logger.Warn("decode kefu browse trail failed", zap.Error(err))
		return nil
	}
	return items
}

// fillAgent 回填接待客服的名称与欢迎语，未分配时用渠道的欢迎语
func (s *KefuVisitorService) fillAgent(result *model.KefuBootstrapResult, session *model.KefuSession) {
	if session.AgentID == 0 {
		return
	}
	agent, err := s.agentRepository.GetByID(session.AgentID)
	if err != nil {
		log.Logger.Warn("load kefu agent failed",
			zap.Int64("agentId", session.AgentID), zap.Error(err))
		return
	}
	if text := strings.TrimSpace(agent.WelcomeText); text != "" {
		result.WelcomeText = text
	}
	users, err := s.agentRepository.ListUsersByIDs([]int64{agent.UserID})
	if err != nil {
		log.Logger.Warn("load kefu agent user failed",
			zap.Int64("userId", agent.UserID), zap.Error(err))
		return
	}
	if len(users) == 0 {
		return
	}
	if nickname := strings.TrimSpace(users[0].Nickname); nickname != "" {
		result.AgentName = nickname
		return
	}
	result.AgentName = users[0].Username
}

// lastChars 返回字符串末尾的 n 个字符
func lastChars(value string, n int) string {
	chars := []rune(value)
	if len(chars) <= n {
		return value
	}
	return string(chars[len(chars)-n:])
}

// isValidEmail 判断邮箱格式是否合法
func isValidEmail(value string) bool {
	address, err := mail.ParseAddress(value)
	if err != nil {
		return false
	}
	return address.Address == value
}

// EnsureMailVisitor 按发件人邮箱取建访客，访客标识用 mail_ 加邮箱 sha1，避免明文邮箱进标识；
// 昵称优先用发件人显示名，取不到时用邮箱前缀，不覆盖已有非空昵称
func (s *KefuVisitorService) EnsureMailVisitor(
	channelCode, email, name string,
) (*model.KefuVisitor, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return nil, ErrVisitorEmailInvalid
	}
	visitorKey := mailVisitorKey(email)

	visitor, err := s.visitorRepository.GetByKey(visitorKey)
	if err == nil {
		nickname := strings.TrimSpace(name)
		if nickname != "" && strings.TrimSpace(visitor.Nickname) == "" {
			visitor.Nickname = nickname
		}
		visitor.LastActiveTime = model.Now()
		visitor.UpdateTime = model.Now()
		if err := s.visitorRepository.Update(visitor); err != nil {
			return nil, err
		}
		return visitor, nil
	}
	if !errors.Is(err, repository.ErrVisitorNotFound) {
		return nil, err
	}

	nickname := strings.TrimSpace(name)
	if nickname == "" {
		nickname = strings.SplitN(email, "@", 2)[0]
	}
	now := model.Now()
	visitor = &model.KefuVisitor{
		VisitorKey:     visitorKey,
		ChannelCode:    channelCode,
		Nickname:       nickname,
		Email:          email,
		LastActiveTime: now,
		CreateTime:     now,
		UpdateTime:     now,
	}
	if err := s.visitorRepository.Create(visitor); err != nil {
		return nil, err
	}
	return visitor, nil
}

// mailVisitorKey 用 sha1 生成邮箱访客标识，避免邮件地址明文进入访客表
func mailVisitorKey(email string) string {
	sum := sha1.Sum([]byte(strings.ToLower(strings.TrimSpace(email))))
	return "mail_" + hex.EncodeToString(sum[:])
}

// MarkMailInvalid 收到退信后把发件人邮箱标记为无效，停止自动回复并提醒人工
func (s *KefuVisitorService) MarkMailInvalid(email string) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return
	}
	visitor, err := s.visitorRepository.GetByKey(mailVisitorKey(email))
	if err != nil {
		return
	}
	if visitor.MailInvalid == 1 {
		return
	}
	visitor.MailInvalid = 1
	visitor.UpdateTime = model.Now()
	if err := s.visitorRepository.Update(visitor); err != nil {
		log.Logger.Warn("mark kefu visitor mail invalid failed",
			zap.String("email", email), zap.Error(err))
	}
}
