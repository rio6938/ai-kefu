package sms

import (
	"encoding/json"
	"strings"

	"go.uber.org/zap"

	"admin/server/pkg/log"
	"admin/server/pkg/setting"
)

// 短信配置在配置中心里的键
const (
	keyEnabled         = "sms.enabled"
	keyProvider        = "sms.provider"
	keyAccessKeyID     = "sms.access_key_id"
	keyAccessKeySecret = "sms.access_key_secret"
	keyRegion          = "sms.region"
	keyEndpoint        = "sms.endpoint"
	keySignName        = "sms.sign_name"
	keySDKAppID        = "sms.sdk_app_id"
	keyTemplates       = "sms.templates"
	keyCodeLength      = "sms.code_length"
	keyExpire          = "sms.expire"
	keyInterval        = "sms.interval"
	keyHourlyLimit     = "sms.hourly_limit"
	keyDailyLimit      = "sms.daily_limit"
	keyMaxAttempts     = "sms.max_attempts"
	keyTimeout         = "sms.timeout"
)

// defaultAccountName 是配置中心只维护一个短信账号时使用的账号名
const defaultAccountName = "default"

// Config 是短信服务配置，由配置中心的配置项组装而成
type Config struct {
	// Enabled 是是否启用短信服务
	Enabled bool
	// CodeLength 是验证码位数
	CodeLength int
	// Expire 是验证码有效期，秒
	Expire int
	// Interval 是同手机号同场景重发间隔，秒
	Interval int
	// HourlyLimit 是同手机号每小时发送上限
	HourlyLimit int
	// DailyLimit 是同手机号自然日发送上限
	DailyLimit int
	// MaxAttempts 是验证码最大校验失败次数
	MaxAttempts int
	// Timeout 是单次调用超时，秒
	Timeout int
	// Default 是默认账号名
	Default string
	// Accounts 是短信账号，键为账号名
	Accounts map[string]Account
}

// Account 是一个短信账号的配置，Provider 决定使用哪家厂商的驱动
type Account struct {
	// Provider 是厂商，支持 aliyun / tencent
	Provider string
	// AccessKeyID 是访问密钥 ID，腾讯云为 SecretId
	AccessKeyID string
	// AccessKeySecret 是访问密钥，腾讯云为 SecretKey
	AccessKeySecret string
	// Region 是地域，为空时用厂商默认值
	Region string
	// Endpoint 是接口地址，为空时用厂商默认值
	Endpoint string
	// SignName 是短信签名
	SignName string
	// SDKAppID 是腾讯云短信应用 ID
	SDKAppID string
	// Templates 是场景到模板 ID 的映射，如 code / reset_password
	Templates map[string]string
}

// loadConfig 从配置中心读取短信配置，未配置的项使用默认值
func loadConfig() Config {
	return Config{
		Enabled:     setting.GetBool(keyEnabled, false),
		CodeLength:  setting.GetInt(keyCodeLength, 6),
		Expire:      setting.GetInt(keyExpire, 300),
		Interval:    setting.GetInt(keyInterval, 60),
		HourlyLimit: setting.GetInt(keyHourlyLimit, 10),
		DailyLimit:  setting.GetInt(keyDailyLimit, 20),
		MaxAttempts: setting.GetInt(keyMaxAttempts, 5),
		Timeout:     setting.GetInt(keyTimeout, 5),
		Default:     defaultAccountName,
		Accounts: map[string]Account{
			defaultAccountName: {
				Provider:        setting.GetString(keyProvider, ""),
				AccessKeyID:     setting.GetString(keyAccessKeyID, ""),
				AccessKeySecret: setting.GetString(keyAccessKeySecret, ""),
				Region:          setting.GetString(keyRegion, ""),
				Endpoint:        setting.GetString(keyEndpoint, ""),
				SignName:        setting.GetString(keySignName, ""),
				SDKAppID:        setting.GetString(keySDKAppID, ""),
				Templates:       loadTemplates(),
			},
		},
	}
}

// loadTemplates 解析场景到模板 ID 的 json 配置，格式不正确时按未配置处理
func loadTemplates() map[string]string {
	templates := make(map[string]string)
	raw := strings.TrimSpace(setting.GetString(keyTemplates, ""))
	if raw == "" {
		return templates
	}
	if err := json.Unmarshal([]byte(raw), &templates); err != nil {
		log.Logger.Warn("短信模板配置不是合法的 json",
			zap.String("value", raw), zap.Error(err))
		return map[string]string{}
	}
	return templates
}
