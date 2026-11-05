package service

import (
	"context"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"admin/server/internal/kefu/im"
	"admin/server/internal/kefu/repository"
	"admin/server/pkg/log"
	rdb "admin/server/pkg/redis"
)

// 客服在线状态缓存的常量
const (
	// agentOnlineKeyPrefix 是客服在线状态的 redis key 前缀, key 为 kefu:agent:online:{userId}
	agentOnlineKeyPrefix = "kefu:agent:online:"
	// agentOnlineTTL 是在线状态的缓存时长，过期后回源 IM 查询
	agentOnlineTTL = 30 * 60
	// agentOfflineTTL 是离线状态的缓存时长，比在线短，便于尽快感知重新上线
	agentOfflineTTL = 2 * 60
)

// KefuOnlineService 是客服在线状态的业务处理：
// 状态不落库，以 IM 在线路由为准，Redis 只做缓存
type KefuOnlineService struct {
	// 客服档案的数据访问
	agentRepository *repository.KefuAgentRepository
}

// NewKefuOnlineService 创建客服在线状态的业务处理
func NewKefuOnlineService() *KefuOnlineService {
	return &KefuOnlineService{
		agentRepository: repository.NewKefuAgentRepository(),
	}
}

// HandleNotify 处理 IM 的在线状态回调，把状态写入缓存并返回本次上线的客服用户id；
// 回调报文无法解析时只记日志并返回空列表，不阻塞 IM
func (s *KefuOnlineService) HandleNotify(ctx context.Context, body []byte) []int64 {
	events, err := im.DecodeOnlineNotify(body)
	if err != nil {
		log.Logger.Warn("decode kefu online webhook failed", zap.Error(err))
		return nil
	}

	onlineUserIDs := make([]int64, 0, len(events))
	for _, event := range events {
		userID, ok := agentUserID(event.UID)
		if !ok {
			continue
		}
		online := event.Online == 1
		s.setCache(ctx, userID, online)
		if online {
			onlineUserIDs = append(onlineUserIDs, userID)
		}
	}
	return onlineUserIDs
}

// IsOnline 查询客服是否在线，缓存没有命中时回源 IM
func (s *KefuOnlineService) IsOnline(ctx context.Context, userID int64) bool {
	if userID <= 0 {
		return false
	}
	return s.OnlineMap(ctx, []int64{userID})[userID]
}

// OnlineMap 批量查询客服在线状态，先读缓存，缺失的一次性回源 IM 并写回缓存
func (s *KefuOnlineService) OnlineMap(ctx context.Context, userIDs []int64) map[int64]bool {
	result := make(map[int64]bool, len(userIDs))
	missing := make([]int64, 0, len(userIDs))
	seen := make(map[int64]bool, len(userIDs))
	for _, userID := range userIDs {
		if userID <= 0 || seen[userID] {
			continue
		}
		seen[userID] = true
		if !s.loadCache(ctx, userID, result) {
			missing = append(missing, userID)
		}
	}
	if len(missing) == 0 {
		return result
	}

	// 缺失的状态一次性回源，避免逐个请求 IM
	uids := make([]string, 0, len(missing))
	uidToUserID := make(map[string]int64, len(missing))
	for _, userID := range missing {
		uid := agentUID(userID)
		uids = append(uids, uid)
		uidToUserID[uid] = userID
	}

	statusMap, err := im.Default.OnlineStatus(ctx, uids)
	if err != nil {
		log.Logger.Warn("query kefu agent online status failed", zap.Error(err))
		for _, userID := range missing {
			result[userID] = false
		}
		return result
	}
	for uid, userID := range uidToUserID {
		online := statusMap[uid]
		result[userID] = online
		s.setCache(ctx, userID, online)
	}
	return result
}

// loadCache 从缓存读取在线状态，命中时写入结果并返回 true
func (s *KefuOnlineService) loadCache(ctx context.Context, userID int64, result map[int64]bool) bool {
	if rdb.Client == nil {
		return false
	}
	value, err := rdb.Client.Get(ctx, onlineKey(userID)).Result()
	if err != nil || value == "" {
		return false
	}
	result[userID] = value == "1"
	return true
}

// setCache 写入在线状态缓存，缓存不可用不影响业务
func (s *KefuOnlineService) setCache(ctx context.Context, userID int64, online bool) {
	if rdb.Client == nil {
		return
	}
	value := "0"
	ttl := agentOfflineTTL
	if online {
		value = "1"
		ttl = agentOnlineTTL
	}
	if err := rdb.Client.Set(ctx, onlineKey(userID), value,
		time.Duration(ttl)*time.Second).Err(); err != nil {
		log.Logger.Warn("cache kefu agent online status failed",
			zap.Int64("userId", userID), zap.Error(err))
	}
}

// onlineKey 返回客服在线状态的缓存 key
func onlineKey(userID int64) string {
	return agentOnlineKeyPrefix + strconv.FormatInt(userID, 10)
}

// agentUserID 从 IM 用户标识 agent_{userId} 解析出管理员用户id
func agentUserID(uid string) (int64, bool) {
	value := strings.TrimSpace(uid)
	if !strings.HasPrefix(value, "agent_") {
		return 0, false
	}
	userID, err := strconv.ParseInt(strings.TrimPrefix(value, "agent_"), 10, 64)
	if err != nil || userID <= 0 {
		return 0, false
	}
	return userID, true
}
