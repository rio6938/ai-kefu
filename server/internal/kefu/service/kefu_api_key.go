package service

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"go.uber.org/zap"

	"admin/server/internal/kefu/repository"
	"admin/server/model"
	"admin/server/pkg/log"
)

// API 密钥的固定前缀与长度
const (
	// apiKeyPrefix 是签发给调用方的密钥前缀，便于在日志与列表里识别
	apiKeyPrefix = "kefu_sk_"
	// apiKeyRandomBytes 是密钥随机部分的字节数, 十六进制后为 48 位
	apiKeyRandomBytes = 24
	// apiKeyDisplayPrefixLen 是库里保存的密钥前缀长度
	apiKeyDisplayPrefixLen = 16
)

// 常见的业务错误
var (
	// ErrApiKeyNotFound 是 API 密钥不存在
	ErrApiKeyNotFound = repository.ErrApiKeyNotFound
	// ErrApiKeyNameRequired 是密钥名称不能为空
	ErrApiKeyNameRequired = errors.New("密钥名称不能为空")
	// ErrApiKeyStatusInvalid 是密钥状态不正确
	ErrApiKeyStatusInvalid = errors.New("密钥状态不正确")
	// ErrApiKeyExpireInvalid 是过期时间格式不正确
	ErrApiKeyExpireInvalid = errors.New("过期时间格式不正确")
	// ErrApiKeyInvalid 是密钥不存在、已停用或已过期
	ErrApiKeyInvalid = errors.New("API 密钥不正确或已失效")
)

// KefuApiKeyService 是对外 API 密钥的业务处理
type KefuApiKeyService struct {
	// 对外 API 密钥的数据访问
	apiKeyRepository *repository.KefuApiKeyRepository
}

// NewKefuApiKeyService 创建对外 API 密钥的业务处理
func NewKefuApiKeyService() *KefuApiKeyService {
	return &KefuApiKeyService{
		apiKeyRepository: repository.NewKefuApiKeyRepository(),
	}
}

// Page 分页查询 API 密钥，输出前清掉密钥哈希
func (s *KefuApiKeyService) Page(
	param model.KefuApiKeyParam,
) ([]model.KefuApiKey, int64, error) {
	list, count, err := s.apiKeyRepository.Page(param, param.PageParam)
	if err != nil {
		return nil, 0, err
	}
	for i := range list {
		list[i].KeyHash = ""
	}
	return list, count, nil
}

// Issue 签发 API 密钥，明文只在签发时返回一次，库里只存前缀与哈希
func (s *KefuApiKeyService) Issue(form model.KefuApiKeyForm) (string, error) {
	name := strings.TrimSpace(form.Name)
	if name == "" {
		return "", ErrApiKeyNameRequired
	}

	expireTime, err := parseApiKeyExpireTime(form.ExpireTime)
	if err != nil {
		return "", err
	}

	plain := apiKeyPrefix + randomHex(apiKeyRandomBytes)
	now := model.Now()
	record := &model.KefuApiKey{
		Name:       name,
		KeyPrefix:  plain[:apiKeyDisplayPrefixLen],
		KeyHash:    hashApiKey(plain),
		Status:     model.KefuApiKeyStatusNormal,
		ExpireTime: expireTime,
		CreateTime: now,
		UpdateTime: now,
	}
	if err := s.apiKeyRepository.Create(record); err != nil {
		return "", err
	}
	return plain, nil
}

// UpdateStatus 修改 API 密钥状态，停用后立即失效
func (s *KefuApiKeyService) UpdateStatus(keyID int64, status int) error {
	if _, err := s.apiKeyRepository.GetByID(keyID); err != nil {
		return err
	}
	if status != model.KefuApiKeyStatusNormal && status != model.KefuApiKeyStatusDisabled {
		return ErrApiKeyStatusInvalid
	}
	return s.apiKeyRepository.UpdateStatus(keyID, status)
}

// Verify 校验 Authorization 里的密钥，通过后刷新最后使用时间
func (s *KefuApiKeyService) Verify(rawKey string) error {
	key := strings.TrimSpace(rawKey)
	if key == "" {
		return ErrApiKeyInvalid
	}

	record, err := s.apiKeyRepository.GetByHash(hashApiKey(key))
	if err != nil {
		if errors.Is(err, repository.ErrApiKeyNotFound) {
			return ErrApiKeyInvalid
		}
		return err
	}
	if record.Status != model.KefuApiKeyStatusNormal {
		return ErrApiKeyInvalid
	}
	if !record.ExpireTime.IsZero() && record.ExpireTime.Before(time.Now()) {
		return ErrApiKeyInvalid
	}

	// 最后使用时间只做统计，写失败不影响本次调用
	if err := s.apiKeyRepository.UpdateLastUsed(record.KeyID); err != nil {
		log.Logger.Warn("update kefu api key last used time failed",
			zap.Int64("keyId", record.KeyID), zap.Error(err))
	}
	return nil
}

// hashApiKey 计算密钥的 sha256 哈希，库里只存哈希不存明文
func hashApiKey(rawKey string) string {
	sum := sha256.Sum256([]byte(rawKey))
	return hex.EncodeToString(sum[:])
}

// parseApiKeyExpireTime 解析过期时间，为空表示永不过期
func parseApiKeyExpireTime(value string) (model.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return model.Time{}, nil
	}
	parsed, err := time.ParseInLocation(model.TimeLayout, value, time.Local)
	if err != nil {
		return model.Time{}, ErrApiKeyExpireInvalid
	}
	return model.Time{Time: parsed}, nil
}
