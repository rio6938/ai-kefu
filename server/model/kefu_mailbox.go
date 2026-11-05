package model

// 邮箱服务商
const (
	// KefuMailProviderGmail 是 Gmail 个人邮箱
	KefuMailProviderGmail = "gmail"
	// KefuMailProviderQQ 是 QQ 邮箱
	KefuMailProviderQQ = "qq"
	// KefuMailProvider163 是 163 邮箱
	KefuMailProvider163 = "163"
	// KefuMailProviderOutlook 是 Outlook 个人邮箱
	KefuMailProviderOutlook = "outlook"
	// KefuMailProviderCustom 是自定义邮箱
	KefuMailProviderCustom = "custom"
)

// 邮箱认证方式
const (
	// KefuMailAuthPassword 是密码或授权码
	KefuMailAuthPassword = "password"
	// KefuMailAuthOAuth2 是 OAuth2
	KefuMailAuthOAuth2 = "oauth2"
)

// 自动回复模式
const (
	// KefuMailReplyModeOff 是只收不回
	KefuMailReplyModeOff = "off"
	// KefuMailReplyModeDraft 是仅生成草稿，人工确认后发送
	KefuMailReplyModeDraft = "draft"
	// KefuMailReplyModeDelay 是延时自动发送
	KefuMailReplyModeDelay = "delay"
	// KefuMailReplyModeAuto 是立即自动发送
	KefuMailReplyModeAuto = "auto"
)

// 邮箱回复语言
const (
	// KefuMailLanguageAuto 是按来信内容自动判断
	KefuMailLanguageAuto = "auto"
	// KefuMailLanguageZh 是中文
	KefuMailLanguageZh = "zh"
	// KefuMailLanguageEn 是英文
	KefuMailLanguageEn = "en"
)

// 邮箱状态
const (
	// KefuMailboxStatusNormal 是正常
	KefuMailboxStatusNormal = 0
	// KefuMailboxStatusDisabled 是停用
	KefuMailboxStatusDisabled = 1
)

// 邮箱翻译字典标识
const (
	// KefuMailProviderDictCode 是邮箱服务商字典标识
	KefuMailProviderDictCode = "kefu_mail_provider"
	// KefuMailReplyModeDictCode 是自动回复模式字典标识
	KefuMailReplyModeDictCode = "kefu_mail_reply_mode"
	// KefuMailAuthTypeDictCode 是邮箱认证方式字典标识
	KefuMailAuthTypeDictCode = "kefu_mail_auth_type"
	// KefuMailSkipReasonDictCode 是邮件跳过原因字典标识
	KefuMailSkipReasonDictCode = "kefu_mail_skip_reason"
)

// KefuMailTemplatePair 是一类模板的中英两版
type KefuMailTemplatePair struct {
	// 中文模板
	Zh string `json:"zh"`
	// 英文模板
	En string `json:"en"`
}

// KefuMailTemplates 是邮箱级的邮件模板，存 kefu_mailbox.templates 的 JSON 文本
type KefuMailTemplates struct {
	// 首次来信自动确认
	Ack KefuMailTemplatePair `json:"ack"`
	// 非工作时间提示
	OffHours KefuMailTemplatePair `json:"offHours"`
	// 转人工告知
	Handoff KefuMailTemplatePair `json:"handoff"`
	// 会话关闭告知
	Closed KefuMailTemplatePair `json:"closed"`
}

// KefuMailbox 是邮箱配置，对应 kefu_mailbox 表；
// 一个渠道只绑定一个邮箱，凭据字段在 service 层加解密，接口只返回掩码
type KefuMailbox struct {
	// 邮箱配置id
	MailboxID int64 `gorm:"column:mailbox_id;primaryKey;autoIncrement;comment:邮箱配置id" json:"mailboxId"`
	// 关联渠道id, 一个渠道一个邮箱
	ChannelID int64 `gorm:"column:channel_id;not null;uniqueIndex;comment:关联渠道id" json:"channelId"`
	// 服务商, gmail/qq/163/outlook/custom
	Provider string `gorm:"column:provider;size:32;not null;default:custom;comment:服务商" json:"provider"`
	// 收件地址, 全局唯一
	EmailAddress string `gorm:"column:email_address;size:128;not null;uniqueIndex;comment:收件地址" json:"emailAddress"`
	// 发件人显示名
	DisplayName string `gorm:"column:display_name;size:128;comment:发件人显示名" json:"displayName"`
	// IMAP 服务器地址
	IMAPHost string `gorm:"column:imap_host;size:128;comment:IMAP服务器地址" json:"imapHost"`
	// IMAP 端口
	IMAPPort int `gorm:"column:imap_port;not null;default:993;comment:IMAP端口" json:"imapPort"`
	// IMAP 是否直连 SSL
	IMAPSSL int `gorm:"column:imap_ssl;not null;default:1;comment:IMAP是否直连SSL" json:"imapSsl"`
	// IMAP 账号
	IMAPUsername string `gorm:"column:imap_username;size:128;comment:IMAP账号" json:"imapUsername"`
	// IMAP 密码或授权码, 加密存储
	IMAPPassword string `gorm:"column:imap_password;size:512;comment:IMAP密码或授权码" json:"-"`
	// SMTP 服务器地址
	SMTPHost string `gorm:"column:smtp_host;size:128;comment:SMTP服务器地址" json:"smtpHost"`
	// SMTP 端口
	SMTPPort int `gorm:"column:smtp_port;not null;default:465;comment:SMTP端口" json:"smtpPort"`
	// SMTP 是否直连 SSL
	SMTPSSL int `gorm:"column:smtp_ssl;not null;default:1;comment:SMTP是否直连SSL" json:"smtpSsl"`
	// SMTP 账号
	SMTPUsername string `gorm:"column:smtp_username;size:128;comment:SMTP账号" json:"smtpUsername"`
	// SMTP 密码或授权码, 加密存储
	SMTPPassword string `gorm:"column:smtp_password;size:512;comment:SMTP密码或授权码" json:"-"`
	// 认证方式, password/oauth2
	AuthType string `gorm:"column:auth_type;size:16;not null;default:password;comment:认证方式" json:"authType"`
	// OAuth 服务商, google/microsoft
	OAuthProvider string `gorm:"column:oauth_provider;size:16;comment:OAuth服务商" json:"oauthProvider"`
	// OAuth 刷新令牌, 加密存储
	OAuthRefreshToken string `gorm:"column:oauth_refresh_token;size:1024;comment:OAuth刷新令牌" json:"-"`
	// OAuth 访问令牌, 加密存储
	OAuthAccessToken string `gorm:"column:oauth_access_token;size:2048;comment:OAuth访问令牌" json:"-"`
	// OAuth 访问令牌过期时间
	OAuthExpireTime Time `gorm:"column:oauth_expire_time;comment:OAuth访问令牌过期时间" json:"oauthExpireTime"`
	// 收信文件夹
	Folder string `gorm:"column:folder;size:128;not null;default:INBOX;comment:收信文件夹" json:"folder"`
	// 是否同步已发送, 0否, 1是
	SyncSent int `gorm:"column:sync_sent;not null;default:0;comment:是否同步已发送" json:"syncSent"`
	// 已发送文件夹
	SentFolder string `gorm:"column:sent_folder;size:128;not null;default:Sent;comment:已发送文件夹" json:"sentFolder"`
	// 是否自动同步, 0否, 1是
	SyncEnabled int `gorm:"column:sync_enabled;not null;default:1;comment:是否自动同步" json:"syncEnabled"`
	// 同步间隔, 秒, 0 用全局默认
	SyncIntervalSec int `gorm:"column:sync_interval_sec;not null;default:0;comment:同步间隔, 秒" json:"syncIntervalSec"`
	// 拉取后是否标记已读, 0否, 1是
	MarkSeen int `gorm:"column:mark_seen;not null;default:0;comment:拉取后是否标记已读" json:"markSeen"`
	// 收信别名, 英文逗号分隔, 与 user+tag@domain 一起作为同一邮箱的收信地址
	Aliases string `gorm:"column:aliases;size:512;comment:收信别名" json:"aliases"`
	// IMAP 游标: UIDVALIDITY
	UIDValidity int64 `gorm:"column:uid_validity;not null;default:0;comment:IMAP游标UIDVALIDITY" json:"uidValidity"`
	// IMAP 游标: 已处理的最大 UID
	LastUID int64 `gorm:"column:last_uid;not null;default:0;comment:IMAP游标已处理的最大UID" json:"lastUid"`
	// 最近同步时间
	LastSyncTime Time `gorm:"column:last_sync_time;comment:最近同步时间" json:"lastSyncTime"`
	// 最近同步错误
	LastError string `gorm:"column:last_error;size:512;comment:最近同步错误" json:"lastError"`
	// 自动回复模式, off/draft/delay/auto
	ReplyMode string `gorm:"column:reply_mode;size:16;not null;default:draft;comment:自动回复模式" json:"replyMode"`
	// 延时自动发送的分钟数
	DelayMinutes int `gorm:"column:delay_minutes;not null;default:15;comment:延时自动发送的分钟数" json:"delayMinutes"`
	// 新线程是否自动确认, 0否, 1是
	AutoAckEnabled int `gorm:"column:auto_ack_enabled;not null;default:1;comment:新线程是否自动确认" json:"autoAckEnabled"`
	// 回复语言, auto/zh/en
	Language string `gorm:"column:language;size:16;not null;default:auto;comment:回复语言" json:"language"`
	// 中文签名
	SignatureZh string `gorm:"column:signature_zh;type:text;comment:中文签名" json:"signatureZh"`
	// 英文签名
	SignatureEn string `gorm:"column:signature_en;type:text;comment:英文签名" json:"signatureEn"`
	// 邮件模板, JSON 文本
	Templates string `gorm:"column:templates;type:text;comment:邮件模板" json:"templates"`
	// 回复是否引用原文, 0否, 1是
	QuoteOriginal int `gorm:"column:quote_original;not null;default:1;comment:回复是否引用原文" json:"quoteOriginal"`
	// 单封上限, MB, 0 用全局默认
	MaxSizeMB int `gorm:"column:max_size_mb;not null;default:0;comment:单封上限, MB" json:"maxSizeMb"`
	// 工作时间开关, 0否, 1是
	WorkTimeEnabled int `gorm:"column:work_time_enabled;not null;default:0;comment:工作时间开关" json:"workTimeEnabled"`
	// 工作时间开始, HH:MM
	WorkTimeStart string `gorm:"column:work_time_start;size:8;comment:工作时间开始" json:"workTimeStart"`
	// 工作时间结束, HH:MM
	WorkTimeEnd string `gorm:"column:work_time_end;size:8;comment:工作时间结束" json:"workTimeEnd"`
	// 工作日的星期, 1-7 英文逗号分隔
	WorkDays string `gorm:"column:work_days;size:32;comment:工作日的星期" json:"workDays"`
	// 节假日日期, yyyy-MM-dd 英文逗号分隔
	Holidays string `gorm:"column:holidays;size:512;comment:节假日日期" json:"holidays"`
	// 状态, 0正常, 1停用
	Status int `gorm:"column:status;not null;default:0;comment:状态, 0正常, 1停用" json:"status"`
	// 是否删除, 0否, 1是
	Deleted int `gorm:"column:deleted;not null;default:0;comment:是否删除" json:"deleted"`
	// 创建时间
	CreateTime Time `gorm:"column:create_time;comment:创建时间" json:"createTime"`
	// 更新时间
	UpdateTime Time `gorm:"column:update_time;comment:更新时间" json:"updateTime"`
}

// TableName 指定表名
func (KefuMailbox) TableName() string {
	return "kefu_mailbox"
}

// KefuMailboxForm 是邮箱配置的提交数据；密码字段为空表示保持原值
type KefuMailboxForm struct {
	// 渠道id
	ChannelID int64 `json:"channelId"`
	// 服务商
	Provider string `json:"provider"`
	// 收件地址
	EmailAddress string `json:"emailAddress"`
	// 发件人显示名
	DisplayName string `json:"displayName"`
	// IMAP 服务器地址
	IMAPHost string `json:"imapHost"`
	// IMAP 端口
	IMAPPort int `json:"imapPort"`
	// IMAP 是否直连 SSL
	IMAPSSL int `json:"imapSsl"`
	// IMAP 账号
	IMAPUsername string `json:"imapUsername"`
	// IMAP 密码或授权码, 为空表示不修改
	IMAPPassword string `json:"imapPassword"`
	// SMTP 服务器地址
	SMTPHost string `json:"smtpHost"`
	// SMTP 端口
	SMTPPort int `json:"smtpPort"`
	// SMTP 是否直连 SSL
	SMTPSSL int `json:"smtpSsl"`
	// SMTP 账号
	SMTPUsername string `json:"smtpUsername"`
	// SMTP 密码或授权码, 为空表示不修改
	SMTPPassword string `json:"smtpPassword"`
	// 认证方式
	AuthType string `json:"authType"`
	// OAuth 服务商
	OAuthProvider string `json:"oauthProvider"`
	// 收信文件夹
	Folder string `json:"folder"`
	// 是否同步已发送
	SyncSent int `json:"syncSent"`
	// 已发送文件夹
	SentFolder string `json:"sentFolder"`
	// 是否自动同步
	SyncEnabled int `json:"syncEnabled"`
	// 同步间隔, 秒
	SyncIntervalSec int `json:"syncIntervalSec"`
	// 拉取后是否标记已读
	MarkSeen int `json:"markSeen"`
	// 收信别名
	Aliases string `json:"aliases"`
	// 自动回复模式
	ReplyMode string `json:"replyMode"`
	// 延时自动发送的分钟数
	DelayMinutes int `json:"delayMinutes"`
	// 新线程是否自动确认
	AutoAckEnabled int `json:"autoAckEnabled"`
	// 回复语言
	Language string `json:"language"`
	// 中文签名
	SignatureZh string `json:"signatureZh"`
	// 英文签名
	SignatureEn string `json:"signatureEn"`
	// 邮件模板
	Templates string `json:"templates"`
	// 回复是否引用原文
	QuoteOriginal int `json:"quoteOriginal"`
	// 单封上限, MB
	MaxSizeMB int `json:"maxSizeMb"`
	// 工作时间开关
	WorkTimeEnabled int `json:"workTimeEnabled"`
	// 工作时间开始
	WorkTimeStart string `json:"workTimeStart"`
	// 工作时间结束
	WorkTimeEnd string `json:"workTimeEnd"`
	// 工作日的星期
	WorkDays string `json:"workDays"`
	// 节假日日期
	Holidays string `json:"holidays"`
	// 状态
	Status int `json:"status"`
}

// KefuMailboxTestForm 是邮箱连接测试的提交数据，protocol 为空表示两项都测
type KefuMailboxTestForm struct {
	// 测试协议, imap/smtp, 为空表示全部
	Protocol string `json:"protocol"`
}

// KefuMailboxResult 是邮箱配置的返回结构，凭据只返回是否已配置
type KefuMailboxResult struct {
	// 邮箱配置id
	MailboxID int64 `json:"mailboxId"`
	// 关联渠道id
	ChannelID int64 `json:"channelId"`
	// 服务商
	Provider string `json:"provider"`
	// 收件地址
	EmailAddress string `json:"emailAddress"`
	// 发件人显示名
	DisplayName string `json:"displayName"`
	// IMAP 服务器地址
	IMAPHost string `json:"imapHost"`
	// IMAP 端口
	IMAPPort int `json:"imapPort"`
	// IMAP 是否直连 SSL
	IMAPSSL int `json:"imapSsl"`
	// IMAP 账号
	IMAPUsername string `json:"imapUsername"`
	// 是否已配置 IMAP 密码
	HasIMAPPassword bool `json:"hasImapPassword"`
	// SMTP 服务器地址
	SMTPHost string `json:"smtpHost"`
	// SMTP 端口
	SMTPPort int `json:"smtpPort"`
	// SMTP 是否直连 SSL
	SMTPSSL int `json:"smtpSsl"`
	// SMTP 账号
	SMTPUsername string `json:"smtpUsername"`
	// 是否已配置 SMTP 密码
	HasSMTPPassword bool `json:"hasSmtpPassword"`
	// 认证方式
	AuthType string `json:"authType"`
	// OAuth 服务商
	OAuthProvider string `json:"oauthProvider"`
	// 是否已授权 OAuth
	HasOAuth bool `json:"hasOauth"`
	// OAuth 访问令牌过期时间
	OAuthExpireTime Time `json:"oauthExpireTime"`
	// 收信文件夹
	Folder string `json:"folder"`
	// 是否同步已发送
	SyncSent int `json:"syncSent"`
	// 已发送文件夹
	SentFolder string `json:"sentFolder"`
	// 是否自动同步
	SyncEnabled int `json:"syncEnabled"`
	// 同步间隔, 秒
	SyncIntervalSec int `json:"syncIntervalSec"`
	// 拉取后是否标记已读
	MarkSeen int `json:"markSeen"`
	// 收信别名
	Aliases string `json:"aliases"`
	// IMAP 游标: UIDVALIDITY
	UIDValidity int64 `json:"uidValidity"`
	// IMAP 游标: 已处理的最大 UID
	LastUID int64 `json:"lastUid"`
	// 最近同步时间
	LastSyncTime Time `json:"lastSyncTime"`
	// 最近同步错误
	LastError string `json:"lastError"`
	// 自动回复模式
	ReplyMode string `json:"replyMode"`
	// 延时自动发送的分钟数
	DelayMinutes int `json:"delayMinutes"`
	// 新线程是否自动确认
	AutoAckEnabled int `json:"autoAckEnabled"`
	// 回复语言
	Language string `json:"language"`
	// 中文签名
	SignatureZh string `json:"signatureZh"`
	// 英文签名
	SignatureEn string `json:"signatureEn"`
	// 邮件模板
	Templates string `json:"templates"`
	// 回复是否引用原文
	QuoteOriginal int `json:"quoteOriginal"`
	// 单封上限, MB
	MaxSizeMB int `json:"maxSizeMb"`
	// 工作时间开关
	WorkTimeEnabled int `json:"workTimeEnabled"`
	// 工作时间开始
	WorkTimeStart string `json:"workTimeStart"`
	// 工作时间结束
	WorkTimeEnd string `json:"workTimeEnd"`
	// 工作日的星期
	WorkDays string `json:"workDays"`
	// 节假日日期
	Holidays string `json:"holidays"`
	// 状态
	Status int `json:"status"`
	// 创建时间
	CreateTime Time `json:"createTime"`
	// 更新时间
	UpdateTime Time `json:"updateTime"`
}

// KefuMailProviderPreset 是服务商预设，后台选中服务商后自动带出
type KefuMailProviderPreset struct {
	// 服务商标识
	Provider string `json:"provider"`
	// 服务商名称
	Name string `json:"name"`
	// IMAP 服务器地址
	IMAPHost string `json:"imapHost"`
	// IMAP 端口
	IMAPPort int `json:"imapPort"`
	// IMAP 是否直连 SSL
	IMAPSSL int `json:"imapSsl"`
	// SMTP 服务器地址
	SMTPHost string `json:"smtpHost"`
	// SMTP 端口
	SMTPPort int `json:"smtpPort"`
	// SMTP 是否直连 SSL
	SMTPSSL int `json:"smtpSsl"`
	// 默认收信文件夹
	Folder string `json:"folder"`
	// 默认认证方式
	AuthType string `json:"authType"`
	// 是否依赖 OAuth2 授权
	NeedOAuth bool `json:"needOauth"`
	// 是否已开放, 未开放的服务商后台置灰
	Enabled bool `json:"enabled"`
	// 使用说明
	Tip string `json:"tip"`
}

// KefuMailSyncResult 是立即同步一次的返回
type KefuMailSyncResult struct {
	// 本次拉取封数
	Fetched int `json:"fetched"`
	// 本次新增会话消息封数
	Received int `json:"received"`
	// 本次跳过的封数
	Skipped int `json:"skipped"`
	// 本次失败的封数
	Failed int `json:"failed"`
}
