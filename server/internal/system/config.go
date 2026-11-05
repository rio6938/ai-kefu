package system

import (
	"admin/server/internal/system/service"
)

// LoadConfigCache 把数据库里的配置加载到内存缓存，由外部在迁移完成后调用
func LoadConfigCache() error {
	return service.NewSysConfigService().Reload()
}
