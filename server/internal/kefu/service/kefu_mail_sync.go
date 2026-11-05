package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"admin/server/internal/kefu/mail"
	"admin/server/internal/kefu/repository"
	"admin/server/model"
	"admin/server/pkg/log"
	rdb "admin/server/pkg/redis"
	"admin/server/pkg/setting"
)

// 收信 worker 的常量
const (
	// mailSyncScanInterval 是扫描邮箱是否到期的间隔
	mailSyncScanInterval = 30 * time.Second
	// mailSyncLockPrefix 是邮箱同步锁的 redis key 前缀
	mailSyncLockPrefix = "kefu:mail:sync:"
	// mailSyncManualTimeout 是手动同步的超时，避免页面等待过久
	mailSyncManualTimeout = 25 * time.Second
	// mailSyncConnectTimeout 是单次收信的总超时
	mailSyncConnectTimeout = 120 * time.Second
)

// 收信相关的业务错误
var (
	// ErrMailSyncBusy 是同一邮箱正在同步
	ErrMailSyncBusy = errors.New("该邮箱正在同步，请稍后再试")
	// ErrMailSyncDisabled 是邮箱未开启同步
	ErrMailSyncDisabled = errors.New("邮箱未开启自动同步")
)

// KefuMailSyncService 是 IMAP 收信的编排：worker 启停、增量游标与手动同步
type KefuMailSyncService struct {
	// 邮箱配置的数据访问
	mailboxRepository *repository.KefuMailboxRepository
	// 邮箱配置的业务处理，取解密后的连接配置
	mailboxService *KefuMailboxService
	// 渠道的数据访问，停用的渠道不再拉信
	channelRepository *repository.KefuChannelRepository
	// 入站邮件的业务处理
	inboundService *KefuMailInboundService
	// 启动标记，保证只启动一次 worker
	startOnce sync.Once
}

// NewKefuMailSyncService 创建 IMAP 收信的编排
func NewKefuMailSyncService() *KefuMailSyncService {
	return &KefuMailSyncService{
		mailboxRepository: repository.NewKefuMailboxRepository(),
		mailboxService:    NewKefuMailboxService(),
		channelRepository: repository.NewKefuChannelRepository(),
		inboundService:    NewKefuMailInboundService(),
	}
}

// StartTasks 启动收信 worker，启动后立即扫描一次，之后每 30 秒扫描一次；
// 每个邮箱按自己的同步间隔执行，用 redis 锁保证多实例只有一个在拉同一邮箱
func (s *KefuMailSyncService) StartTasks() {
	s.startOnce.Do(func() {
		go func() {
			s.scan()
			ticker := time.NewTicker(mailSyncScanInterval)
			defer ticker.Stop()
			for range ticker.C {
				s.scan()
			}
		}()
		log.Kernel.Info("start kefu mail sync worker")
	})
}

// scan 扫描所有启用邮箱，到达同步间隔的邮箱异步拉取
func (s *KefuMailSyncService) scan() {
	if !mailEnabled() {
		return
	}
	list, err := s.mailboxRepository.ListEnabled()
	if err != nil {
		log.Logger.Error("list kefu mailboxes failed", zap.Error(err))
		return
	}
	now := time.Now()
	for i := range list {
		mailbox := list[i]
		if mailbox.SyncEnabled != 1 {
			continue
		}
		interval := mailbox.SyncIntervalSec
		if interval <= 0 {
			interval = setting.GetInt(configKeyMailSyncIntervalSec, defaultMailSyncIntervalSec)
		}
		if interval <= 0 {
			interval = defaultMailSyncIntervalSec
		}
		last := time.Time(mailbox.LastSyncTime.Time)
		if !last.IsZero() && now.Sub(last) < time.Duration(interval)*time.Second {
			continue
		}
		go func(mailboxID int64) {
			ctx, cancel := context.WithTimeout(context.Background(), mailSyncConnectTimeout)
			defer cancel()
			if _, err := s.syncMailbox(ctx, mailboxID); err != nil {
				log.Logger.Warn("sync kefu mailbox failed",
					zap.Int64("mailboxId", mailboxID), zap.Error(err))
			}
		}(mailbox.MailboxID)
	}
}

// SyncChannel 手动同步渠道绑定的邮箱，返回本次拉取结果
func (s *KefuMailSyncService) SyncChannel(
	ctx context.Context, channelID int64,
) (*model.KefuMailSyncResult, error) {
	mailbox, err := s.mailboxRepository.GetByChannelID(channelID)
	if err != nil {
		return nil, err
	}
	if mailbox.Status != model.KefuMailboxStatusNormal {
		return nil, errors.New("邮箱已停用")
	}
	if !mailEnabled() {
		return nil, ErrMailDisabled
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, mailSyncManualTimeout)
	defer cancel()
	return s.syncMailbox(timeoutCtx, mailbox.MailboxID)
}

// syncMailbox 同步一个邮箱：加锁、连接、按游标增量拉取、逐封解析归档
func (s *KefuMailSyncService) syncMailbox(
	ctx context.Context, mailboxID int64,
) (*model.KefuMailSyncResult, error) {
	mailbox, err := s.mailboxRepository.GetByID(mailboxID)
	if err != nil {
		return nil, err
	}
	if mailbox.Status != model.KefuMailboxStatusNormal {
		return nil, errors.New("邮箱已停用")
	}
	// 渠道停用后不再拉信，避免给已下线的渠道建会话
	if channel, err := s.channelRepository.GetByID(mailbox.ChannelID); err != nil {
		return nil, err
	} else if channel.Status != model.KefuChannelStatusNormal {
		return nil, errors.New("渠道已停用")
	}

	unlock, ok := s.lock(ctx, mailbox)
	if !ok {
		return nil, ErrMailSyncBusy
	}
	if unlock != nil {
		defer unlock()
	}

	config, err := s.mailboxService.imapConfig(ctx, mailbox)
	if err != nil {
		s.markSyncError(mailbox, err)
		return nil, err
	}
	session, err := mail.New(config).Connect()
	if err != nil {
		s.markSyncError(mailbox, err)
		return nil, err
	}
	defer session.Close()

	uidValidity, err := session.Select()
	if err != nil {
		s.markSyncError(mailbox, err)
		return nil, err
	}

	validityChanged := mailbox.UIDValidity != 0 && mailbox.UIDValidity != int64(uidValidity)
	firstSync := mailbox.LastUID == 0 || validityChanged
	var fetched []mail.Fetched
	if firstSync {
		days := setting.GetInt(configKeyMailFirstSyncDays, defaultMailFirstSyncDays)
		fetched, err = session.FetchSinceDays(days)
	} else {
		fetched, err = session.FetchSinceUID(uint32(mailbox.LastUID))
	}
	if err != nil {
		s.markSyncError(mailbox, err)
		return nil, err
	}

	result := new(model.KefuMailSyncResult)
	result.Fetched = len(fetched)
	maxUID := mailbox.LastUID
	lastError := ""
	for _, item := range fetched {
		uid := int64(item.UID)
		if uid > maxUID {
			maxUID = uid
		}
		var status string
		var handleErr error
		if validityChanged {
			status, handleErr = s.inboundService.HandleRescan(ctx, mailbox, item.UID, item.Size, item.Raw)
		} else {
			status, handleErr = s.inboundService.Handle(ctx, mailbox, item.UID, item.Size, item.Raw)
		}
		switch status {
		case model.KefuMailStatusReceived:
			result.Received++
		case model.KefuMailStatusSkipped:
			result.Skipped++
		case model.KefuMailStatusFailed:
			result.Failed++
			if handleErr != nil {
				lastError = handleErr.Error()
			}
		}
	}

	// 游标始终前进，单封失败不阻塞整批，失败记录可通过「重新解析」重跑
	fields := map[string]any{
		"last_uid":       maxUID,
		"uid_validity":   int64(uidValidity),
		"last_sync_time": model.Now(),
		"last_error":     lastError,
	}
	if err := s.mailboxRepository.UpdateFields(mailbox.MailboxID, fields); err != nil {
		log.Logger.Warn("update kefu mailbox cursor failed",
			zap.Int64("mailboxId", mailbox.MailboxID), zap.Error(err))
	}
	return result, nil
}

// markSyncError 记录最近一次同步错误，便于后台排查
func (s *KefuMailSyncService) markSyncError(mailbox *model.KefuMailbox, err error) {
	message := err.Error()
	if len([]rune(message)) > 500 {
		message = string([]rune(message)[:500])
	}
	if updateErr := s.mailboxRepository.UpdateFields(mailbox.MailboxID, map[string]any{
		"last_error":     message,
		"last_sync_time": model.Now(),
	}); updateErr != nil {
		log.Logger.Warn("mark kefu mailbox sync error failed",
			zap.Int64("mailboxId", mailbox.MailboxID), zap.Error(updateErr))
	}
}

// lock 获取邮箱同步锁，redis 不可用时退化为进程内直接放行
func (s *KefuMailSyncService) lock(
	ctx context.Context, mailbox *model.KefuMailbox,
) (func(), bool) {
	if rdb.Client == nil {
		return nil, true
	}
	interval := mailbox.SyncIntervalSec
	if interval <= 0 {
		interval = setting.GetInt(configKeyMailSyncIntervalSec, defaultMailSyncIntervalSec)
	}
	ttl := time.Duration(interval*2) * time.Second
	if ttl < 2*time.Minute {
		ttl = 2 * time.Minute
	}

	key := fmt.Sprintf("%s%d", mailSyncLockPrefix, mailbox.MailboxID)
	ok, err := rdb.Client.SetNX(ctx, key, "1", ttl).Result()
	if err != nil {
		log.Logger.Warn("lock kefu mailbox sync failed",
			zap.Int64("mailboxId", mailbox.MailboxID), zap.Error(err))
		return nil, true
	}
	if !ok {
		return nil, false
	}
	return func() {
		if err := rdb.Client.Del(context.Background(), key).Err(); err != nil {
			log.Logger.Warn("unlock kefu mailbox sync failed",
				zap.Int64("mailboxId", mailbox.MailboxID), zap.Error(err))
		}
	}, true
}
