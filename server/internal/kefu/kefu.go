// Package kefu 是 AI 智能客服模块，内部分 handler / service / repository 三层，
// 并按客户端、IM 回调、客服端、OpenAI 兼容四组路由对外提供服务
package kefu

import (
	"admin/server/internal/kefu/im"
	"admin/server/internal/kefu/service"
	"admin/server/internal/kefu/vector"
	"admin/server/pkg/config"
)

// corsConfig 是跨域配置，只对 /kefu 与 /v1 公开路由生效，/admin 一律不开放跨域
var corsConfig config.Cors

// Init 用配置文件里的 IM、向量库与跨域配置初始化客服模块的依赖；
// 向量库不可达只记 Warn，不阻塞服务启动
func Init(imCfg config.WuKongIM, vectorCfg config.Qdrant, corsCfg config.Cors) error {
	if err := im.Init(imCfg); err != nil {
		return err
	}
	vector.Init(vectorCfg)
	corsConfig = corsCfg
	return nil
}

// StartTasks 启动客服模块的后台定时任务，需要数据库与 redis 已就绪，
// 由应用层在迁移与配置加载完成后调用，Init 本身不启动任何协程
func StartTasks() {
	service.NewKefuAssignService().StartQueueTimeoutTask()
	// 知识库向量化任务需要数据库、redis 与配置中心就绪
	service.NewKefuKnowledgeService().StartEmbedTask()
	// 访客附件保留期清理任务，只清 create_user_id = 0 的访客上传
	service.NewKefuFileService().StartCleanTask()
	// 邮箱渠道的 IMAP 收信 worker，按邮箱配置的间隔增量拉取
	service.NewKefuMailSyncService().StartTasks()
	// 邮箱自动回复的延时发送与日常维护任务
	service.NewKefuMailAutoService().StartTasks()
}
