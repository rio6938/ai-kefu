package im

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// 频道类型
const (
	// ChannelTypePerson 是单聊频道
	ChannelTypePerson uint8 = 1
	// ChannelTypeGroup 是群聊频道，客服会话使用群频道 session_{sessionId}
	ChannelTypeGroup uint8 = 2
)

// 流式事件类型，event_type 只接受这几个值
const (
	// EventTypeStreamOpen 是流开始
	EventTypeStreamOpen = "stream.open"
	// EventTypeStreamDelta 是流增量
	EventTypeStreamDelta = "stream.delta"
	// EventTypeStreamClose 是流关闭，已废弃，收尾统一用 stream.finish
	EventTypeStreamClose = "stream.close"
	// EventTypeStreamError 是流出错
	EventTypeStreamError = "stream.error"
	// EventTypeStreamCancel 是流被中断
	EventTypeStreamCancel = "stream.cancel"
	// EventTypeStreamSnapshot 是流快照
	EventTypeStreamSnapshot = "stream.snapshot"
	// EventTypeStreamFinish 是流结束，带完整快照
	EventTypeStreamFinish = "stream.finish"
)

// 消息 payload 里的业务类型，对应 FR-2.3 协议
const (
	// PayloadTypeText 是文本
	PayloadTypeText = "text"
	// PayloadTypeImage 是图片
	PayloadTypeImage = "image"
	// PayloadTypeFile 是文件
	PayloadTypeFile = "file"
	// PayloadTypeSystem 是系统消息
	PayloadTypeSystem = "system"
	// PayloadTypeAiAnswer 是 AI 回答
	PayloadTypeAiAnswer = "ai_answer"
	// PayloadTypeQueue 是客户要求转人工的通知，只推给客服本人，用于工作台提醒
	PayloadTypeQueue = "queue"
	// PayloadTypeTyping 是正在输入的瞬时状态，不落库、不计未读
	PayloadTypeTyping = "typing"
)

// 消息 payload 里的角色，对应 FR-2.3 协议
const (
	// PayloadRoleVisitor 是访客
	PayloadRoleVisitor = "visitor"
	// PayloadRoleAgent 是客服
	PayloadRoleAgent = "agent"
	// PayloadRoleAi 是 AI 服务号
	PayloadRoleAi = "ai"
	// PayloadRoleSystem 是系统
	PayloadRoleSystem = "system"
)

// Payload 是消息 payload 的业务协议，version 用于后续兼容
type Payload struct {
	// 协议版本
	Version int `json:"version"`
	// 内容类型, text/image/file/system/ai_answer
	Type string `json:"type"`
	// 发送者角色, visitor/agent/ai/system
	Role string `json:"role"`
	// 文本内容
	Content string `json:"content"`
	// 业务消息id
	MessageID int64 `json:"messageId"`
	// 会话id
	SessionID int64 `json:"sessionId"`
	// 附加信息
	Meta map[string]any `json:"meta,omitempty"`
}

// EventPayload 是流式事件的数据体
type EventPayload struct {
	// 内容类型, 目前只有 text
	Kind string `json:"kind"`
	// 本次增量文本
	Delta string `json:"delta,omitempty"`
	// 结束时的完整快照
	Snapshot *EventSnapshot `json:"snapshot,omitempty"`
	// 中断或出错的原因
	Reason string `json:"reason,omitempty"`
}

// EventSnapshot 是流结束时的完整内容
type EventSnapshot struct {
	// 内容类型, 目前只有 text
	Kind string `json:"kind"`
	// 完整文本
	Text string `json:"text"`
}

// MessageHeader 是消息的设置项，setting 属于请求体顶层字段，不在这里
type MessageHeader struct {
	// 为 1 时消息不落库
	NoPersist int `json:"no_persist"`
	// 为 1 时计未读红点
	RedDot int `json:"red_dot"`
	// 为 1 时只在在线端可见
	SyncOnce int `json:"sync_once"`
}

// IM 设备标识，取值与协议一致；IM 按 uid + 设备标识分别保存 token，
// 客户端用哪个设备标识连接，token 就必须用同一个设备标识登记
const (
	// DeviceFlagApp 是 App 客户端
	DeviceFlagApp = 0
	// DeviceFlagWeb 是 Web 客户端，客服工作台与网页聊天窗都是这种
	DeviceFlagWeb = 1
	// DeviceFlagDesktop 是桌面客户端
	DeviceFlagDesktop = 2
)

// UserTokenReq 是注册用户 token 的请求体，token 由我方生成后登记到 IM
type UserTokenReq struct {
	// 用户标识, 如 visitor_1 / agent_2 / ai_service
	UID string `json:"uid"`
	// 用户 token
	Token string `json:"token"`
	// 设备标识, 不传时 IM 默认按 App 登记，Web 端连接会鉴权失败
	DeviceFlag int `json:"device_flag"`
}

// ChannelReq 是创建或更新频道的请求体，subscribers 为频道成员
type ChannelReq struct {
	// 频道标识, 如 session_88
	ChannelID string `json:"channel_id"`
	// 频道类型
	ChannelType uint8 `json:"channel_type"`
	// 频道成员
	Subscribers []string `json:"subscribers"`
}

// SubscribersReq 是增减频道成员的请求体
type SubscribersReq struct {
	// 频道标识
	ChannelID string `json:"channel_id"`
	// 频道类型
	ChannelType uint8 `json:"channel_type"`
	// 成员列表
	Subscribers []string `json:"subscribers"`
}

// MessageSendReq 是发送消息的请求体，payload 为 base64 编码的业务协议
//
// 注意 setting 在 WuKongIM v3 是请求体顶层字段，放在 header 里会被忽略，
// 导致流式基础消息以普通消息落库，后续 /message/event 被拒
// （v3 报错 committed stream base message required）
type MessageSendReq struct {
	// 消息设置标记，AI 流式基础消息固定为 2
	Setting int32 `json:"setting,omitempty"`
	// 消息设置项
	Header MessageHeader `json:"header"`
	// 幂等消息编号
	ClientMsgNo string `json:"client_msg_no"`
	// 发送者标识
	FromUID string `json:"from_uid"`
	// 频道标识
	ChannelID string `json:"channel_id"`
	// 频道类型
	ChannelType uint8 `json:"channel_type"`
	// 消息内容, json 编码后自动转成 base64
	Payload []byte `json:"payload"`
}

// MessageSendResp 是发送消息的响应
type MessageSendResp struct {
	// 消息id
	MessageID int64 `json:"message_id"`
	// 幂等消息编号
	ClientMsgNo string `json:"client_msg_no"`
	// 消息序号
	MessageSeq uint32 `json:"message_seq"`
}

// MessageEventReq 是追加流式事件的请求体，event_id 幂等
type MessageEventReq struct {
	// 事件id, 同一事件重复提交只生效一次
	EventID string `json:"event_id"`
	// 事件类型
	EventType string `json:"event_type"`
	// 频道标识
	ChannelID string `json:"channel_id"`
	// 频道类型
	ChannelType uint8 `json:"channel_type"`
	// 发送者标识
	FromUID string `json:"from_uid"`
	// 所属基础消息的幂等消息编号
	ClientMsgNo string `json:"client_msg_no"`
	// 所属基础消息的序号
	MessageSeq uint32 `json:"message_seq"`
	// 事件数据, json 编码后自动转成 base64
	Data []byte `json:"data"`
}

// MessageEventResp 是追加流式事件的响应
type MessageEventResp struct {
	// 消息id
	MessageID int64 `json:"message_id"`
	// 消息序号
	MessageSeq uint32 `json:"message_seq"`
	// 幂等消息编号
	ClientMsgNo string `json:"client_msg_no"`
}

// SyncChannelMessagesReq 是拉取频道消息的请求体，重连后取终态时使用
type SyncChannelMessagesReq struct {
	// 登录用户标识
	LoginUID string `json:"login_uid"`
	// 频道标识
	ChannelID string `json:"channel_id"`
	// 频道类型
	ChannelType uint8 `json:"channel_type"`
	// 起始消息序号, 从大到小拉取
	StartMessageSeq uint32 `json:"start_message_seq"`
	// 结束消息序号
	EndMessageSeq uint32 `json:"end_message_seq"`
	// 拉取条数
	Limit int `json:"limit"`
	// 拉取模式, 1 表示向上翻页
	PullMode int `json:"pull_mode"`
}

// SyncChannelMessagesResp 是拉取频道消息的响应
type SyncChannelMessagesResp struct {
	// 消息列表
	Messages []ChannelMessage `json:"messages"`
	// 是否还有更多
	More bool `json:"more"`
}

// ChannelMessage 是频道消息
type ChannelMessage struct {
	// 消息id
	MessageID int64 `json:"message_id"`
	// 消息序号
	MessageSeq uint32 `json:"message_seq"`
	// 幂等消息编号
	ClientMsgNo string `json:"client_msg_no"`
	// 发送者标识
	FromUID string `json:"from_uid"`
	// 频道标识
	ChannelID string `json:"channel_id"`
	// 频道类型
	ChannelType uint8 `json:"channel_type"`
	// 消息内容, base64 编码
	Payload string `json:"payload"`
	// 服务端时间戳, 秒
	Timestamp int64 `json:"timestamp"`
}

// MessageNotify 是 IM 消息回调（msg.notify）的请求体
type MessageNotify struct {
	// 发送者标识
	FromUID string `json:"from_uid"`
	// 频道标识
	ChannelID string `json:"channel_id"`
	// 频道类型
	ChannelType uint8 `json:"channel_type"`
	// IM 消息id
	MessageID int64 `json:"message_id"`
	// IM 消息序号
	MessageSeq uint32 `json:"message_seq"`
	// 幂等消息编号
	ClientMsgNo string `json:"client_msg_no"`
	// 服务端时间戳, 秒
	Timestamp int64 `json:"timestamp"`
	// 消息内容, base64 编码的业务协议
	Payload string `json:"payload"`
}

// OnlineNotify 是在线状态回调（user.onlinestatus）里的一条状态变更
type OnlineNotify struct {
	// 用户标识, 如 visitor_1 / agent_2
	UID string `json:"uid"`
	// 在线状态, 1 在线, 0 离线
	Online int `json:"online"`
	// 最后离线时间, 秒级时间戳
	LastOffline int64 `json:"last_offline"`
}

// onlineNotifyEnvelope 是在线状态回调里包裹多条事件的形态
type onlineNotifyEnvelope struct {
	// 状态变更列表
	Events []OnlineNotify `json:"events"`
}

// DecodeOnlineNotify 解析在线状态回调，兼容单条、数组与包裹在 events 里的三种形态；
// 无法解析时返回错误，由调用方记日志
func DecodeOnlineNotify(body []byte) ([]OnlineNotify, error) {
	raw := bytes.TrimSpace(body)
	if len(raw) == 0 {
		return nil, errors.New("在线状态回调内容为空")
	}

	// 数组形态：直接是多条状态变更
	if raw[0] == '[' {
		var list []OnlineNotify
		if err := json.Unmarshal(raw, &list); err != nil {
			return nil, fmt.Errorf("解析在线状态回调失败: %w", err)
		}
		return list, nil
	}

	// 外壳形态：{"events":[...]}
	var envelope onlineNotifyEnvelope
	if err := json.Unmarshal(raw, &envelope); err == nil && len(envelope.Events) > 0 {
		return envelope.Events, nil
	}

	// 单条形态
	var single OnlineNotify
	if err := json.Unmarshal(raw, &single); err != nil {
		return nil, fmt.Errorf("解析在线状态回调失败: %w", err)
	}
	return []OnlineNotify{single}, nil
}

// ReceiptNotify 是 IM 消息回执回调（msg.offline / readed 等）的请求体
type ReceiptNotify struct {
	// 发送者标识
	FromUID string `json:"from_uid"`
	// 频道标识
	ChannelID string `json:"channel_id"`
	// 频道类型
	ChannelType uint8 `json:"channel_type"`
	// 消息id
	MessageID int64 `json:"message_id"`
	// 消息序号
	MessageSeq uint32 `json:"message_seq"`
	// 幂等消息编号
	ClientMsgNo string `json:"client_msg_no"`
	// 服务端时间戳, 秒
	Timestamp int64 `json:"timestamp"`
}

// DecodePayload 解码回调里的消息内容，兼容 base64 与明文 JSON 两种形式
func (n *MessageNotify) DecodePayload() (*Payload, error) {
	if n == nil || strings.TrimSpace(n.Payload) == "" {
		return nil, errors.New("消息内容为空")
	}

	payload := new(Payload)
	raw := strings.TrimSpace(n.Payload)
	// IM 回调里的 payload 是 base64 编码的字节数组，明文 JSON 也一并兼容
	if decoded, err := base64.StdEncoding.DecodeString(raw); err == nil && len(decoded) > 0 {
		if err := json.Unmarshal(decoded, payload); err == nil {
			return payload, nil
		}
	}
	if err := json.Unmarshal([]byte(raw), payload); err != nil {
		return nil, fmt.Errorf("解析消息内容失败: %w", err)
	}
	return payload, nil
}

// SessionID 从频道标识 session_{id} 解析会话id，解析失败返回 0
func (n *MessageNotify) SessionID() int64 {
	if n == nil {
		return 0
	}
	return sessionIDFromChannel(n.ChannelID)
}

// sessionIDFromChannel 从频道标识 session_{id} 解析会话id
func sessionIDFromChannel(channelID string) int64 {
	value := strings.TrimSpace(channelID)
	if !strings.HasPrefix(value, channelPrefix) {
		return 0
	}
	sessionID, err := strconv.ParseInt(strings.TrimPrefix(value, channelPrefix), 10, 64)
	if err != nil || sessionID <= 0 {
		return 0
	}
	return sessionID
}

// ChannelIDOfSession 返回会话对应的群频道标识 session_{id}
func ChannelIDOfSession(sessionID int64) string {
	return fmt.Sprintf("%s%d", channelPrefix, sessionID)
}

// channelPrefix 是会话群频道标识的前缀
const channelPrefix = "session_"
