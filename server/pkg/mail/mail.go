package mail

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	netmail "net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// defaultTimeout 是 Timeout 未配置时的默认发送超时
const defaultTimeout = 10 * time.Second

// 认证方式
const (
	// AuthTypePassword 是密码或授权码
	AuthTypePassword = "password"
	// AuthTypeOAuth2 是 XOAUTH2
	AuthTypeOAuth2 = "oauth2"
)

// Config 是 SMTP 发件配置，由调用方传入，不读取配置文件
type Config struct {
	Host        string        // SMTP 服务器地址
	Port        int           // SMTP 端口，SSL 一般 465，STARTTLS 一般 587
	Username    string        // 登录账号，为空时不做认证
	Password    string        // 登录密码或授权码
	AuthType    string        // 认证方式，password / oauth2，为空按 password
	AccessToken string        // OAuth2 访问令牌
	OAuthUser   string        // OAuth2 登录账号，为空时用 Username
	From        string        // 发件人地址，为空时用 Username
	FromName    string        // 发件人显示名
	SSL         bool          // true 直连 TLS（465），false 用 STARTTLS
	Timeout     time.Duration // 单次发送超时，0 时用默认 10 秒
}

// Client 是邮件发送客户端，创建后配置只读，可被多个 goroutine 并发使用
type Client struct {
	cfg Config
}

// New 校验并保存发件配置，不建立连接；同一配置复用同一个 Client 即可
func New(cfg Config) (*Client, error) {
	if cfg.Host == "" {
		return nil, errors.New("SMTP 服务器地址不能为空")
	}
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return nil, errors.New("SMTP 端口不正确")
	}
	if cfg.From == "" {
		cfg.From = cfg.Username
	}
	if cfg.From == "" {
		return nil, errors.New("发件人地址不能为空")
	}
	parsed, err := netmail.ParseAddress(cfg.From)
	if err != nil {
		return nil, fmt.Errorf("发件人地址 %q 不正确: %w", cfg.From, err)
	}
	// 统一存纯地址，显示名只用 FromName，避免 MAIL FROM 带上显示名
	cfg.From = parsed.Address
	if cfg.AuthType == "" {
		cfg.AuthType = AuthTypePassword
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultTimeout
	}
	return &Client{cfg: cfg}, nil
}

// Send 发送一封邮件，每次调用使用独立连接，可并发调用
func (c *Client) Send(msg Message) error {
	_, err := c.SendMessage(msg)
	return err
}

// SendMessage 发送一封邮件并返回实际使用的 Message-ID，
// 出站记录需要把 Message-ID 回填到 kefu_mail_log
func (c *Client) SendMessage(msg Message) (string, error) {
	to, err := validRecipients(msg.To)
	if err != nil {
		return "", err
	}
	body, messageID := buildMessageWithID(c.cfg, msg, to)

	client, err := c.dial()
	if err != nil {
		return "", err
	}
	defer func() {
		_ = client.Close()
	}()

	if err := c.authenticate(client); err != nil {
		return "", err
	}
	if err := client.Mail(c.cfg.From); err != nil {
		return "", fmt.Errorf("设置发件人失败: %w", err)
	}
	for _, addr := range to {
		if err := client.Rcpt(addr); err != nil {
			return "", fmt.Errorf("设置收件人 %q 失败: %w", addr, err)
		}
	}
	for _, addr := range msg.Cc {
		parsed, err := netmail.ParseAddress(addr)
		if err != nil {
			return "", fmt.Errorf("抄送地址 %q 不正确: %w", addr, err)
		}
		if err := client.Rcpt(parsed.Address); err != nil {
			return "", fmt.Errorf("设置抄送 %q 失败: %w", parsed.Address, err)
		}
	}

	writer, err := client.Data()
	if err != nil {
		return "", fmt.Errorf("准备邮件内容失败: %w", err)
	}
	if _, err := writer.Write(body); err != nil {
		_ = writer.Close()
		return "", fmt.Errorf("写入邮件内容失败: %w", err)
	}
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("提交邮件内容失败: %w", err)
	}
	// 邮件已提交，退出会话失败不再向上报错，避免调用方重试导致重复发送
	_ = client.Quit()
	return messageID, nil
}

// Test 只做连接与认证测试，不发送任何邮件，避免打扰客户
func (c *Client) Test() error {
	client, err := c.dial()
	if err != nil {
		return err
	}
	defer func() {
		_ = client.Close()
	}()
	if err := c.authenticate(client); err != nil {
		return err
	}
	_ = client.Quit()
	return nil
}

// authenticate 按认证方式登录 SMTP，OAuth2 使用 XOAUTH2
func (c *Client) authenticate(client *smtp.Client) error {
	if c.cfg.AuthType == AuthTypeOAuth2 {
		user := c.cfg.OAuthUser
		if user == "" {
			user = c.cfg.Username
		}
		if user == "" {
			return errors.New("OAuth2 认证需要邮箱账号")
		}
		if strings.TrimSpace(c.cfg.AccessToken) == "" {
			return errors.New("OAuth2 访问令牌为空，请先完成授权")
		}
		if err := client.Auth(&xoauth2Auth{user: user, token: c.cfg.AccessToken}); err != nil {
			return fmt.Errorf("SMTP 认证失败: %w", err)
		}
		return nil
	}
	if c.cfg.Username != "" {
		auth := smtp.PlainAuth("", c.cfg.Username, c.cfg.Password, c.cfg.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP 认证失败: %w", err)
		}
	}
	return nil
}

// dial 建立一次 SMTP 会话，SSL 直连或 STARTTLS 升级，连接与读写都带超时
func (c *Client) dial() (*smtp.Client, error) {
	addr := net.JoinHostPort(c.cfg.Host, strconv.Itoa(c.cfg.Port))
	dialer := &net.Dialer{Timeout: c.cfg.Timeout}

	if c.cfg.SSL {
		conn, err := tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{ServerName: c.cfg.Host})
		if err != nil {
			return nil, fmt.Errorf("连接 SMTP 服务器失败: %w", err)
		}
		return c.newClient(conn)
	}

	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("连接 SMTP 服务器失败: %w", err)
	}
	client, err := c.newClient(conn)
	if err != nil {
		return nil, err
	}
	// 未直连 TLS 时，服务器支持就升级到 TLS，明文内网中继也能用
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: c.cfg.Host}); err != nil {
			_ = client.Close()
			return nil, fmt.Errorf("STARTTLS 升级失败: %w", err)
		}
	}
	return client, nil
}

// newClient 基于已建立的连接创建 SMTP 会话，并统一设置本次会话的超时
func (c *Client) newClient(conn net.Conn) (*smtp.Client, error) {
	if err := conn.SetDeadline(time.Now().Add(c.cfg.Timeout)); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("设置 SMTP 超时失败: %w", err)
	}
	client, err := smtp.NewClient(conn, c.cfg.Host)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("创建 SMTP 会话失败: %w", err)
	}
	return client, nil
}

// xoauth2Auth 是 SMTP 的 XOAUTH2 认证，Outlook 个人邮箱等需要它
type xoauth2Auth struct {
	// 登录账号
	user string
	// OAuth2 访问令牌
	token string
}

// Start 返回 XOAUTH2 的首个认证报文
func (a *xoauth2Auth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	return "XOAUTH2", []byte("user=" + a.user + "\x01auth=Bearer " + a.token + "\x01\x01"), nil
}

// Next 不支持多轮认证，服务器返回错误时直接结束
func (a *xoauth2Auth) Next(_ []byte, more bool) ([]byte, error) {
	if more {
		return nil, errors.New("XOAUTH2 认证失败")
	}
	return nil, nil
}
