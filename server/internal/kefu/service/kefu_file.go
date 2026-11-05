package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"

	"admin/server/internal/kefu/repository"
	systemservice "admin/server/internal/system/service"
	"admin/server/model"
	"admin/server/pkg/log"
	rdb "admin/server/pkg/redis"
	"admin/server/pkg/setting"
)

// 附件相关的配置项，全部可在配置中心修改，改完立即生效
const (
	// configKeyFileEnabled 是访客附件上传总开关
	configKeyFileEnabled = "kefu.file.enabled"
	// configKeyFileMaxSizeMB 是单文件大小上限，单位 MB
	configKeyFileMaxSizeMB = "kefu.file.max_size_mb"
	// configKeyFileAllowedExt 是允许上传的后缀，英文逗号分隔
	configKeyFileAllowedExt = "kefu.file.allowed_ext"
	// configKeyFileIPDailyLimitMB 是单 IP 每日累计上传大小上限，单位 MB
	configKeyFileIPDailyLimitMB = "kefu.file.ip_daily_limit_mb"
	// configKeyFileKeepDays 是访客附件的保留天数，0 表示不自动清理
	configKeyFileKeepDays = "kefu.file.keep_days"
)

// 附件相关的常量
const (
	// defaultFileMaxSizeMB 是单文件大小上限的默认值
	defaultFileMaxSizeMB = 10
	// defaultFileAllowedExt 是允许后缀的默认值
	defaultFileAllowedExt = "jpg,jpeg,png,gif,webp,pdf,doc,docx,xls,xlsx,txt,zip"
	// defaultFileIPDailyLimitMB 是单 IP 每日累计上限的默认值
	defaultFileIPDailyLimitMB = 50
	// defaultFileKeepDays 是访客附件保留天数的默认值
	defaultFileKeepDays = 30
	// fileIPCounterKeyPrefix 是单 IP 每日累计大小的 redis key 前缀,
	// key 为 kefu:file:ip:{ip}:{yyyyMMdd}
	fileIPCounterKeyPrefix = "kefu:file:ip:"
	// fileIPCounterTTL 是单 IP 日计数的过期时长，跨天后自动失效
	fileIPCounterTTL = 48 * time.Hour
	// fileCleanInterval 是访客附件清理任务的执行间隔
	fileCleanInterval = 6 * time.Hour
	// fileCleanDelay 是清理任务首次执行的延迟，避免与服务启动抢资源
	fileCleanDelay = 5 * time.Minute
	// fileCleanBatch 是清理任务单批处理的记录数
	fileCleanBatch = 500
)

// 附件相关的业务错误
var (
	// ErrFileDisabled 是访客附件上传已关闭
	ErrFileDisabled = errors.New("附件上传已关闭")
	// ErrFileExtNotAllowed 是不支持的文件类型
	ErrFileExtNotAllowed = errors.New("不支持该文件类型")
	// ErrFileIPDailyLimited 是单 IP 每日额度已用完
	ErrFileIPDailyLimited = errors.New("今日附件上传额度已用完")
	// ErrFilePathInvalid 是附件路径不合法
	ErrFilePathInvalid = errors.New("附件路径不正确")
)

// KefuFileSizeError 是单文件大小超限的错误，
// 上限随配置变化，因此用带文案的类型返回
type KefuFileSizeError struct {
	// 单文件大小上限, 单位 MB
	MaxSizeMB int
}

// Error 返回中文提示文案
func (e *KefuFileSizeError) Error() string {
	return fmt.Sprintf("文件大小不能超过 %d MB", e.MaxSizeMB)
}

// KefuFileService 是访客附件的业务处理：
// 上传前按配置校验开关、后缀、单文件大小与单 IP 每日累计，
// 落库落盘复用系统模块的上传能力，上传人固定为 0（访客）
type KefuFileService struct {
	// 上传文件记录的数据访问
	fileRepository *repository.KefuFileRepository
	// 系统模块的上传能力，落库与落盘复用
	sysFileService *systemservice.SysFileService
}

// NewKefuFileService 创建访客附件的业务处理
func NewKefuFileService() *KefuFileService {
	return &KefuFileService{
		fileRepository: repository.NewKefuFileRepository(),
		sysFileService: systemservice.NewSysFileService(),
	}
}

// KefuFileUploadResult 是访客上传附件后返回的结构
type KefuFileUploadResult struct {
	// 文件记录id
	FileID int64 `json:"fileId"`
	// 文件名称
	Name string `json:"name"`
	// 文件大小, 单位字节
	Size int64 `json:"size"`
	// 附件访问地址, 聊天窗直接用它发消息
	URL string `json:"url"`
	// 文件类型
	ContentType string `json:"contentType"`
}

// Upload 保存访客上传的附件，校验不通过时不落盘也不建记录
func (s *KefuFileService) Upload(
	ip, fileName string, r io.Reader, size int64, contentType string,
) (*KefuFileUploadResult, error) {
	if !setting.GetBool(configKeyFileEnabled, true) {
		return nil, ErrFileDisabled
	}

	if !s.extAllowed(strings.ToLower(filepath.Ext(fileName))) {
		return nil, ErrFileExtNotAllowed
	}

	maxSizeMB := fileMaxSizeMB()
	if size > int64(maxSizeMB)<<20 {
		return nil, &KefuFileSizeError{MaxSizeMB: maxSizeMB}
	}

	// 单 IP 每日累计额度，redis 不可用时只记 Warn 并跳过
	over, release := s.reserveIPQuota(ip, size)
	if over {
		return nil, ErrFileIPDailyLimited
	}

	record, err := s.sysFileService.Save(fileName, r, size, contentType, 0)
	if err != nil {
		// 落盘失败要把已占用的额度还回去，避免额度被无效请求吃掉
		release()
		return nil, err
	}
	return &KefuFileUploadResult{
		FileID:      record.ID,
		Name:        record.Name,
		Size:        record.Length,
		URL:         "/kefu/file/raw/" + record.Path,
		ContentType: record.ContentType,
	}, nil
}

// RawPath 校验并返回访客附件的磁盘路径，
// 必须做 filepath.Clean 防目录穿越，并限制在后缀白名单内
func (s *KefuFileService) RawPath(relative string) (string, error) {
	value := strings.TrimPrefix(strings.TrimSpace(relative), "/")
	if value == "" {
		return "", ErrFilePathInvalid
	}

	clean := filepath.Clean(filepath.FromSlash(value))
	if clean == "." || filepath.IsAbs(clean) ||
		clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", ErrFilePathInvalid
	}
	if !s.extAllowed(strings.ToLower(filepath.Ext(clean))) {
		return "", ErrFilePathInvalid
	}

	fullPath := filepath.Join(systemservice.UploadDir, clean)
	if _, err := os.Stat(fullPath); err != nil {
		return "", ErrFilePathInvalid
	}
	return fullPath, nil
}

// StartCleanTask 启动访客附件保留期清理任务：
// 启动后延迟 5 分钟跑首次，之后每 6 小时一次
func (s *KefuFileService) StartCleanTask() {
	go func() {
		// 首次延迟执行，避开与服务启动抢资源
		time.Sleep(fileCleanDelay)
		s.cleanExpired()

		ticker := time.NewTicker(fileCleanInterval)
		defer ticker.Stop()
		for range ticker.C {
			s.cleanExpired()
		}
	}()
	log.Kernel.Info("start kefu file clean task")
}

// cleanExpired 清理一轮过期的访客附件：先删磁盘文件再软删除记录，
// 只处理 create_user_id = 0 的访客上传，客服与后台上传的文件不动
func (s *KefuFileService) cleanExpired() {
	keepDays := setting.GetInt(configKeyFileKeepDays, defaultFileKeepDays)
	if keepDays <= 0 {
		log.Logger.Debug("kefu file clean task skipped, keep_days is 0")
		return
	}

	before := model.Time{Time: time.Now().AddDate(0, 0, -keepDays)}
	scanned, removed, skipped := 0, 0, 0
	for {
		list, err := s.fileRepository.ListExpiredVisitorFiles(before, fileCleanBatch)
		if err != nil {
			log.Logger.Error("list kefu expired visitor files failed", zap.Error(err))
			break
		}
		if len(list) == 0 {
			break
		}

		ids := make([]int64, 0, len(list))
		for _, file := range list {
			scanned++
			ids = append(ids, file.ID)
			path := filepath.Join(systemservice.UploadDir, filepath.FromSlash(file.Path))
			if err := os.Remove(path); err != nil {
				// 文件已不存在时跳过，其它错误只记 Warn 不中断整轮清理
				skipped++
				if !errors.Is(err, os.ErrNotExist) {
					log.Logger.Warn("remove kefu visitor file failed",
						zap.Int64("fileId", file.ID), zap.String("path", file.Path), zap.Error(err))
				}
			}
		}
		if err := s.fileRepository.SoftDeleteByIDs(ids); err != nil {
			log.Logger.Warn("soft delete kefu visitor files failed",
				zap.Int("count", len(ids)), zap.Error(err))
			break
		}
		removed += len(ids)

		// 记录数不足一批说明已经扫完，避免再查一次空结果
		if len(list) < fileCleanBatch {
			break
		}
	}

	log.Logger.Info("kefu file clean task finished",
		zap.Int("scanned", scanned), zap.Int("removed", removed), zap.Int("skipped", skipped))
}

// reserveIPQuota 预占单 IP 每日上传额度，超限时返回 over 为 true；
// redis 不可用时只记 Warn 并跳过额度限制，返回的 release 用于回滚
func (s *KefuFileService) reserveIPQuota(ip string, size int64) (bool, func()) {
	noop := func() {}
	if rdb.Client == nil {
		log.Logger.Warn("redis unavailable, skip kefu file ip daily quota",
			zap.String("ip", ip))
		return false, noop
	}

	limit := int64(
		setting.GetInt(configKeyFileIPDailyLimitMB, defaultFileIPDailyLimitMB),
	) << 20

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key := fileIPCounterKey(ip)
	used, err := rdb.Client.IncrBy(ctx, key, size).Result()
	if err != nil {
		log.Logger.Warn("increase kefu file ip daily quota failed",
			zap.String("ip", ip), zap.Error(err))
		return false, noop
	}
	if err := rdb.Client.Expire(ctx, key, fileIPCounterTTL).Err(); err != nil {
		log.Logger.Warn("set kefu file ip daily quota ttl failed",
			zap.String("ip", ip), zap.Error(err))
	}
	if used > limit {
		// 超限时把本次占用的额度还回去，保证后续正常大小的文件还能传
		s.releaseIPQuota(key, size)
		return true, noop
	}
	return false, func() { s.releaseIPQuota(key, size) }
}

// releaseIPQuota 回滚已占用的单 IP 每日额度
func (s *KefuFileService) releaseIPQuota(key string, size int64) {
	if rdb.Client == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Client.DecrBy(ctx, key, size).Err(); err != nil {
		log.Logger.Warn("rollback kefu file ip daily quota failed",
			zap.String("key", key), zap.Error(err))
	}
}

// extAllowed 判断后缀是否在允许的上传白名单里
func (s *KefuFileService) extAllowed(ext string) bool {
	for _, item := range fileAllowedExt() {
		if "."+item == ext {
			return true
		}
	}
	return false
}

// fileIPCounterKey 返回单 IP 每日累计大小的 redis key，按日期分桶
func fileIPCounterKey(ip string) string {
	value := strings.TrimSpace(ip)
	if value == "" {
		value = "unknown"
	}
	return fileIPCounterKeyPrefix + value + ":" + time.Now().Format("20060102")
}

// fileAllowedExt 返回允许上传的后缀列表，不含点号，供前端 accept 与公开配置使用
func fileAllowedExt() []string {
	value := setting.GetString(configKeyFileAllowedExt, defaultFileAllowedExt)
	list := make([]string, 0, 8)
	for _, item := range strings.Split(value, ",") {
		allowed := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(item)), ".")
		if allowed == "" {
			continue
		}
		list = append(list, allowed)
	}
	return list
}

// fileEnabled 返回访客附件上传开关
func fileEnabled() bool {
	return setting.GetBool(configKeyFileEnabled, true)
}

// fileMaxSizeMB 返回单文件大小上限，单位 MB
func fileMaxSizeMB() int {
	value := setting.GetInt(configKeyFileMaxSizeMB, defaultFileMaxSizeMB)
	if value <= 0 {
		return defaultFileMaxSizeMB
	}
	return value
}
