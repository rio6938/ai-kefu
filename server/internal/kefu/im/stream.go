package im

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"admin/server/pkg/log"
	rdb "admin/server/pkg/redis"

	"go.uber.org/zap"
)

// AI 回复使用到的常量
const (
	// interruptKeyPrefix 是中断标记的 redis key 前缀, key 为 kefu:ai:interrupt:{replyId}
	interruptKeyPrefix = "kefu:ai:interrupt:"
	// interruptTTL 是中断标记的过期时间，异常退出后自动清理
	interruptTTL = 10 * time.Minute
)

// ErrStreamFinished 是回复已经收尾，不能再写增量
var ErrStreamFinished = errors.New("AI 回复已结束")

// InterruptKey 返回 AI 回复对应的中断标记 key
func InterruptKey(replyID int64) string {
	return interruptKeyPrefix + strconv.FormatInt(replyID, 10)
}

// MarkInterrupt 给进行中的 AI 回复置中断标记，生成循环在每个增量前检查它
func MarkInterrupt(ctx context.Context, replyID int64, by string) error {
	if rdb.Client == nil || replyID <= 0 {
		return nil
	}
	if err := rdb.Client.Set(ctx, InterruptKey(replyID), by, interruptTTL).Err(); err != nil {
		return fmt.Errorf("写入 AI 中断标记失败: %w", err)
	}
	return nil
}

// InterruptReason 读取 AI 回复的中断来源，没有中断标记时返回空串
func InterruptReason(ctx context.Context, replyID int64) string {
	if rdb.Client == nil || replyID <= 0 {
		return ""
	}
	value, err := rdb.Client.Get(ctx, InterruptKey(replyID)).Result()
	if err != nil {
		return ""
	}
	return value
}

// ClearInterrupt 清除 AI 回复的中断标记
func ClearInterrupt(ctx context.Context, replyID int64) error {
	if rdb.Client == nil || replyID <= 0 {
		return nil
	}
	if err := rdb.Client.Del(ctx, InterruptKey(replyID)).Err(); err != nil {
		return fmt.Errorf("清除 AI 中断标记失败: %w", err)
	}
	return nil
}

// StreamReply 是一次 AI 回复。
// 本 IM 版本的流式事件只承载它自己的状态，不带业务文本（客户端收到的 payload 恒为 null），
// 因此这里改成"边生成边累积、收尾时发一条完整消息"：
// 逐字效果放弃，客户端收到即完整内容，不会再出现气泡一直空着或消息成批出现
type StreamReply struct {
	// IM 客户端
	client *Client
	// 发消息使用的上下文，已经去掉上游的取消信号
	ctx context.Context
	// 会话id
	sessionID int64
	// AI 回复id
	replyID int64
	// 消息的幂等编号 reply_{replyId}
	clientMsgNo string
	// 已经累积的文本
	buf strings.Builder
	// 是否已经开始
	opened bool
	// 是否已经收尾
	finished bool
	// 是否已经读到中断标记
	interrupted atomic.Bool
	// 保护累积文本与状态的互斥锁，保证增量串行写入
	mu sync.Mutex
}

// NewStreamReply 创建一次 AI 回复；
// 收尾发消息必须能在任何路径下完成，因此这里用 context.WithoutCancel 去掉上游的取消信号
func NewStreamReply(ctx context.Context, sessionID, replyID int64) *StreamReply {
	return &StreamReply{
		client:      Default,
		ctx:         context.WithoutCancel(ctx),
		sessionID:   sessionID,
		replyID:     replyID,
		clientMsgNo: fmt.Sprintf("reply_%d", replyID),
	}
}

// ClientMsgNo 返回消息的幂等编号 reply_{replyId}
func (s *StreamReply) ClientMsgNo() string {
	return s.clientMsgNo
}

// Begin 标记回复开始，真正的消息在 Finish 时一次发出
func (s *StreamReply) Begin() (string, error) {
	if s.client == nil {
		return "", ErrNotInitialized
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.opened = true
	return s.clientMsgNo, nil
}

// Delta 累积一段增量文本，收尾时随完整消息一起发出
func (s *StreamReply) Delta(text string) error {
	if text == "" {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finished {
		return ErrStreamFinished
	}
	s.buf.WriteString(text)
	return nil
}

// Cancel 标记回复被中断，已累积的文本仍然由 Finish 发出去
func (s *StreamReply) Cancel(reason string) error {
	log.Logger.Debug("kefu ai reply cancelled",
		zap.Int64("replyId", s.replyID), zap.String("reason", reason))
	return nil
}

// Discard 丢弃本次回复，之后不再发出任何消息；
// 客户要求转人工时由系统消息替代 AI 回复，避免把半截回复发给客户
func (s *StreamReply) Discard() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.finished = true
	s.buf.Reset()
	return nil
}

// Error 标记回复出错，兜底话术仍然由 Finish 发出去
func (s *StreamReply) Error(reason string) error {
	log.Logger.Debug("kefu ai reply failed",
		zap.Int64("replyId", s.replyID), zap.String("reason", reason))
	return nil
}

// Finish 用完整文本发出一条持久消息，重复调用只生效一次；
// 文本为空时不发消息，避免客户收到空气泡；
// 发送失败时换一个新的幂等编号重试一次：频道日志冲突这类错误表示本次并未追加成功，
// 换编号重试不会让客户收到重复气泡
func (s *StreamReply) Finish(snapshot string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finished {
		return nil
	}
	s.finished = true

	text := strings.TrimSpace(snapshot)
	if text == "" {
		text = strings.TrimSpace(s.buf.String())
	}
	if text == "" || s.client == nil {
		return nil
	}

	raw, err := json.Marshal(s.payload(text))
	if err != nil {
		return fmt.Errorf("序列化 AI 消息失败: %w", err)
	}
	_, err = s.client.SendMessage(s.ctx, s.sendReq(s.clientMsgNo, raw))
	if err == nil {
		return nil
	}
	log.Logger.Warn("send kefu ai message failed, retry with new client msg no",
		zap.Int64("replyId", s.replyID), zap.String("clientMsgNo", s.clientMsgNo), zap.Error(err))
	if _, retryErr := s.client.SendMessage(s.ctx,
		s.sendReq(fmt.Sprintf("%s_%d", s.clientMsgNo, time.Now().UnixNano()), raw)); retryErr != nil {
		return retryErr
	}
	return nil
}

// sendReq 构造发送持久消息的请求
func (s *StreamReply) sendReq(clientMsgNo string, payload []byte) *MessageSendReq {
	return &MessageSendReq{
		Header:      MessageHeader{RedDot: 1},
		ClientMsgNo: clientMsgNo,
		FromUID:     s.client.aiUID,
		ChannelID:   ChannelIDOfSession(s.sessionID),
		ChannelType: ChannelTypeGroup,
		Payload:     payload,
	}
}

// Interrupted 读取中断标记，客服关闭 AI 或点中断后生成循环据此停止产出
func (s *StreamReply) Interrupted() bool {
	if s.interrupted.Load() {
		return true
	}
	if rdb.Client == nil || s.replyID <= 0 {
		return false
	}
	count, err := rdb.Client.Exists(context.Background(), InterruptKey(s.replyID)).Result()
	if err != nil || count == 0 {
		return false
	}
	s.interrupted.Store(true)
	return true
}

// payload 返回 AI 消息的业务协议，meta 里带回复id与状态
func (s *StreamReply) payload(text string) *Payload {
	return &Payload{
		Version:   1,
		Type:      PayloadTypeAiAnswer,
		Role:      PayloadRoleAi,
		Content:   text,
		SessionID: s.sessionID,
		Meta: map[string]any{
			"replyId": s.replyID,
			"status":  "finished",
		},
	}
}
