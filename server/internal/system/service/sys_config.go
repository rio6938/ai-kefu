package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"go.uber.org/zap"

	"admin/server/internal/system/repository"
	"admin/server/model"
	"admin/server/pkg/encrypt"
	"admin/server/pkg/log"
	"admin/server/pkg/setting"
)

// 常见的业务错误
var (
	// ErrConfigGroupRequired 是配置分组不能为空
	ErrConfigGroupRequired = errors.New("配置分组不能为空")
	// ErrConfigItemRequired 是配置项不能为空
	ErrConfigItemRequired = errors.New("配置项不能为空")
	// ErrConfigNotFound 是配置项不存在
	ErrConfigNotFound = errors.New("配置项不存在")
	// ErrConfigValueInvalid 是配置值格式不正确
	ErrConfigValueInvalid = errors.New("配置值不正确")
)

// SysConfigService 是配置中心的业务处理
type SysConfigService struct {
	repo     *repository.SysConfigRepository
	userRepo *repository.SysUserRepository
}

// NewSysConfigService 创建配置中心的业务处理
func NewSysConfigService() *SysConfigService {
	return &SysConfigService{
		repo:     repository.NewSysConfigRepository(),
		userRepo: repository.NewSysUserRepository(),
	}
}

// List 查询配置项列表，密钥类配置只返回掩码，不返回明文
func (s *SysConfigService) List(groupCode string) ([]model.Config, error) {
	list, err := s.repo.List(strings.TrimSpace(groupCode))
	if err != nil {
		return nil, err
	}
	s.fillSecretMask(list)
	s.fillUpdateUsername(list)
	return list, nil
}

// SaveGroup 按分组保存配置值，密钥类配置提交空值时保持原值不变
func (s *SysConfigService) SaveGroup(form model.ConfigGroupForm, userID int64) error {
	groupCode := strings.TrimSpace(form.GroupCode)
	if groupCode == "" {
		return ErrConfigGroupRequired
	}
	if len(form.Items) == 0 {
		return ErrConfigItemRequired
	}

	existing, err := s.repo.List(groupCode)
	if err != nil {
		return err
	}
	configs := make(map[string]model.Config, len(existing))
	for _, item := range existing {
		configs[item.ConfigKey] = item
	}

	values := make(map[string]string, len(form.Items))
	for _, item := range form.Items {
		configKey := strings.TrimSpace(item.ConfigKey)
		if configKey == "" {
			return ErrConfigItemRequired
		}
		config, ok := configs[configKey]
		if !ok {
			return fmt.Errorf("%w: %s", ErrConfigNotFound, configKey)
		}

		value := item.ConfigValue
		if config.IsSecret == 1 {
			// 密钥类配置留空表示不修改，避免前端回显明文
			if strings.TrimSpace(value) == "" {
				continue
			}
			encrypted, err := encrypt.Encrypt(value)
			if err != nil {
				return err
			}
			values[configKey] = encrypted
			continue
		}

		value = strings.TrimSpace(value)
		if err := validateConfigValue(config, value); err != nil {
			return err
		}
		values[configKey] = value
	}
	if len(values) == 0 {
		return nil
	}

	if err := s.repo.UpdateValues(values, userID, model.Now()); err != nil {
		return err
	}
	return s.Reload()
}

// Reload 把数据库里的配置加载到内存缓存，并触发依赖配置的客户端重建
func (s *SysConfigService) Reload() error {
	list, err := s.repo.List("")
	if err != nil {
		return err
	}

	items := make(map[string]setting.Item, len(list))
	for _, config := range list {
		value, err := decryptValue(config)
		if err != nil {
			log.Logger.Error("配置项解密失败，本次加载跳过该项",
				zap.String("key", config.ConfigKey), zap.Error(err))
			continue
		}
		items[config.ConfigKey] = setting.Item{
			Value:    value,
			IsSecret: config.IsSecret == 1,
			IsPublic: config.IsPublic == 1,
		}
	}
	setting.Replace(items)
	return nil
}

// fillSecretMask 把密钥类配置的明文换成掩码，避免接口输出密钥
func (s *SysConfigService) fillSecretMask(list []model.Config) {
	for i := range list {
		if list[i].IsSecret != 1 {
			continue
		}
		value, err := decryptValue(list[i])
		if err != nil {
			log.Logger.Error("配置项解密失败",
				zap.String("key", list[i].ConfigKey), zap.Error(err))
		}
		list[i].ConfigValue = ""
		list[i].SecretMask = encrypt.Mask(value)
	}
}

// fillUpdateUsername 回填最后修改人的账号
func (s *SysConfigService) fillUpdateUsername(list []model.Config) {
	if len(list) == 0 {
		return
	}
	userIDs := make([]int64, 0, len(list))
	seen := make(map[int64]struct{}, len(list))
	for _, item := range list {
		if item.UpdateUserID == 0 {
			continue
		}
		if _, ok := seen[item.UpdateUserID]; ok {
			continue
		}
		seen[item.UpdateUserID] = struct{}{}
		userIDs = append(userIDs, item.UpdateUserID)
	}
	if len(userIDs) == 0 {
		return
	}

	users, err := s.userRepo.ListByIDs(userIDs)
	if err != nil {
		log.Logger.Warn("回填配置修改人失败", zap.Error(err))
		return
	}
	usernames := make(map[int64]string, len(users))
	for _, user := range users {
		usernames[user.UserID] = user.Username
	}
	for i := range list {
		list[i].UpdateUsername = usernames[list[i].UpdateUserID]
	}
}

// decryptValue 解密配置值，非密钥类配置原样返回
func decryptValue(config model.Config) (string, error) {
	if config.IsSecret != 1 {
		return config.ConfigValue, nil
	}
	return encrypt.Decrypt(config.ConfigValue)
}

// validateConfigValue 按配置项的值类型校验提交的值
func validateConfigValue(config model.Config, value string) error {
	var err error
	switch config.ValueType {
	case model.ConfigTypeInt:
		_, err = strconv.Atoi(value)
	case model.ConfigTypeFloat:
		_, err = strconv.ParseFloat(value, 64)
	case model.ConfigTypeBool:
		_, err = strconv.ParseBool(value)
	case model.ConfigTypeJSON:
		if !json.Valid([]byte(value)) {
			err = errors.New("不是合法的 json")
		}
	}
	if err != nil {
		return fmt.Errorf("%w: %s", ErrConfigValueInvalid, config.ConfigName)
	}
	return nil
}
