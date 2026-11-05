package config

import (
	"os"

	"github.com/pelletier/go-toml/v2"
)

// DefaultPath 是 -c 未指定时使用的配置文件路径
const DefaultPath = "config.toml"

// Config 对应 TOML 配置文件的结构，只包含启动阶段必须的引导配置，
// 短信、邮件等业务配置存放在数据库配置中心，由配置中心页面维护
type Config struct {
	Server   Server   `toml:"server"`
	Log      Log      `toml:"log"`
	DB       DB       `toml:"db"`
	Redis    Redis    `toml:"redis"`
	JWT      JWT      `toml:"jwt"`
	Captcha  Captcha  `toml:"captcha"`
	Security Security `toml:"security"`
	WuKongIM WuKongIM `toml:"wukongim"`
	Qdrant   Qdrant   `toml:"qdrant"`
	Cors     Cors     `toml:"cors"`
}

type Server struct {
	Addr string `toml:"addr"`
	Mode string `toml:"mode"`
}

// Log 是日志配置
type Log struct {
	Level        string `toml:"level"`         // 写文件的日志级别
	ConsoleLevel string `toml:"console_level"` // 输出到控制台的日志级别
	SQLLevel     string `toml:"sql_level"`     // SQL 日志级别，silent 为关闭
	Path         string `toml:"path"`          // 日志目录，为空时只输出到控制台
	MaxSize      int    `toml:"max_size"`      // 单个日志文件的大小上限，单位 MB
	MaxAge       int    `toml:"max_age"`       // 日志文件的保留天数
}

type DB struct {
	DSN         string `toml:"dsn"`
	MaxIdle     int    `toml:"max_idle"`
	MaxOpen     int    `toml:"max_open"`
	MaxLifetime int    `toml:"max_lifetime"` // 秒
}

// Redis 是 redis 连接配置
type Redis struct {
	Addr     string `toml:"addr"`
	Password string `toml:"password"`
	DB       int    `toml:"db"`
}

// JWT 是登录 token 的签发配置
type JWT struct {
	Secret string `toml:"secret"`
	// UserSecret 是前台用户 token 的密钥，为空时用 Secret 拼接默认值
	UserSecret string `toml:"user_secret"`
	Expire     int    `toml:"expire"` // 秒
}

// Captcha 是图形验证码配置
type Captcha struct {
	Length int `toml:"length"` // 字符个数
	Expire int `toml:"expire"` // 有效期，秒
	Width  int `toml:"width"`  // 图片宽度，像素
	Height int `toml:"height"` // 图片高度，像素
}

// Security 是配置中心里密钥类配置的加密配置
type Security struct {
	// ConfigKey 是加密主密钥，支持 32 字节的 base64 或任意口令（内部按 sha256 派生）；
	// 为空时无法保存密钥类配置，更换后已经保存的密钥类配置将无法解密
	ConfigKey string `toml:"config_key"`
}

// WuKongIM 是 WuKongIM 服务端的接入配置
// （HTTP API 只在内网暴露，浏览器只拿短期 token 与 gateway 地址）
type WuKongIM struct {
	// ApiURL 是服务端 HTTP API 地址
	ApiURL string `toml:"api_url"`
	// WsURL 是客户端连接 gateway 的地址，登录时下发给前端
	WsURL string `toml:"ws_url"`
	// WebhookToken 是校验 IM 回调的共享密钥
	WebhookToken string `toml:"webhook_token"`
	// AiUID 是 AI 服务号的 UID
	AiUID string `toml:"ai_uid"`
}

// Qdrant 是向量库的连接配置
type Qdrant struct {
	// URL 是向量库地址，支持 http / https，默认使用 gRPC 端口 6334
	URL string `toml:"url"`
	// ApiKey 是向量库的访问密钥，为空表示未开启鉴权
	ApiKey string `toml:"api_key"`
	// CollectionPrefix 是集合名前缀，为空时使用 kefu
	CollectionPrefix string `toml:"collection_prefix"`
	// VectorSize 是向量维度，换 embedding 模型时需要同步修改并重建集合
	VectorSize int `toml:"vector_size"`
}

// Cors 是跨域配置，只对 /kefu 与 /v1 公开路由生效，/admin 一律不开放跨域
type Cors struct {
	// AllowOrigins 是允许的来源，为空表示不启用跨域（同源反代场景无需跨域）
	AllowOrigins []string `toml:"allow_origins"`
	// AllowCredentials 标识是否允许携带 cookie
	AllowCredentials bool `toml:"allow_credentials"`
}

// defaultConfig 返回内置默认配置，Load 读取的配置会覆盖其中的字段
func defaultConfig() *Config {
	return &Config{
		Server: Server{Addr: ":8080", Mode: "debug"},
		Log: Log{
			Level:        "info",
			ConsoleLevel: "info",
			SQLLevel:     "debug",
			MaxSize:      20,
			MaxAge:       30,
		},
		DB:      DB{MaxIdle: 10, MaxOpen: 100, MaxLifetime: 3600},
		Redis:   Redis{Addr: "127.0.0.1:6379"},
		JWT:     JWT{Expire: 7200},
		Captcha: Captcha{Length: 4, Expire: 300, Width: 130, Height: 48},
		WuKongIM: WuKongIM{
			ApiURL: "http://127.0.0.1:5001",
			WsURL:  "ws://127.0.0.1:5200",
			AiUID:  "ai_service",
		},
		Qdrant: Qdrant{
			URL:              "http://127.0.0.1:6334",
			CollectionPrefix: "kefu",
			VectorSize:       1536,
		},
	}
}

// Load 读取并解析 TOML 配置文件，未配置的字段保留默认值
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := defaultConfig()
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	// 前台用户 token 的密钥未配置时根据 secret 派生，保证与后台 token 不通用
	if cfg.JWT.UserSecret == "" {
		cfg.JWT.UserSecret = cfg.JWT.Secret + "-user"
	}
	return cfg, nil
}
