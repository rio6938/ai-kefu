package internal

import (
	"admin/server/internal/kefu"
	"admin/server/internal/system"
	"admin/server/internal/user"
)

// Migrate 迁移所有模块的表结构并初始化基础数据，具体实现由各模块自行提供
func Migrate() error {
	if err := system.Migrate(); err != nil {
		return err
	}
	if err := user.Migrate(); err != nil {
		return err
	}
	return kefu.Migrate()
}
