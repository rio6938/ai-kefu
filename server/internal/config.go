package internal

import (
	"admin/server/internal/kefu"
	"admin/server/internal/system"
	"admin/server/pkg/config"
)

// LoadConfigCache 在迁移完成后把数据库里的配置加载到内存缓存
func LoadConfigCache() error {
	return system.LoadConfigCache()
}

// InitKefu 用配置文件里的 IM、向量库与跨域配置初始化客服模块的依赖，
// 并启动客服模块的后台定时任务（此时数据库与配置中心已就绪）
func InitKefu(cfg *config.Config) error {
	if err := kefu.Init(cfg.WuKongIM, cfg.Qdrant, cfg.Cors); err != nil {
		return err
	}
	kefu.StartTasks()
	return nil
}
