// Package im 是 WuKongIM 服务端 HTTP API 的封装，只做请求与错误转换，不含客服业务；
// HTTP API 只在内网调用，客户端连接需要的 token 由登录接口下发给前端
package im

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"

	"admin/server/pkg/config"
	"admin/server/pkg/log"
)

// requestTimeout 是单次请求的超时
const requestTimeout = 10 * time.Second

// retryTimes 是请求失败后的重试次数，按指数退避
const retryTimes = 3

// retryInterval 是首次重试的等待时间，之后翻倍
const retryInterval = 100 * time.Millisecond

// ErrNotInitialized 是 IM 客户端未初始化
var ErrNotInitialized = errors.New("IM 服务未初始化")

// Error 是 IM 接口返回的业务错误
type Error struct {
	// 请求路径
	Path string
	// HTTP 状态码
	Status int
	// IM 返回的错误描述
	Reason string
}

// Error 返回中文错误提示
func (e *Error) Error() string {
	reason := strings.TrimSpace(e.Reason)
	if reason == "" {
		reason = fmt.Sprintf("HTTP 状态码 %d", e.Status)
	}
	return fmt.Sprintf("IM 接口调用失败: %s", reason)
}

// 成功的业务状态码，不同接口可能返回 0 或 200
const (
	statusSuccess     = 0
	statusSuccessHTTP = 200
)

// response 是 IM 的统一响应外壳，业务字段在 data 里；
// 部分接口直接返回业务字段，解析时两种结构都兼容
type response struct {
	// 业务状态码
	Status int `json:"status"`
	// 提示信息
	Msg string `json:"msg"`
	// 错误描述，非空表示失败
	Reason string `json:"reason"`
	// 业务数据
	Data json.RawMessage `json:"data"`
}

// Client 是 WuKongIM 的 HTTP 客户端
type Client struct {
	// IM 服务端 HTTP API 地址
	apiURL string
	// 客户端连接 gateway 的地址，登录时下发给前端
	wsURL string
	// AI 服务号的 UID
	aiUID string
	// 校验 IM 回调的共享密钥
	webhookToken string
	// HTTP 客户端
	httpClient *http.Client
}

// Default 是全局的 IM 客户端，Init 之后可用
var Default *Client

// Init 用配置文件里的 IM 配置初始化全局客户端
func Init(cfg config.WuKongIM) error {
	apiURL := strings.TrimRight(strings.TrimSpace(cfg.ApiURL), "/")
	if apiURL == "" {
		return errors.New("未配置 wukongim.api_url")
	}

	Default = &Client{
		apiURL:       apiURL,
		wsURL:        strings.TrimSpace(cfg.WsURL),
		aiUID:        strings.TrimSpace(cfg.AiUID),
		webhookToken: strings.TrimSpace(cfg.WebhookToken),
		httpClient:   &http.Client{Timeout: requestTimeout},
	}
	log.Kernel.Info("init wukongim client",
		zap.String("apiUrl", apiURL), zap.String("wsUrl", cfg.WsURL), zap.String("aiUid", cfg.AiUID))
	return nil
}

// WsURL 返回下发给前端的 gateway 地址
func WsURL() string {
	if Default == nil {
		return ""
	}
	return Default.wsURL
}

// AiUID 返回 AI 服务号的 UID
func AiUID() string {
	if Default == nil {
		return ""
	}
	return Default.aiUID
}

// WebhookToken 返回校验 IM 回调的共享密钥，为空表示不校验
func WebhookToken() string {
	if Default == nil {
		return ""
	}
	return Default.webhookToken
}

// EnsureUserToken 把 uid 与 token 登记到 IM，登录时前端带上该 token 连接 gateway；
// 工作台与聊天窗都用 easyjssdk 的 Web 设备标识连接，登记时必须传同一个设备标识，
// 否则 IM 只记住 App 设备的 token，连接时会返回 ReasonAuthFail
func (c *Client) EnsureUserToken(ctx context.Context, uid, token string) error {
	if strings.TrimSpace(uid) == "" || strings.TrimSpace(token) == "" {
		return errors.New("IM 用户与 token 不能为空")
	}
	req := &UserTokenReq{UID: uid, Token: token, DeviceFlag: DeviceFlagWeb}
	return c.do(ctx, "/user/token", req, nil)
}

// OnlineStatus 批量查询用户在线状态，返回 uid 到是否在线的映射；
// IM 的请求体是 uid 数组，返回体是每台设备一条记录，同一 uid 只要有一条在线即算在线
func (c *Client) OnlineStatus(ctx context.Context, uids []string) (map[string]bool, error) {
	if len(uids) == 0 {
		return map[string]bool{}, nil
	}

	var list []onlineStatusResp
	if err := c.do(ctx, "/user/onlinestatus", uids, &list); err != nil {
		return nil, err
	}
	result := make(map[string]bool, len(uids))
	for _, uid := range uids {
		result[uid] = false
	}
	for _, item := range list {
		if item.Online == 1 {
			result[item.UID] = true
		}
	}
	return result, nil
}

// onlineStatusResp 是一条设备维度的在线状态记录
type onlineStatusResp struct {
	// 用户标识
	UID string `json:"uid"`
	// 设备标识
	DeviceFlag int `json:"device_flag"`
	// 在线状态, 1 在线
	Online int `json:"online"`
}

// UpsertChannel 创建或更新频道并全量重置成员，会话群频道使用它
func (c *Client) UpsertChannel(ctx context.Context, channelID string, channelType uint8, subscribers []string) error {
	if strings.TrimSpace(channelID) == "" {
		return errors.New("IM 频道标识不能为空")
	}
	req := &ChannelReq{ChannelID: channelID, ChannelType: channelType, Subscribers: subscribers}
	return c.do(ctx, "/channel", req, nil)
}

// AddSubscribers 往频道里添加成员，转接时把新客服加进会话群
func (c *Client) AddSubscribers(ctx context.Context, channelID string, channelType uint8, subscribers []string) error {
	if len(subscribers) == 0 {
		return nil
	}
	req := &SubscribersReq{ChannelID: channelID, ChannelType: channelType, Subscribers: subscribers}
	return c.do(ctx, "/channel/subscriber_add", req, nil)
}

// RemoveSubscribers 从频道里移除成员，转接时把旧客服移出会话群
func (c *Client) RemoveSubscribers(ctx context.Context, channelID string, channelType uint8, subscribers []string) error {
	if len(subscribers) == 0 {
		return nil
	}
	req := &SubscribersReq{ChannelID: channelID, ChannelType: channelType, Subscribers: subscribers}
	return c.do(ctx, "/channel/subscriber_remove", req, nil)
}

// SendMessage 发送持久消息，payload 为消息的业务协议
func (c *Client) SendMessage(ctx context.Context, req *MessageSendReq) (*MessageSendResp, error) {
	if req == nil {
		return nil, errors.New("发送消息的请求不能为空")
	}
	var resp MessageSendResp
	if err := c.do(ctx, "/message/send", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// AppendEvent 追加流式事件，event_id 幂等，同一事件重复提交只生效一次
func (c *Client) AppendEvent(ctx context.Context, req *MessageEventReq) (*MessageEventResp, error) {
	if req == nil {
		return nil, errors.New("追加流事件的请求不能为空")
	}
	var resp MessageEventResp
	if err := c.do(ctx, "/message/event", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// SyncChannelMessages 拉取频道消息，客户端重连后用它恢复流式消息的终态
func (c *Client) SyncChannelMessages(ctx context.Context, req *SyncChannelMessagesReq) (*SyncChannelMessagesResp, error) {
	if req == nil {
		return nil, errors.New("同步频道消息的请求不能为空")
	}
	var resp SyncChannelMessagesResp
	if err := c.do(ctx, "/channel/messagesync", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// do 发送 POST 请求并把响应解析到 out，失败按指数退避重试；
// 判断成功不能只看 HTTP 200，还要看业务状态码与 reason
func (c *Client) do(ctx context.Context, path string, body any, out any) error {
	if c == nil {
		return ErrNotInitialized
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("序列化 IM 请求失败: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= retryTimes; attempt++ {
		if attempt > 0 {
			// 指数退避，等待期间响应取消信号
			wait := retryInterval << (attempt - 1)
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}

		lastErr = c.request(ctx, path, payload, out)
		if lastErr == nil {
			return nil
		}
		// 上下文取消或业务错误不重试，只有网络与 5xx 才重试
		if ctx.Err() != nil || !isRetryable(lastErr) {
			break
		}
		log.Logger.Warn("wukongim request retry",
			zap.String("path", path), zap.Int("attempt", attempt+1), zap.Error(lastErr))
	}

	log.Logger.Error("wukongim request failed", zap.String("path", path), zap.Error(lastErr))
	return lastErr
}

// request 发送一次请求并解析响应
func (c *Client) request(ctx context.Context, path string, payload []byte, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL+path, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("创建 IM 请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("请求 IM 接口失败: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	data, readErr := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if readErr != nil {
		return fmt.Errorf("读取 IM 响应失败: %w", readErr)
	}

	if err := parseResponse(path, resp.StatusCode, data, out); err != nil {
		return err
	}
	return nil
}

// parseResponse 解析响应体，兼容带 data 外壳与直接返回业务字段两种结构；
// 判断成功不能只看 HTTP 状态，还要看 status 与 reason
func parseResponse(path string, statusCode int, data []byte, out any) error {
	body := bytes.TrimSpace(data)

	var envelope response
	hasEnvelope := len(body) > 0 && json.Unmarshal(body, &envelope) == nil

	// 先看外层外壳的业务状态，status 非成功码时把 msg 作为错误描述
	reason := ""
	if hasEnvelope {
		reason = strings.TrimSpace(envelope.Reason)
		if reason == "" && envelope.Status != statusSuccess && envelope.Status != statusSuccessHTTP {
			reason = strings.TrimSpace(envelope.Msg)
		}
	}

	// 带 data 外壳时业务字段在 data 里，否则就是整个响应体
	target := body
	if hasEnvelope && len(envelope.Data) > 0 && !bytes.Equal(envelope.Data, []byte("null")) {
		target = envelope.Data
	}

	// 有的接口把 reason 直接放在业务字段里，非空同样表示失败
	if reason == "" {
		var bodyReason struct {
			Reason string `json:"reason"`
		}
		if len(target) > 0 && json.Unmarshal(target, &bodyReason) == nil {
			reason = strings.TrimSpace(bodyReason.Reason)
		}
	}

	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		if reason == "" {
			reason = strings.TrimSpace(string(body))
		}
		return &Error{Path: path, Status: statusCode, Reason: reason}
	}
	if reason != "" {
		return &Error{Path: path, Status: statusCode, Reason: reason}
	}
	if out == nil || len(target) == 0 {
		return nil
	}
	if err := json.Unmarshal(target, out); err != nil {
		return fmt.Errorf("解析 IM 响应失败: %w", err)
	}
	return nil
}

// isRetryable 判断错误是否值得重试：业务错误不重试，网络与 5xx 才重试
func isRetryable(err error) bool {
	var imErr *Error
	if errors.As(err, &imErr) {
		return imErr.Status >= http.StatusInternalServerError
	}
	return true
}
