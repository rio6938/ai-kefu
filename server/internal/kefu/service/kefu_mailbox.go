package service

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	mailclient "admin/server/internal/kefu/mail"
	"admin/server/internal/kefu/repository"
	"admin/server/model"
	"admin/server/pkg/encrypt"
	pkgmail "admin/server/pkg/mail"
	"admin/server/pkg/setting"
)

// 邮箱渠道使用到的配置项，全部可在配置中心修改
const (
	// configKeyMailEnabled 是邮箱渠道总开关
	configKeyMailEnabled = "kefu.mail.enabled"
	// configKeyMailMaxSizeMB 是单封邮件大小上限，单位 MB
	configKeyMailMaxSizeMB = "kefu.mail.max_size_mb"
	// configKeyMailSyncIntervalSec 是收信轮询间隔，单位秒
	configKeyMailSyncIntervalSec = "kefu.mail.sync_interval_sec"
	// configKeyMailFirstSyncDays 是首次同步只拉最近 N 天
	configKeyMailFirstSyncDays = "kefu.mail.first_sync_days"
	// configKeyMailThreadWindowDays 是无线程头时按主题匹配线程的时间窗口
	configKeyMailThreadWindowDays = "kefu.mail.thread_window_days"
	// configKeyMailReopenDays 是已结束线程再次来信的重开窗口
	configKeyMailReopenDays = "kefu.mail.reopen_days"
	// configKeyMailCloseDays 是邮件会话自动结束的天数
	configKeyMailCloseDays = "kefu.mail.close_days"
	// configKeyMailDefaultReplyMode 是新建邮箱的默认回复模式
	configKeyMailDefaultReplyMode = "kefu.mail.default_reply_mode"
	// configKeyMailDelayMinutes 是延时自动发送的默认分钟数
	configKeyMailDelayMinutes = "kefu.mail.delay_minutes"
	// configKeyMailAutoAckEnabled 是新建邮箱默认是否自动确认
	configKeyMailAutoAckEnabled = "kefu.mail.auto_ack_enabled"
	// configKeyMailMaxAutoReplyPerThread 是同一线程最多自动回复次数
	configKeyMailMaxAutoReplyPerThread = "kefu.mail.max_auto_reply_per_thread"
	// configKeyMailMaxAutoReplyPerSenderDay 是同一发件人每日自动回复上限
	configKeyMailMaxAutoReplyPerSenderDay = "kefu.mail.max_auto_reply_per_sender_day"
	// configKeyMailMinAutoReplyIntervalMin 是同一线程两次自动回复最小间隔
	configKeyMailMinAutoReplyIntervalMin = "kefu.mail.min_auto_reply_interval_min"
	// configKeyMailReplyMaxChars 是 AI 邮件正文最大字符数
	configKeyMailReplyMaxChars = "kefu.mail.reply_max_chars"
	// configKeyMailRawKeepDays 是原始邮件保留天数
	configKeyMailRawKeepDays = "kefu.mail.raw_keep_days"
	// configKeyMailAttachmentKeepDays 是邮件附件保留天数
	configKeyMailAttachmentKeepDays = "kefu.mail.attachment_keep_days"
	// configKeyMailOAuthGoogleClientID 是 Google OAuth2 客户端 id
	configKeyMailOAuthGoogleClientID = "kefu.mail.oauth.google_client_id"
	// configKeyMailOAuthGoogleClientSecret 是 Google OAuth2 密钥
	configKeyMailOAuthGoogleClientSecret = "kefu.mail.oauth.google_client_secret"
	// configKeyMailOAuthMicrosoftClientID 是 Microsoft OAuth2 客户端 id
	configKeyMailOAuthMicrosoftClientID = "kefu.mail.oauth.microsoft_client_id"
	// configKeyMailOAuthMicrosoftClientSecret 是 Microsoft OAuth2 密钥
	configKeyMailOAuthMicrosoftClientSecret = "kefu.mail.oauth.microsoft_client_secret"
	// configKeyMailOAuthRedirectURL 是 OAuth2 回调地址
	configKeyMailOAuthRedirectURL = "kefu.mail.oauth.redirect_url"
)

// 邮箱渠道的默认值
const (
	// defaultMailMaxSizeMB 是单封邮件大小上限默认值
	defaultMailMaxSizeMB = 5
	// defaultMailSyncIntervalSec 是收信轮询间隔默认值
	defaultMailSyncIntervalSec = 60
	// defaultMailFirstSyncDays 是首次同步天数默认值
	defaultMailFirstSyncDays = 7
	// defaultMailThreadWindowDays 是线程匹配时间窗口默认值
	defaultMailThreadWindowDays = 30
	// defaultMailReopenDays 是重开会话窗口默认值
	defaultMailReopenDays = 7
	// defaultMailCloseDays 是邮件会话自动结束默认值
	defaultMailCloseDays = 14
	// defaultMailDelayMinutes 是延时发送默认分钟数
	defaultMailDelayMinutes = 15
	// defaultMailMaxAutoReplyPerThread 是同线程自动回复次数上限
	defaultMailMaxAutoReplyPerThread = 3
	// defaultMailMaxAutoReplyPerSenderDay 是同发件人每日自动回复上限
	defaultMailMaxAutoReplyPerSenderDay = 10
	// defaultMailMinAutoReplyIntervalMin 是同线程自动回复最小间隔
	defaultMailMinAutoReplyIntervalMin = 10
	// defaultMailReplyMaxChars 是 AI 邮件正文最大字符数
	defaultMailReplyMaxChars = 2000
	// defaultMailRawKeepDays 是原始邮件保留天数默认值
	defaultMailRawKeepDays = 30
	// defaultMailAttachmentKeepDays 是邮件附件保留天数默认值
	defaultMailAttachmentKeepDays = 30
)

// 邮箱授权与收发的其它常量
const (
	// mailPasswordMask 是前端回显的密码占位符，提交时表示不修改
	mailPasswordMask = "********"
	// mailOAuthStatePrefix 是 OAuth2 一次性 state 的 redis 前缀
	mailOAuthStatePrefix = "kefu:mail:oauth:state:"
	// mailOAuthStateTTL 是 state 的有效期
	mailOAuthStateTTL = 10 * time.Minute
	// mailTokenRefreshAhead 是访问令牌提前刷新的时间
	mailTokenRefreshAhead = 2 * time.Minute
)

// 邮箱相关的业务错误
var (
	// ErrMailboxNotFound 是邮箱配置不存在
	ErrMailboxNotFound = repository.ErrMailboxNotFound
	// ErrMailChannelOnly 是只能给邮箱渠道配置邮箱
	ErrMailChannelOnly = errors.New("只有邮箱类型的渠道可以配置邮箱")
	// ErrMailEmailRequired 是邮箱地址不能为空
	ErrMailEmailRequired = errors.New("邮箱地址不能为空")
	// ErrMailEmailInvalid 是邮箱地址格式不正确
	ErrMailEmailInvalid = errors.New("邮箱地址格式不正确")
	// ErrMailEmailExists 是邮箱地址已被其它渠道绑定
	ErrMailEmailExists = errors.New("该邮箱地址已被其它渠道绑定")
	// ErrMailChannelBound 是该渠道已绑定其它邮箱
	ErrMailChannelBound = errors.New("该渠道已绑定其它邮箱")
	// ErrMailProviderInvalid 是服务商不支持
	ErrMailProviderInvalid = errors.New("不支持该邮箱服务商")
	// ErrMailAuthTypeInvalid 是认证方式不支持
	ErrMailAuthTypeInvalid = errors.New("认证方式不正确")
	// ErrMailReplyModeInvalid 是自动回复模式不正确
	ErrMailReplyModeInvalid = errors.New("自动回复模式不正确")
	// ErrMailLanguageInvalid 是回复语言不正确
	ErrMailLanguageInvalid = errors.New("回复语言不正确")
	// ErrMailPasswordRequired 是密码或授权码不能为空
	ErrMailPasswordRequired = errors.New("密码或授权码不能为空")
	// ErrMailCredentialsMissing 是邮箱凭据不完整
	ErrMailCredentialsMissing = errors.New("邮箱账号或密码未配置")
	// ErrMailOAuthRequired 是 Outlook 等邮箱必须使用 OAuth2
	ErrMailOAuthRequired = errors.New("该邮箱需要使用 OAuth2 授权")
	// ErrMailOAuthNotConfigured 是 OAuth2 客户端信息未配置
	ErrMailOAuthNotConfigured = errors.New("请先在配置中心填写 OAuth 客户端信息")
	// ErrMailOAuthStateInvalid 是 OAuth2 回调 state 无效或已过期
	ErrMailOAuthStateInvalid = errors.New("授权已过期，请重新发起授权")
	// ErrMailDisabled 是邮箱渠道总开关已关闭
	ErrMailDisabled = errors.New("邮箱渠道已关闭")
	// ErrMailboxStatusInvalid 是邮箱状态不正确
	ErrMailboxStatusInvalid = errors.New("邮箱状态不正确")
)

// KefuMailboxService 是邮箱配置的业务处理
type KefuMailboxService struct {
	// 邮箱配置的数据访问
	mailboxRepository *repository.KefuMailboxRepository
	// 渠道的数据访问
	channelRepository *repository.KefuChannelRepository
}

// NewKefuMailboxService 创建邮箱配置的业务处理
func NewKefuMailboxService() *KefuMailboxService {
	return &KefuMailboxService{
		mailboxRepository: repository.NewKefuMailboxRepository(),
		channelRepository: repository.NewKefuChannelRepository(),
	}
}

// mailProviderPresets 返回服务商预设，outlook 依赖配置中心是否已填 OAuth 客户端信息
func mailProviderPresets() []model.KefuMailProviderPreset {
	microsoftReady := strings.TrimSpace(setting.GetString(configKeyMailOAuthMicrosoftClientID, "")) != ""
	return []model.KefuMailProviderPreset{
		{
			Provider: model.KefuMailProviderGmail, Name: "Gmail 个人", IMAPHost: "imap.gmail.com",
			IMAPPort: 993, IMAPSSL: 1, SMTPHost: "smtp.gmail.com", SMTPPort: 465, SMTPSSL: 1,
			Folder: "INBOX", AuthType: model.KefuMailAuthPassword, Enabled: true,
			Tip: "仅 @gmail.com 个人邮箱；需开启两步验证后生成应用专用密码，并在 Gmail 设置中启用 IMAP",
		},
		{
			Provider: model.KefuMailProviderQQ, Name: "QQ 邮箱", IMAPHost: "imap.qq.com",
			IMAPPort: 993, IMAPSSL: 1, SMTPHost: "smtp.qq.com", SMTPPort: 465, SMTPSSL: 1,
			Folder: "INBOX", AuthType: model.KefuMailAuthPassword, Enabled: true,
			Tip: "需在设置中开启 IMAP/SMTP 服务，密码填授权码，不是登录密码",
		},
		{
			Provider: model.KefuMailProvider163, Name: "163 邮箱", IMAPHost: "imap.163.com",
			IMAPPort: 993, IMAPSSL: 1, SMTPHost: "smtp.163.com", SMTPPort: 465, SMTPSSL: 1,
			Folder: "INBOX", AuthType: model.KefuMailAuthPassword, Enabled: true,
			Tip: "需开启 IMAP/SMTP 服务，密码填授权码；登录后系统会自动发送 IMAP ID 命令",
		},
		{
			Provider: model.KefuMailProviderOutlook, Name: "Outlook 个人", IMAPHost: "outlook.office365.com",
			IMAPPort: 993, IMAPSSL: 1, SMTPHost: "smtp.office365.com", SMTPPort: 587, SMTPSSL: 0,
			Folder: "INBOX", AuthType: model.KefuMailAuthOAuth2, NeedOAuth: true, Enabled: microsoftReady,
			Tip: "仅 @outlook.com / @hotmail.com / @live.com 个人邮箱，必须使用 OAuth2 授权",
		},
		{
			Provider: model.KefuMailProviderCustom, Name: "自定义", Folder: "INBOX",
			AuthType: model.KefuMailAuthPassword, Enabled: true,
			Tip: "仅作兜底，企业邮局不承诺可用；自定义 SMTP 需自行配置 SPF、DKIM、DMARC",
		},
	}
}

// Providers 返回服务商预设列表
func (s *KefuMailboxService) Providers() []model.KefuMailProviderPreset {
	return mailProviderPresets()
}

// Get 查询渠道绑定的邮箱配置，凭据只返回是否已配置
func (s *KefuMailboxService) Get(channelID int64) (*model.KefuMailboxResult, error) {
	mailbox, err := s.mailboxRepository.GetByChannelID(channelID)
	if err != nil {
		return nil, err
	}
	return toMailboxResult(mailbox), nil
}

// DisableByChannelID 渠道改成非邮箱类型时停用邮箱配置，保留记录与凭据，不物理删除
func (s *KefuMailboxService) DisableByChannelID(channelID int64) {
	mailbox, err := s.mailboxRepository.GetByChannelID(channelID)
	if err != nil {
		return
	}
	if mailbox.Status == model.KefuMailboxStatusDisabled {
		return
	}
	_ = s.mailboxRepository.UpdateFields(mailbox.MailboxID, map[string]any{
		"status":      model.KefuMailboxStatusDisabled,
		"update_time": model.Now(),
	})
}

// Save 新增或修改渠道绑定的邮箱配置，一个渠道只能绑定一个邮箱
func (s *KefuMailboxService) Save(channelID int64, form model.KefuMailboxForm) error {
	channel, err := s.channelRepository.GetByID(channelID)
	if err != nil {
		return err
	}
	if channel.ChannelType != model.KefuChannelTypeEmail {
		return ErrMailChannelOnly
	}

	if err := normalizeMailboxForm(&form); err != nil {
		return err
	}
	form.ChannelID = channelID

	exists, err := s.mailboxRepository.ExistsByEmail(form.EmailAddress, 0)
	if err != nil {
		return err
	}
	mailbox, err := s.mailboxRepository.GetByChannelID(channelID)
	if err != nil && !errors.Is(err, repository.ErrMailboxNotFound) {
		return err
	}
	if mailbox != nil {
		exists, err = s.mailboxRepository.ExistsByEmail(form.EmailAddress, mailbox.MailboxID)
		if err != nil {
			return err
		}
	}
	if exists {
		return ErrMailEmailExists
	}

	isNew := mailbox == nil
	if isNew {
		mailbox = &model.KefuMailbox{
			ChannelID:      channelID,
			ReplyMode:      defaultMailReplyMode(),
			DelayMinutes:   setting.GetInt(configKeyMailDelayMinutes, defaultMailDelayMinutes),
			AutoAckEnabled: boolToInt(setting.GetBool(configKeyMailAutoAckEnabled, true)),
			Language:       model.KefuMailLanguageAuto,
			QuoteOriginal:  1,
			SyncEnabled:    1,
			Folder:         "INBOX",
			Status:         model.KefuMailboxStatusNormal,
			CreateTime:     model.Now(),
			UpdateTime:     model.Now(),
		}
	}

	if err := s.applyMailboxForm(mailbox, form); err != nil {
		return err
	}
	if isNew {
		return s.mailboxRepository.Create(mailbox)
	}
	return s.mailboxRepository.Update(mailbox)
}

// applyMailboxForm 把提交数据合并到邮箱配置，密码为空时保留原密文
func (s *KefuMailboxService) applyMailboxForm(
	mailbox *model.KefuMailbox, form model.KefuMailboxForm,
) error {
	mailbox.Provider = form.Provider
	mailbox.EmailAddress = form.EmailAddress
	mailbox.DisplayName = form.DisplayName
	mailbox.IMAPHost = form.IMAPHost
	mailbox.IMAPPort = form.IMAPPort
	mailbox.IMAPSSL = form.IMAPSSL
	mailbox.IMAPUsername = form.IMAPUsername
	mailbox.SMTPHost = form.SMTPHost
	mailbox.SMTPPort = form.SMTPPort
	mailbox.SMTPSSL = form.SMTPSSL
	mailbox.SMTPUsername = form.SMTPUsername
	mailbox.AuthType = form.AuthType
	mailbox.OAuthProvider = form.OAuthProvider
	mailbox.Folder = form.Folder
	mailbox.SyncSent = form.SyncSent
	mailbox.SentFolder = form.SentFolder
	mailbox.SyncEnabled = form.SyncEnabled
	mailbox.SyncIntervalSec = form.SyncIntervalSec
	mailbox.MarkSeen = form.MarkSeen
	mailbox.Aliases = form.Aliases
	mailbox.ReplyMode = form.ReplyMode
	mailbox.DelayMinutes = form.DelayMinutes
	mailbox.AutoAckEnabled = form.AutoAckEnabled
	mailbox.Language = form.Language
	mailbox.SignatureZh = form.SignatureZh
	mailbox.SignatureEn = form.SignatureEn
	if form.Templates != "" {
		mailbox.Templates = form.Templates
	}
	mailbox.QuoteOriginal = form.QuoteOriginal
	mailbox.MaxSizeMB = form.MaxSizeMB
	mailbox.WorkTimeEnabled = form.WorkTimeEnabled
	mailbox.WorkTimeStart = form.WorkTimeStart
	mailbox.WorkTimeEnd = form.WorkTimeEnd
	mailbox.WorkDays = form.WorkDays
	mailbox.Holidays = form.Holidays
	mailbox.Status = form.Status
	mailbox.UpdateTime = model.Now()

	if mailbox.AuthType == model.KefuMailAuthOAuth2 {
		if oauthProviderOf(mailbox.Provider) == "" {
			return ErrMailProviderInvalid
		}
		mailbox.OAuthProvider = oauthProviderOf(mailbox.Provider)
	} else {
		// 切回密码认证时清掉 OAuth 令牌，避免残留凭据继续留在库里
		mailbox.OAuthProvider = ""
		mailbox.OAuthAccessToken = ""
		mailbox.OAuthRefreshToken = ""
		mailbox.OAuthExpireTime = model.Time{}
	}

	imapPassword, err := encryptMailboxSecret(form.IMAPPassword)
	if err != nil {
		return err
	}
	if imapPassword != "" {
		mailbox.IMAPPassword = imapPassword
	}
	smtpPassword, err := encryptMailboxSecret(form.SMTPPassword)
	if err != nil {
		return err
	}
	if smtpPassword != "" {
		mailbox.SMTPPassword = smtpPassword
	}
	// SMTP 与 IMAP 同账号时允许复用同一个密码，减少重复填写
	if mailbox.SMTPPassword == "" && mailbox.AuthType == model.KefuMailAuthPassword &&
		strings.EqualFold(mailbox.SMTPUsername, mailbox.IMAPUsername) {
		mailbox.SMTPPassword = mailbox.IMAPPassword
	}
	if mailbox.AuthType == model.KefuMailAuthPassword &&
		mailbox.IMAPPassword == "" && mailbox.SMTPPassword == "" {
		return ErrMailPasswordRequired
	}
	return nil
}

// encryptMailboxSecret 加密邮箱密码，空值与掩码返回空串表示不修改
func encryptMailboxSecret(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || value == mailPasswordMask {
		return "", nil
	}
	encrypted, err := encrypt.Encrypt(value)
	if err != nil {
		return "", err
	}
	return encrypted, nil
}

// normalizeMailboxForm 校验并归一化邮箱配置的提交数据
func normalizeMailboxForm(form *model.KefuMailboxForm) error {
	form.EmailAddress = strings.ToLower(strings.TrimSpace(form.EmailAddress))
	form.DisplayName = strings.TrimSpace(form.DisplayName)
	form.IMAPHost = strings.TrimSpace(form.IMAPHost)
	form.SMTPHost = strings.TrimSpace(form.SMTPHost)
	form.IMAPUsername = strings.TrimSpace(form.IMAPUsername)
	form.SMTPUsername = strings.TrimSpace(form.SMTPUsername)
	form.Folder = strings.TrimSpace(form.Folder)
	form.SentFolder = strings.TrimSpace(form.SentFolder)
	form.AuthType = strings.TrimSpace(form.AuthType)
	form.Provider = strings.TrimSpace(form.Provider)
	form.ReplyMode = strings.TrimSpace(form.ReplyMode)
	form.Language = strings.TrimSpace(form.Language)
	form.Aliases = strings.TrimSpace(form.Aliases)
	form.Templates = strings.TrimSpace(form.Templates)
	form.WorkTimeStart = strings.TrimSpace(form.WorkTimeStart)
	form.WorkTimeEnd = strings.TrimSpace(form.WorkTimeEnd)
	form.WorkDays = strings.TrimSpace(form.WorkDays)
	form.Holidays = strings.TrimSpace(form.Holidays)

	if form.EmailAddress == "" {
		return ErrMailEmailRequired
	}
	if _, err := mail.ParseAddress(form.EmailAddress); err != nil {
		return ErrMailEmailInvalid
	}
	if form.Provider == "" {
		form.Provider = model.KefuMailProviderCustom
	}
	if !mailProviderSupported(form.Provider) {
		return ErrMailProviderInvalid
	}
	if form.AuthType == "" {
		form.AuthType = model.KefuMailAuthPassword
	}
	if form.AuthType != model.KefuMailAuthPassword && form.AuthType != model.KefuMailAuthOAuth2 {
		return ErrMailAuthTypeInvalid
	}
	// Outlook 个人已停用基本认证，必须走 OAuth2
	if form.Provider == model.KefuMailProviderOutlook &&
		form.AuthType != model.KefuMailAuthOAuth2 {
		return ErrMailOAuthRequired
	}
	if form.ReplyMode == "" {
		form.ReplyMode = defaultMailReplyMode()
	}
	switch form.ReplyMode {
	case model.KefuMailReplyModeOff, model.KefuMailReplyModeDraft,
		model.KefuMailReplyModeDelay, model.KefuMailReplyModeAuto:
	default:
		return ErrMailReplyModeInvalid
	}
	if form.Language == "" {
		form.Language = model.KefuMailLanguageAuto
	}
	switch form.Language {
	case model.KefuMailLanguageAuto, model.KefuMailLanguageZh, model.KefuMailLanguageEn:
	default:
		return ErrMailLanguageInvalid
	}
	if form.Folder == "" {
		form.Folder = "INBOX"
	}
	if form.SentFolder == "" {
		form.SentFolder = "Sent"
	}
	if form.IMAPPort <= 0 || form.IMAPPort > 65535 {
		return errors.New("IMAP 端口不正确")
	}
	if form.SMTPPort <= 0 || form.SMTPPort > 65535 {
		return errors.New("SMTP 端口不正确")
	}
	if form.IMAPHost == "" || form.SMTPHost == "" {
		return errors.New("请填写收信与发信服务器地址")
	}
	if form.IMAPUsername == "" {
		form.IMAPUsername = form.EmailAddress
	}
	if form.SMTPUsername == "" {
		form.SMTPUsername = form.EmailAddress
	}
	if form.DelayMinutes < 0 {
		form.DelayMinutes = 0
	}
	if form.Status != model.KefuMailboxStatusNormal &&
		form.Status != model.KefuMailboxStatusDisabled {
		return ErrMailboxStatusInvalid
	}
	return nil
}

// mailProviderSupported 判断服务商是否支持
func mailProviderSupported(provider string) bool {
	for _, item := range mailProviderPresets() {
		if item.Provider == provider {
			return true
		}
	}
	return false
}

// oauthProviderOf 返回邮箱服务商对应的 OAuth 服务商
func oauthProviderOf(provider string) string {
	switch provider {
	case model.KefuMailProviderGmail:
		return "google"
	case model.KefuMailProviderOutlook:
		return "microsoft"
	default:
		return ""
	}
}

// toMailboxResult 把邮箱配置转成脱敏的返回结构
func toMailboxResult(mailbox *model.KefuMailbox) *model.KefuMailboxResult {
	return &model.KefuMailboxResult{
		MailboxID:       mailbox.MailboxID,
		ChannelID:       mailbox.ChannelID,
		Provider:        mailbox.Provider,
		EmailAddress:    mailbox.EmailAddress,
		DisplayName:     mailbox.DisplayName,
		IMAPHost:        mailbox.IMAPHost,
		IMAPPort:        mailbox.IMAPPort,
		IMAPSSL:         mailbox.IMAPSSL,
		IMAPUsername:    mailbox.IMAPUsername,
		HasIMAPPassword: mailbox.IMAPPassword != "",
		SMTPHost:        mailbox.SMTPHost,
		SMTPPort:        mailbox.SMTPPort,
		SMTPSSL:         mailbox.SMTPSSL,
		SMTPUsername:    mailbox.SMTPUsername,
		HasSMTPPassword: mailbox.SMTPPassword != "",
		AuthType:        mailbox.AuthType,
		OAuthProvider:   mailbox.OAuthProvider,
		HasOAuth:        mailbox.OAuthRefreshToken != "",
		OAuthExpireTime: mailbox.OAuthExpireTime,
		Folder:          mailbox.Folder,
		SyncSent:        mailbox.SyncSent,
		SentFolder:      mailbox.SentFolder,
		SyncEnabled:     mailbox.SyncEnabled,
		SyncIntervalSec: mailbox.SyncIntervalSec,
		MarkSeen:        mailbox.MarkSeen,
		Aliases:         mailbox.Aliases,
		UIDValidity:     mailbox.UIDValidity,
		LastUID:         mailbox.LastUID,
		LastSyncTime:    mailbox.LastSyncTime,
		LastError:       mailbox.LastError,
		ReplyMode:       mailbox.ReplyMode,
		DelayMinutes:    mailbox.DelayMinutes,
		AutoAckEnabled:  mailbox.AutoAckEnabled,
		Language:        mailbox.Language,
		SignatureZh:     mailbox.SignatureZh,
		SignatureEn:     mailbox.SignatureEn,
		Templates:       mailbox.Templates,
		QuoteOriginal:   mailbox.QuoteOriginal,
		MaxSizeMB:       mailbox.MaxSizeMB,
		WorkTimeEnabled: mailbox.WorkTimeEnabled,
		WorkTimeStart:   mailbox.WorkTimeStart,
		WorkTimeEnd:     mailbox.WorkTimeEnd,
		WorkDays:        mailbox.WorkDays,
		Holidays:        mailbox.Holidays,
		Status:          mailbox.Status,
		CreateTime:      mailbox.CreateTime,
		UpdateTime:      mailbox.UpdateTime,
	}
}

// Test 测试 IMAP 与 SMTP 连接，不发测试邮件
func (s *KefuMailboxService) Test(
	ctx context.Context, channelID int64, form model.KefuMailboxTestForm,
) (*model.KefuMailTestResult, error) {
	mailbox, err := s.mailboxRepository.GetByChannelID(channelID)
	if err != nil {
		return nil, err
	}
	protocol := strings.ToLower(strings.TrimSpace(form.Protocol))
	result := new(model.KefuMailTestResult)

	if protocol == "" || protocol == "imap" {
		config, err := s.imapConfig(ctx, mailbox)
		if err != nil {
			result.Messages = append(result.Messages, "IMAP："+mailHint(mailbox, err))
		} else if err := mailclient.New(config).Test(); err != nil {
			result.Messages = append(result.Messages, "IMAP："+mailHint(mailbox, err))
		} else {
			result.IMAPOK = true
			result.Messages = append(result.Messages, "IMAP 连接成功")
		}
	}
	if protocol == "" || protocol == "smtp" {
		config, err := s.smtpConfig(ctx, mailbox)
		if err != nil {
			result.Messages = append(result.Messages, "SMTP："+mailHint(mailbox, err))
		} else {
			client, err := pkgmail.New(config)
			if err != nil {
				result.Messages = append(result.Messages, "SMTP："+mailHint(mailbox, err))
			} else if err := client.Test(); err != nil {
				result.Messages = append(result.Messages, "SMTP："+mailHint(mailbox, err))
			} else {
				result.SMTPOK = true
				result.Messages = append(result.Messages, "SMTP 连接成功")
			}
		}
	}
	if len(result.Messages) == 0 {
		return nil, errors.New("请指定要测试的协议")
	}
	return result, nil
}

// mailHint 把常见的认证失败翻译成可直接照做的中文提示，不泄露密码
func mailHint(mailbox *model.KefuMailbox, err error) string {
	message := err.Error()
	lower := strings.ToLower(message)
	if strings.Contains(lower, "unsafe login") {
		return "登录被拒绝，163 邮箱需要发送 IMAP ID 命令，请确认账号已开通 IMAP"
	}
	if strings.Contains(lower, "auth") || strings.Contains(lower, "login") ||
		strings.Contains(lower, "认证") {
		switch mailbox.Provider {
		case model.KefuMailProviderQQ:
			return "认证失败，QQ 邮箱请使用授权码，不是登录密码"
		case model.KefuMailProvider163:
			return "认证失败，163 邮箱请使用授权码，不是登录密码"
		case model.KefuMailProviderGmail:
			return "认证失败，Gmail 个人邮箱请使用应用专用密码，并确认已开启两步验证"
		case model.KefuMailProviderOutlook:
			return "认证失败，请重新完成 OAuth2 授权"
		}
		return "认证失败，请检查账号与密码或授权码"
	}
	return message
}

// imapConfig 组装 IMAP 收信配置，OAuth2 时自动刷新访问令牌
func (s *KefuMailboxService) imapConfig(
	ctx context.Context, mailbox *model.KefuMailbox,
) (mailclient.Config, error) {
	mailbox, err := s.ensureAccessToken(ctx, mailbox)
	if err != nil {
		return mailclient.Config{}, err
	}

	config := mailclient.Config{
		Host:     mailbox.IMAPHost,
		Port:     mailbox.IMAPPort,
		SSL:      mailbox.IMAPSSL == 1,
		Username: mailbox.IMAPUsername,
		AuthType: mailbox.AuthType,
		Provider: mailbox.Provider,
		Folder:   mailbox.Folder,
		MarkSeen: mailbox.MarkSeen == 1,
	}
	if mailbox.AuthType == model.KefuMailAuthOAuth2 {
		accessToken, err := encrypt.Decrypt(mailbox.OAuthAccessToken)
		if err != nil {
			return mailclient.Config{}, fmt.Errorf("OAuth2 访问令牌解密失败: %w", err)
		}
		if strings.TrimSpace(accessToken) == "" {
			return mailclient.Config{}, errors.New("邮箱未完成 OAuth2 授权")
		}
		config.AccessToken = accessToken
		return config, nil
	}
	password, err := encrypt.Decrypt(mailbox.IMAPPassword)
	if err != nil {
		return mailclient.Config{}, fmt.Errorf("IMAP 密码解密失败: %w", err)
	}
	if config.Username == "" || password == "" {
		return mailclient.Config{}, ErrMailCredentialsMissing
	}
	config.Password = password
	return config, nil
}

// smtpConfig 组装 SMTP 发信配置，OAuth2 时自动刷新访问令牌
func (s *KefuMailboxService) smtpConfig(
	ctx context.Context, mailbox *model.KefuMailbox,
) (pkgmail.Config, error) {
	mailbox, err := s.ensureAccessToken(ctx, mailbox)
	if err != nil {
		return pkgmail.Config{}, err
	}

	config := pkgmail.Config{
		Host:      mailbox.SMTPHost,
		Port:      mailbox.SMTPPort,
		Username:  mailbox.SMTPUsername,
		AuthType:  mailbox.AuthType,
		From:      mailbox.EmailAddress,
		FromName:  mailbox.DisplayName,
		SSL:       mailbox.SMTPSSL == 1,
		OAuthUser: mailbox.SMTPUsername,
	}
	if mailbox.AuthType == model.KefuMailAuthOAuth2 {
		accessToken, err := encrypt.Decrypt(mailbox.OAuthAccessToken)
		if err != nil {
			return pkgmail.Config{}, fmt.Errorf("OAuth2 访问令牌解密失败: %w", err)
		}
		if strings.TrimSpace(accessToken) == "" {
			return pkgmail.Config{}, errors.New("邮箱未完成 OAuth2 授权")
		}
		config.AccessToken = accessToken
		return config, nil
	}
	password, err := encrypt.Decrypt(mailbox.SMTPPassword)
	if err != nil {
		return pkgmail.Config{}, fmt.Errorf("SMTP 密码解密失败: %w", err)
	}
	if config.Username == "" || password == "" {
		return pkgmail.Config{}, ErrMailCredentialsMissing
	}
	config.Password = password
	return config, nil
}

// ensureAccessToken 在 OAuth2 模式下保证访问令牌可用，过期前自动刷新
func (s *KefuMailboxService) ensureAccessToken(
	ctx context.Context, mailbox *model.KefuMailbox,
) (*model.KefuMailbox, error) {
	if mailbox.AuthType != model.KefuMailAuthOAuth2 {
		return mailbox, nil
	}
	if mailbox.OAuthAccessToken != "" &&
		time.Time(mailbox.OAuthExpireTime.Time).After(time.Now().Add(mailTokenRefreshAhead)) {
		return mailbox, nil
	}
	return s.refreshAccessToken(ctx, mailbox)
}

// defaultMailReplyMode 返回新建邮箱的默认回复模式
func defaultMailReplyMode() string {
	mode := strings.TrimSpace(setting.GetString(configKeyMailDefaultReplyMode,
		model.KefuMailReplyModeDraft))
	switch mode {
	case model.KefuMailReplyModeOff, model.KefuMailReplyModeDraft,
		model.KefuMailReplyModeDelay, model.KefuMailReplyModeAuto:
		return mode
	default:
		return model.KefuMailReplyModeDraft
	}
}

// mailEnabled 返回邮箱渠道总开关
func mailEnabled() bool {
	return setting.GetBool(configKeyMailEnabled, true)
}

// mailMaxSizeMB 返回单封邮件大小上限
func mailMaxSizeMB() int {
	value := setting.GetInt(configKeyMailMaxSizeMB, defaultMailMaxSizeMB)
	if value <= 0 {
		return defaultMailMaxSizeMB
	}
	return value
}

// mailboxMaxSizeMB 返回邮箱配置的单封上限，0 表示用全局默认
func mailboxMaxSizeMB(mailbox *model.KefuMailbox) int {
	if mailbox != nil && mailbox.MaxSizeMB > 0 {
		return mailbox.MaxSizeMB
	}
	return mailMaxSizeMB()
}

// mailboxLanguage 返回邮箱配置的回复语言
func mailboxLanguage(mailbox *model.KefuMailbox) string {
	if mailbox == nil || mailbox.Language == "" {
		return model.KefuMailLanguageAuto
	}
	return mailbox.Language
}

// boolToInt 把布尔值转成 0/1
func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
