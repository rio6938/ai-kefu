package user

import (
	"fmt"

	"admin/server/model"
	"admin/server/pkg/db"
)

// Migrate 自动迁移 user 模块的表结构并初始化配置项
func Migrate() error {
	if err := db.DB.AutoMigrate(
		&model.User{},
		&model.UserLoginRecord{},
		&model.UserOperationRecord{},
	); err != nil {
		return fmt.Errorf("auto migrate user tables: %w", err)
	}
	return seedConfigs()
}

// seedConfigs 初始化前台用户的配置项，配置项由本模块维护默认值
func seedConfigs() error {
	configs := []model.Config{
		{ConfigKey: "user.register.enabled", ConfigValue: "true", ValueType: model.ConfigTypeBool, ConfigName: "开放注册", GroupCode: "user", GroupName: "用户设置", SortNumber: 1, IsPublic: 1, Comments: "关闭后注册接口不可用"},
		{ConfigKey: "user.register.need_phone_verify", ConfigValue: "false", ValueType: model.ConfigTypeBool, ConfigName: "注册需手机号验证", GroupCode: "user", GroupName: "用户设置", SortNumber: 2, IsPublic: 1},
		{ConfigKey: "user.register.need_email_verify", ConfigValue: "false", ValueType: model.ConfigTypeBool, ConfigName: "注册需邮箱验证", GroupCode: "user", GroupName: "用户设置", SortNumber: 3, IsPublic: 1},
		{ConfigKey: "user.password.forget.enabled", ConfigValue: "true", ValueType: model.ConfigTypeBool, ConfigName: "开放找回密码", GroupCode: "user", GroupName: "用户设置", SortNumber: 4, IsPublic: 1, Comments: "关闭后前台不显示找回密码入口"},
		{ConfigKey: "user.password.forget.channels", ConfigValue: `["sms","email"]`, ValueType: model.ConfigTypeJSON, ConfigName: "找回密码验证方式", GroupCode: "user", GroupName: "用户设置", SortNumber: 5, IsPublic: 1, Comments: "可选 sms 或 email"},
	}
	for _, item := range configs {
		if err := seedConfig(item); err != nil {
			return err
		}
	}
	return nil
}

// seedConfig 在配置键不存在时写入配置项，保证新增的内置配置项能补进已有的库
func seedConfig(item model.Config) error {
	var count int64
	if err := db.DB.Model(&model.Config{}).
		Where("config_key = ?", item.ConfigKey).Count(&count).Error; err != nil {
		return fmt.Errorf("count sys_config %s: %w", item.ConfigKey, err)
	}
	if count > 0 {
		return nil
	}

	now := model.Now()
	item.CreateTime = now
	item.UpdateTime = now
	if err := db.DB.Create(&item).Error; err != nil {
		return fmt.Errorf("create sys_config %s: %w", item.ConfigKey, err)
	}
	return nil
}
