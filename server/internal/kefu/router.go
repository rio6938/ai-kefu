package kefu

import (
	"github.com/gin-gonic/gin"

	"admin/server/internal/kefu/handler"
	"admin/server/internal/middleware"
)

// Register 注册客户端接口，路径前缀为 /kefu；
// 这些接口免登录，跨域中间件只挂在这一组上
func Register(rg *gin.RouterGroup) {
	rg.Use(middleware.Cors(corsConfig))

	// 渠道公开配置与访客接入，聊天窗免登录调用
	rg.GET("/channel/:code/config", handler.KefuChannelConfig)
	rg.POST("/bootstrap", handler.KefuVisitorBootstrap)
	// 访客浏览轨迹上报，开关未开启时后端直接忽略
	rg.POST("/visitor/browse", handler.KefuVisitorTrack)

	// 会话查询，visitorKey 用于校验会话归属
	rg.GET("/session/:id", handler.KefuClientSessionDetail)
	rg.GET("/session/:id/messages", handler.KefuClientSessionMessages)
	rg.POST("/session/:id/read", handler.KefuClientSessionRead)
	// 访客主动转人工，AI 进行中时点「转人工」调用
	rg.POST("/session/:id/handoff", handler.KefuClientSessionHandoff)
	// 访客评价会话，会话结束后调用
	rg.POST("/session/:id/rate", handler.KefuClientSessionRate)
	// 访客提交留言，无人工客服在线或会话转留言时调用
	rg.POST("/session/:id/leave-message", handler.KefuClientSessionLeaveMessage)
	// 访客正在输入，客户端节流 3s 上报一次
	rg.POST("/session/:id/typing", handler.KefuClientSessionTyping)

	// 访客快捷问题，按渠道校验渠道可用
	rg.GET("/quick-reply", handler.KefuQuickReplyVisitorList)

	// 访客附件上传与读取，不做逐条鉴权，靠随机文件名 + 后缀白名单 + 防穿越
	rg.POST("/file/upload", handler.KefuFileUpload)
	rg.GET("/file/raw/*path", handler.KefuFilePath)
}

// RegisterWebhook 注册 IM 回调接口，路径前缀为 /kefu/im/webhook；
// 回调免登录，用共享密钥校验
func RegisterWebhook(rg *gin.RouterGroup) {
	rg.POST("/message", handler.KefuWebhookMessage)
	rg.POST("/online", handler.KefuWebhookOnline)
}

// RegisterAdmin 注册客服端接口，路径前缀为 /admin/kefu
func RegisterAdmin(rg *gin.RouterGroup) {
	// OAuth2 回调免登录，但校验一次性 state，必须放在 auth 分组之外
	rg.GET("/mail/oauth/callback", handler.KefuMailOAuthCallback)

	// 以下路由统一要求登录，auth 分组内按资源再分组
	auth := rg.Group("", middleware.Auth())

	auth.GET("/ping", handler.KefuPing)

	// 渠道管理
	channel := auth.Group("/channel")
	channel.GET("/page", middleware.Permission("kefu:channel:list"), handler.KefuChannelPage)
	channel.GET("/list", middleware.Permission("kefu:channel:list"), handler.KefuChannelList)
	channel.POST("", middleware.Permission("kefu:channel:save"), handler.KefuChannelAdd)
	channel.PUT("", middleware.Permission("kefu:channel:save"), handler.KefuChannelUpdate)
	channel.PUT("/status", middleware.Permission("kefu:channel:save"), handler.KefuChannelUpdateStatus)

	// 邮箱配置，一个渠道一个邮箱，service 层校验类型与唯一性
	channel.GET("/:id/mailbox", middleware.Permission("kefu:channel:list"), handler.KefuMailboxDetail)
	channel.PUT("/:id/mailbox", middleware.Permission("kefu:channel:save"), handler.KefuMailboxSave)
	channel.POST("/:id/mailbox/test", middleware.Permission("kefu:channel:save"), handler.KefuMailboxTest)
	channel.POST("/:id/mailbox/sync", middleware.Permission("kefu:channel:save"), handler.KefuMailboxSync)
	channel.POST("/:id/mailbox/oauth/url", middleware.Permission("kefu:channel:save"), handler.KefuMailboxOAuthURL)
	channel.POST("/:id/mailbox/oauth/revoke", middleware.Permission("kefu:channel:save"), handler.KefuMailboxOAuthRevoke)

	// 邮箱服务商预设与发信记录
	mailGroup := auth.Group("/mail")
	mailGroup.GET("/provider", middleware.Permission("kefu:channel:list"), handler.KefuMailProviderList)
	mailGroup.GET("/log/page", middleware.Permission("kefu:mail:log:list"), handler.KefuMailLogPage)
	mailGroup.POST("/log/:id/resend", middleware.Permission("kefu:mail:log:resend"), handler.KefuMailLogResend)
	mailGroup.POST("/log/:id/reparse", middleware.Permission("kefu:channel:save"), handler.KefuMailLogReparse)
	// 邮件统计复用分析页权限，管理员可看全量
	mailGroup.GET("/analysis", middleware.Permission("kefu:analysis:admin"), handler.KefuMailAnalysis)

	// 会话与消息
	session := auth.Group("/session")
	session.GET("/page", middleware.Permission("kefu:session:list"), handler.KefuSessionPage)
	session.GET("/:id", middleware.Permission("kefu:session:list"), handler.KefuSessionDetail)
	session.GET("/:id/messages", middleware.Permission("kefu:session:list"), handler.KefuSessionMessages)
	session.GET("/:id/transfers", middleware.Permission("kefu:session:list"), handler.KefuSessionTransfers)
	// 会话内消息搜索
	session.GET("/:id/messages/search", middleware.Permission("kefu:session:list"), handler.KefuSessionMessageSearch)
	session.POST("/:id/read", middleware.Permission("kefu:session:list"), handler.KefuSessionRead)
	session.POST("/:id/message", middleware.Permission("kefu:session:send"), handler.KefuSessionSendMessage)
	// 客服正在输入，客户端节流上报
	session.POST("/:id/typing", middleware.Permission("kefu:session:send"), handler.KefuSessionTyping)
	// 领取接待只校验登录，客服角色都可以领到自己名下；指定客服由主管操作，在业务层校验
	session.PUT("/assign", handler.KefuSessionAssign)
	// 会话标签与内部备注
	session.PUT("/tags", middleware.Permission("kefu:session:tag"), handler.KefuSessionTags)
	session.PUT("/remark", middleware.Permission("kefu:session:tag"), handler.KefuSessionRemark)
	session.PUT("/transfer", middleware.Permission("kefu:session:transfer"), handler.KefuSessionTransfer)
	session.PUT("/close", middleware.Permission("kefu:session:close"), handler.KefuSessionClose)
	// AI 干预：开关、中断与提示词
	session.PUT("/ai-mode", middleware.Permission("kefu:session:ai-mode"), handler.KefuSessionAiMode)
	session.PUT("/ai-prompt", middleware.Permission("kefu:session:ai-mode"), handler.KefuSessionAiPromptSave)
	session.GET("/:id/ai-prompt", middleware.Permission("kefu:session:ai-mode"), handler.KefuSessionAiPrompt)
	session.POST("/:id/ai-interrupt", middleware.Permission("kefu:session:ai-mode"), handler.KefuSessionAiInterrupt)
	// 邮件草稿：查询、采用并发送、重新生成、丢弃
	session.GET("/:id/mail-draft", middleware.Permission("kefu:session:list"), handler.KefuMailDraftDetail)
	session.POST("/:id/mail-draft/send", middleware.Permission("kefu:session:send"), handler.KefuMailDraftSend)
	session.POST("/:id/mail-draft/regenerate", middleware.Permission("kefu:session:ai-mode"), handler.KefuMailDraftRegenerate)
	session.POST("/:id/mail-draft/discard", middleware.Permission("kefu:session:ai-mode"), handler.KefuMailDraftDiscard)
	// 右键 AI 生成草稿，前端用 useAiSdk 消费，所以路径与 /chat/completions 对齐
	session.POST("/:id/chat/completions", middleware.Permission("kefu:session:ai-draft"), handler.KefuSessionAiDraft)

	// 提示词模板
	prompt := auth.Group("/prompt")
	prompt.GET("/page", middleware.Permission("kefu:prompt:list"), handler.KefuPromptPage)
	prompt.GET("/list", middleware.Permission("kefu:prompt:list"), handler.KefuPromptList)
	prompt.POST("", middleware.Permission("kefu:prompt:save"), handler.KefuPromptAdd)
	prompt.PUT("", middleware.Permission("kefu:prompt:save"), handler.KefuPromptUpdate)
	prompt.DELETE("/:id", middleware.Permission("kefu:prompt:remove"), handler.KefuPromptRemove)

	// 知识库
	knowledge := auth.Group("/knowledge")
	knowledge.GET("/page", middleware.Permission("kefu:knowledge:list"), handler.KefuKnowledgePage)
	knowledge.POST("/search", middleware.Permission("kefu:knowledge:list"), handler.KefuKnowledgeSearch)
	knowledge.POST("", middleware.Permission("kefu:knowledge:save"), handler.KefuKnowledgeAdd)
	knowledge.PUT("", middleware.Permission("kefu:knowledge:save"), handler.KefuKnowledgeUpdate)
	knowledge.POST("/:id/reindex", middleware.Permission("kefu:knowledge:save"), handler.KefuKnowledgeReindex)
	knowledge.DELETE("/:id", middleware.Permission("kefu:knowledge:remove"), handler.KefuKnowledgeRemove)

	// 文档库分类，复用知识库的权限码
	category := auth.Group("/knowledge-category")
	category.GET("/page", middleware.Permission("kefu:knowledge:list"), handler.KefuKnowledgeCategoryPage)
	category.GET("/list", middleware.Permission("kefu:knowledge:list"), handler.KefuKnowledgeCategoryList)
	category.POST("", middleware.Permission("kefu:knowledge:save"), handler.KefuKnowledgeCategoryAdd)
	category.PUT("", middleware.Permission("kefu:knowledge:save"), handler.KefuKnowledgeCategoryUpdate)
	category.PUT("/status", middleware.Permission("kefu:knowledge:save"), handler.KefuKnowledgeCategoryUpdateStatus)
	category.DELETE("/:id", middleware.Permission("kefu:knowledge:remove"), handler.KefuKnowledgeCategoryRemove)

	// 关联链
	route := auth.Group("/route")
	route.GET("/page", middleware.Permission("kefu:route:list"), handler.KefuRoutePage)
	route.GET("/list", middleware.Permission("kefu:route:list"), handler.KefuRouteList)
	route.GET("/:id", middleware.Permission("kefu:route:list"), handler.KefuRouteDetail)
	route.POST("", middleware.Permission("kefu:route:save"), handler.KefuRouteAdd)
	route.PUT("", middleware.Permission("kefu:route:save"), handler.KefuRouteUpdate)
	route.PUT("/status", middleware.Permission("kefu:route:save"), handler.KefuRouteUpdateStatus)
	route.DELETE("/:id", middleware.Permission("kefu:route:remove"), handler.KefuRouteRemove)

	// 对话库，优质对话示例，可绑定关联链后喂给 AI
	dialogue := auth.Group("/dialogue")
	dialogue.GET("/page", middleware.Permission("kefu:dialogue:list"), handler.KefuDialoguePage)
	dialogue.GET("/session-draft", middleware.Permission("kefu:dialogue:list"), handler.KefuDialogueSessionDraft)
	dialogue.GET("/:id", middleware.Permission("kefu:dialogue:list"), handler.KefuDialogueDetail)
	dialogue.POST("", middleware.Permission("kefu:dialogue:save"), handler.KefuDialogueAdd)
	dialogue.PUT("", middleware.Permission("kefu:dialogue:save"), handler.KefuDialogueUpdate)
	dialogue.PUT("/status", middleware.Permission("kefu:dialogue:save"), handler.KefuDialogueUpdateStatus)
	dialogue.DELETE("/:id", middleware.Permission("kefu:dialogue:remove"), handler.KefuDialogueRemove)

	// 客服档案
	agent := auth.Group("/agent")
	agent.GET("/page", middleware.Permission("kefu:agent:list"), handler.KefuAgentPage)
	agent.GET("/list", middleware.Permission("kefu:agent:list"), handler.KefuAgentList)
	agent.POST("", middleware.Permission("kefu:agent:save"), handler.KefuAgentAdd)
	agent.PUT("", middleware.Permission("kefu:agent:update"), handler.KefuAgentUpdate)
	agent.PUT("/status", middleware.Permission("kefu:agent:update"), handler.KefuAgentUpdateStatus)
	// 坐席自助切换工作状态，只改自己的状态，所以只要求登录
	agent.PUT("/work-status", handler.KefuAgentWorkStatus)
	agent.DELETE("/:id", middleware.Permission("kefu:agent:remove"), handler.KefuAgentRemove)

	// 模型配置
	model := auth.Group("/model")
	model.GET("/page", middleware.Permission("kefu:model:list"), handler.KefuModelPage)
	model.GET("/list", middleware.Permission("kefu:model:list"), handler.KefuModelList)
	model.POST("", middleware.Permission("kefu:model:save"), handler.KefuModelAdd)
	model.PUT("", middleware.Permission("kefu:model:save"), handler.KefuModelUpdate)
	model.PUT("/status", middleware.Permission("kefu:model:save"), handler.KefuModelUpdateStatus)
	model.POST("/:id/test", middleware.Permission("kefu:model:save"), handler.KefuModelTest)
	model.DELETE("/:id", middleware.Permission("kefu:model:remove"), handler.KefuModelRemove)

	// 对外 API 密钥
	apiKey := auth.Group("/api-key")
	apiKey.GET("/page", middleware.Permission("kefu:apikey:list"), handler.KefuApiKeyPage)
	apiKey.POST("", middleware.Permission("kefu:apikey:save"), handler.KefuApiKeyIssue)
	apiKey.PUT("/status", middleware.Permission("kefu:apikey:save"), handler.KefuApiKeyUpdateStatus)

	// 分析页，管理员视角看全量、客服视角看自己名下
	analysis := auth.Group("/analysis")
	analysis.GET("/overview", middleware.Permission("kefu:analysis:admin"), handler.KefuAnalysisOverview)
	analysis.GET("/agent", middleware.Permission("kefu:analysis:agent"), handler.KefuAnalysisAgent)

	// 工作台
	workbench := auth.Group("/workbench")
	workbench.GET("/summary", middleware.Permission("kefu:session:list"), handler.KefuWorkbenchSummary)
	// 客服连接 IM 的凭据，工作台进入时拉取
	workbench.GET("/im-token", handler.KefuAgentImCredential)

	// 翻译，客户消息自动翻译与输入框翻译按钮共用
	auth.POST("/translate", middleware.Permission("kefu:session:translate"), handler.KefuTranslate)

	// 离线留言，访客提交后在这里处理
	leaveMessage := auth.Group("/leave-message")
	leaveMessage.GET("/page", middleware.Permission("kefu:leave:list"), handler.KefuMessageLeavePage)
	leaveMessage.PUT("/handle", middleware.Permission("kefu:leave:handle"), handler.KefuMessageLeaveHandle)

	// 快捷回复，访客快捷问题与客服快捷话术共用
	quickReply := auth.Group("/quick-reply")
	quickReply.GET("/page", middleware.Permission("kefu:quickreply:list"), handler.KefuQuickReplyPage)
	quickReply.GET("/list", middleware.Permission("kefu:quickreply:list"), handler.KefuQuickReplyList)
	quickReply.POST("", middleware.Permission("kefu:quickreply:save"), handler.KefuQuickReplyAdd)
	quickReply.PUT("", middleware.Permission("kefu:quickreply:save"), handler.KefuQuickReplyUpdate)
	quickReply.PUT("/status", middleware.Permission("kefu:quickreply:save"), handler.KefuQuickReplyUpdateStatus)
	quickReply.POST("/:id/use", middleware.Permission("kefu:quickreply:list"), handler.KefuQuickReplyUse)
	quickReply.DELETE("/:id", middleware.Permission("kefu:quickreply:remove"), handler.KefuQuickReplyRemove)
}

// RegisterOpenAI 注册 OpenAI 兼容接口，路径前缀为 /v1；
// 该路由族不使用统一响应结构，直接返回 OpenAI 规范错误体，
// 鉴权用 Authorization: Bearer <kefu_api_key>
func RegisterOpenAI(rg *gin.RouterGroup) {
	rg.Use(middleware.Cors(corsConfig))

	v1 := rg.Group("", handler.KefuOpenAIAuth)
	v1.POST("/chat/completions", handler.KefuOpenAIChatCompletions)
	v1.GET("/models", handler.KefuOpenAIModels)
}
