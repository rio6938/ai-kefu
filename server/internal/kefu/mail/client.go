// Package mail 是邮箱渠道的 IMAP 收信与 MIME 解析，与 im 包同级；
// 只负责协议与报文处理，线程、会话、落库由 service 层编排
package mail

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

// defaultTimeout 是连接与命令的默认超时
const defaultTimeout = 20 * time.Second

// maxFetchPerSync 是单次同步最多拉取的邮件数，避免首次同步拉全量历史
const maxFetchPerSync = 200

// Config 是 IMAP 收信配置，由 service 层从邮箱配置解密后传入
type Config struct {
	// IMAP 服务器地址
	Host string
	// IMAP 端口
	Port int
	// 是否直连 SSL，false 时使用 STARTTLS
	SSL bool
	// 登录账号
	Username string
	// 密码或授权码；OAuth2 时为空
	Password string
	// 认证方式，password / oauth2
	AuthType string
	// OAuth2 访问令牌
	AccessToken string
	// 服务商标识，qq/163 登录后需要发送 ID 命令
	Provider string
	// 收信文件夹
	Folder string
	// 是否把拉取过的邮件标记为已读
	MarkSeen bool
}

// Client 是 IMAP 收信客户端，每次同步创建一次会话
type Client struct {
	cfg Config
}

// New 创建 IMAP 客户端，不做连接
func New(cfg Config) *Client {
	if cfg.Folder == "" {
		cfg.Folder = "INBOX"
	}
	return &Client{cfg: cfg}
}

// Fetched 是一封拉取到的原始邮件
type Fetched struct {
	// IMAP UID
	UID uint32
	// 原始大小, 字节
	Size int64
	// 原始邮件内容
	Raw []byte
}

// Session 是一次 IMAP 会话，调用方用完必须 Close
type Session struct {
	// IMAP 客户端
	client *imapclient.Client
	// 邮箱配置
	cfg Config
}

// Connect 建立连接并登录，QQ / 163 登录后发送 ID 命令避免 Unsafe Login
func (c *Client) Connect() (*Session, error) {
	if c.cfg.Host == "" {
		return nil, errors.New("IMAP 服务器地址不能为空")
	}
	if c.cfg.Port <= 0 || c.cfg.Port > 65535 {
		return nil, errors.New("IMAP 端口不正确")
	}

	addr := net.JoinHostPort(c.cfg.Host, strconv.Itoa(c.cfg.Port))
	options := &imapclient.Options{Dialer: &net.Dialer{Timeout: defaultTimeout}}
	var client *imapclient.Client
	var err error
	if c.cfg.SSL {
		client, err = imapclient.DialTLS(addr, options)
	} else {
		client, err = imapclient.DialStartTLS(addr, options)
	}
	if err != nil {
		return nil, fmt.Errorf("连接 IMAP 服务器失败: %w", err)
	}

	if err := c.login(client); err != nil {
		_ = client.Close()
		return nil, err
	}
	// 部分服务商（163）在 ID 之前拒绝任何邮箱操作，失败只记日志不阻断登录
	c.sendID(client)
	return &Session{client: client, cfg: c.cfg}, nil
}

// login 按认证方式登录 IMAP
func (c *Client) login(client *imapclient.Client) error {
	if c.cfg.AuthType == "oauth2" {
		if c.cfg.AccessToken == "" {
			return errors.New("OAuth2 访问令牌为空，请先完成授权")
		}
		// go-imap v2 自带 XOAUTH2 的 SASL 客户端在 imapclient 的 Authenticate 里，
		// 这里使用标准库实现的 SASL 认证
		if err := client.Authenticate(&xoauth2SASL{username: c.cfg.Username, token: c.cfg.AccessToken}); err != nil {
			return fmt.Errorf("IMAP 认证失败: %w", err)
		}
		return nil
	}
	if err := client.Login(c.cfg.Username, c.cfg.Password).Wait(); err != nil {
		return fmt.Errorf("IMAP 认证失败: %w", err)
	}
	return nil
}

// sendID 发送 IMAP ID 命令，QQ / 163 不发送会报 Unsafe Login；
// 服务器不支持 ID 扩展时命令失败，忽略即可
func (c *Client) sendID(client *imapclient.Client) {
	if !client.Caps().Has(imap.CapID) {
		return
	}
	_, _ = client.ID(&imap.IDData{
		Name:    "ai-kefu",
		Version: "1.0",
		Vendor:  "ai-kefu",
	}).Wait()
}

// Test 测试 IMAP 登录与文件夹选择，不修改任何邮件标记
func (c *Client) Test() error {
	session, err := c.Connect()
	if err != nil {
		return err
	}
	defer session.Close()
	if _, err := session.Select(); err != nil {
		return err
	}
	return nil
}

// Select 选择收信文件夹并返回 UIDVALIDITY
func (s *Session) Select() (uint32, error) {
	data, err := s.client.Select(s.cfg.Folder, nil).Wait()
	if err != nil {
		return 0, fmt.Errorf("打开文件夹 %s 失败: %w", s.cfg.Folder, err)
	}
	return data.UIDValidity, nil
}

// FetchSinceUID 拉取 UID 大于 sinceUID 的邮件，最多 maxFetchPerSync 封
func (s *Session) FetchSinceUID(sinceUID uint32) ([]Fetched, error) {
	criteria := &imap.SearchCriteria{}
	if sinceUID > 0 {
		criteria.UID = []imap.UIDSet{imap.UIDSetNum(imap.UID(sinceUID + 1))}
	}
	return s.searchAndFetch(criteria)
}

// FetchSinceDays 拉取最近 N 天的邮件，最多 maxFetchPerSync 封，用于首次同步
func (s *Session) FetchSinceDays(days int) ([]Fetched, error) {
	if days <= 0 {
		days = 7
	}
	criteria := &imap.SearchCriteria{
		Since: time.Now().AddDate(0, 0, -days),
	}
	return s.searchAndFetch(criteria)
}

// searchAndFetch 按条件搜索并拉取原始邮件，默认用 BODY.PEEK 不改变已读标记
func (s *Session) searchAndFetch(criteria *imap.SearchCriteria) ([]Fetched, error) {
	data, err := s.client.UIDSearch(criteria, nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("搜索邮件失败: %w", err)
	}
	uids := uidList(data.All)
	if len(uids) == 0 {
		return nil, nil
	}
	// 只取最新的若干封，避免一次拉取过多历史邮件
	if len(uids) > maxFetchPerSync {
		uids = uids[len(uids)-maxFetchPerSync:]
	}

	options := &imap.FetchOptions{
		UID:         true,
		RFC822Size:  true,
		Envelope:    true,
		BodySection: []*imap.FetchItemBodySection{{Peek: true}},
	}
	fetchCmd := s.client.Fetch(imap.UIDSetNum(uids...), options)

	list := make([]Fetched, 0, len(uids))
	var markSeen []imap.UID
	for {
		message := fetchCmd.Next()
		if message == nil {
			break
		}
		buffer, err := message.Collect()
		if err != nil {
			_ = fetchCmd.Close()
			return nil, fmt.Errorf("读取邮件内容失败: %w", err)
		}
		if len(buffer.BodySection) == 0 || len(buffer.BodySection[0].Bytes) == 0 {
			continue
		}
		list = append(list, Fetched{
			UID:  uint32(buffer.UID),
			Size: buffer.RFC822Size,
			Raw:  buffer.BodySection[0].Bytes,
		})
		markSeen = append(markSeen, buffer.UID)
	}
	if err := fetchCmd.Close(); err != nil {
		return nil, fmt.Errorf("拉取邮件失败: %w", err)
	}
	if s.cfg.MarkSeen && len(markSeen) > 0 {
		if _, err := s.client.Store(imap.UIDSetNum(markSeen...), &imap.StoreFlags{
			Op:     imap.StoreFlagsAdd,
			Flags:  []imap.Flag{imap.FlagSeen},
			Silent: true,
		}, nil).Collect(); err != nil {
			// 标记已读失败不影响收信，下次拉取会按 UID 去重
			return list, nil
		}
	}
	return list, nil
}

// Close 关闭 IMAP 连接
func (s *Session) Close() {
	if s.client == nil {
		return
	}
	_ = s.client.Logout().Wait()
	_ = s.client.Close()
}

// uidList 把 IMAP 搜索结果转换成 UID 升序列表
func uidList(numSet imap.NumSet) []imap.UID {
	set, ok := numSet.(imap.UIDSet)
	if !ok {
		return nil
	}
	list := make([]imap.UID, 0)
	for _, item := range set {
		if item.Start == item.Stop {
			list = append(list, item.Start)
			continue
		}
		for uid := item.Start; uid <= item.Stop; uid++ {
			list = append(list, uid)
		}
	}
	return list
}

// xoauth2SASL 是 IMAP 的 XOAUTH2 认证，Outlook 个人邮箱等需要它
type xoauth2SASL struct {
	// 登录账号
	username string
	// OAuth2 访问令牌
	token string
}

// Start 返回 XOAUTH2 机制名与首个认证报文
func (a *xoauth2SASL) Start() (string, []byte, error) {
	return "XOAUTH2", []byte("user=" + a.username + "\x01auth=Bearer " + a.token + "\x01\x01"), nil
}

// Next 不支持多轮认证，服务器要求继续时直接失败
func (a *xoauth2SASL) Next(_ []byte) ([]byte, error) {
	return nil, errors.New("XOAUTH2 认证失败")
}

// Append 把一封已发送邮件写入指定文件夹，用于「已发送」同步；
// 追加失败不影响发信结果，由调用方记日志
func (s *Session) Append(folder string, raw []byte) error {
	if strings.TrimSpace(folder) == "" {
		return errors.New("已发送文件夹不能为空")
	}
	command := s.client.Append(folder, int64(len(raw)), &imap.AppendOptions{
		Flags: []imap.Flag{imap.FlagSeen},
	})
	if _, err := command.Write(raw); err != nil {
		_ = command.Close()
		return fmt.Errorf("写入已发送邮件失败: %w", err)
	}
	if err := command.Close(); err != nil {
		return fmt.Errorf("提交已发送邮件失败: %w", err)
	}
	if _, err := command.Wait(); err != nil {
		return fmt.Errorf("追加已发送邮件失败: %w", err)
	}
	return nil
}
