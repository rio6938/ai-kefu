package model

import "admin/server/internal/common"

// 客服档案状态
const (
	// KefuAgentStatusNormal 是正常
	KefuAgentStatusNormal = 0
	// KefuAgentStatusDisabled 是停用
	KefuAgentStatusDisabled = 1
)

// 客服接单开关
const (
	// KefuAgentAcceptOff 是停止接单
	KefuAgentAcceptOff = 0
	// KefuAgentAcceptOn 是开启接单
	KefuAgentAcceptOn = 1
)

// 客服工作状态，与接单开关独立：accept_status = 0 优先于 work_status，
// 只要停止接单就不再进候选池，work_status 只表达坐席当前是否在工位
const (
	// KefuAgentWorkOffline 是离线
	KefuAgentWorkOffline = 0
	// KefuAgentWorkOnline 是在线
	KefuAgentWorkOnline = 1
	// KefuAgentWorkRest 是小休
	KefuAgentWorkRest = 2
)

// 客服模块的角色标识，与迁移种子保持一致
const (
	// KefuRoleAgent 是客服角色
	KefuRoleAgent = "kefu"
	// KefuRoleSupervisor 是客服主管角色，可以看全部会话
	KefuRoleSupervisor = "kefu_admin"
)

// KefuAgentWorkStatusDictCode 是坐席工作状态的字典标识
const KefuAgentWorkStatusDictCode = "kefu_agent_work_status"

// KefuRateTagDictCode 是会话评价标签的字典标识
const KefuRateTagDictCode = "kefu_rate_tag"

// KefuSessionTagDictCode 是会话标签的字典标识
const KefuSessionTagDictCode = "kefu_session_tag"

// KefuAgent 是客服档案，对应 kefu_agent 表；
// 客服账号本身仍是 sys_user，这里只存客服专属字段
type KefuAgent struct {
	// 客服id
	AgentID int64 `gorm:"column:agent_id;primaryKey;autoIncrement;comment:客服id" json:"agentId"`
	// 关联的管理员用户id
	UserID int64 `gorm:"column:user_id;not null;uniqueIndex;comment:关联的管理员用户id" json:"userId"`
	// 客服工号，展示用
	AgentNo string `gorm:"column:agent_no;size:64;comment:客服工号" json:"agentNo"`
	// 最大同时接待会话数
	MaxConcurrent int `gorm:"column:max_concurrent;not null;default:5;comment:最大同时接待会话数" json:"maxConcurrent"`
	// 接单开关, 0关闭, 1开启
	AcceptStatus int `gorm:"column:accept_status;not null;default:1;comment:接单开关, 0关闭, 1开启" json:"acceptStatus"`
	// 工作状态, 0离线, 1在线, 2小休
	WorkStatus int `gorm:"column:work_status;not null;default:1;comment:工作状态, 0离线, 1在线, 2小休" json:"workStatus"`
	// 小休原因, 最长 64 字
	RestReason string `gorm:"column:rest_reason;size:64;comment:小休原因" json:"restReason"`
	// 工作状态变更时间
	StatusTime Time `gorm:"column:status_time;comment:工作状态变更时间" json:"statusTime"`
	// 新会话默认开启 AI 自动回复, 0否, 1是
	AiDefault int `gorm:"column:ai_default;not null;default:1;comment:新会话默认开启AI自动回复" json:"aiDefault"`
	// 默认提示词模板id, 0 表示未设置
	PromptID int64 `gorm:"column:prompt_id;not null;default:0;comment:默认提示词模板id" json:"promptId"`
	// 该客服的欢迎语, 为空时用渠道默认欢迎语
	WelcomeText string `gorm:"column:welcome_text;size:512;comment:该客服的欢迎语" json:"welcomeText"`
	// 状态, 0正常, 1停用
	Status int `gorm:"column:status;not null;default:0;comment:状态, 0正常, 1停用" json:"status"`
	// 是否删除, 0否, 1是
	Deleted int `gorm:"column:deleted;not null;default:0;comment:是否删除" json:"deleted"`
	// 创建时间
	CreateTime Time `gorm:"column:create_time;comment:创建时间" json:"createTime"`
	// 更新时间
	UpdateTime Time `gorm:"column:update_time;comment:更新时间" json:"updateTime"`
	// 关联的管理员账号, 查询时回填
	Username string `gorm:"-" json:"username,omitempty"`
	// 关联的管理员昵称, 查询时回填
	Nickname string `gorm:"-" json:"nickname,omitempty"`
	// 当前进行中的会话数, 查询时回填
	SessionCount int64 `gorm:"-" json:"sessionCount"`
	// 是否在线, 查询时回填
	Online bool `gorm:"-" json:"online"`
}

// TableName 指定表名
func (KefuAgent) TableName() string {
	return "kefu_agent"
}

// KefuAgentParam 是客服档案的查询参数
type KefuAgentParam struct {
	common.PageParam
	// 关联的管理员用户id
	UserID *int64 `form:"userId"`
	// 客服工号
	AgentNo string `form:"agentNo"`
	// 接单开关
	AcceptStatus *int `form:"acceptStatus"`
	// 状态
	Status *int `form:"status"`
}

// KefuAgentForm 是客服档案的编辑提交数据
type KefuAgentForm struct {
	// 客服id
	AgentID int64 `json:"agentId"`
	// 关联的管理员用户id
	UserID int64 `json:"userId"`
	// 客服工号
	AgentNo string `json:"agentNo"`
	// 最大同时接待会话数
	MaxConcurrent int `json:"maxConcurrent"`
	// 接单开关, 0关闭, 1开启
	AcceptStatus int `json:"acceptStatus"`
	// 新会话默认开启 AI 自动回复, 0否, 1是
	AiDefault int `json:"aiDefault"`
	// 默认提示词模板id, 0 表示未设置
	PromptID int64 `json:"promptId"`
	// 该客服的欢迎语
	WelcomeText string `json:"welcomeText"`
}

// KefuAgentStatusForm 是客服状态开关的提交数据，status 与 acceptStatus 二选一提交
type KefuAgentStatusForm struct {
	// 客服id
	AgentID int64 `json:"agentId"`
	// 状态, 0正常, 1停用, 未提交时不修改
	Status *int `json:"status"`
	// 接单开关, 0关闭, 1开启, 未提交时不修改
	AcceptStatus *int `json:"acceptStatus"`
}

// KefuAgentWorkStatusForm 是坐席切换工作状态的提交数据，只改自己的状态
type KefuAgentWorkStatusForm struct {
	// 工作状态, 0离线, 1在线, 2小休
	WorkStatus int `json:"workStatus"`
	// 小休原因, 最长 64 字, 切在线时清空
	RestReason string `json:"restReason"`
}

// KefuImCredential 是客服端连接 IM 的凭据，客服工作台登录后拉取
type KefuImCredential struct {
	// 客服的 IM 用户标识, 形如 agent_{userId}
	UID string `json:"uid"`
	// 客服的 IM token
	Token string `json:"token"`
	// IM gateway 地址
	WsURL string `json:"wsUrl"`
	// 客服id, 没有客服档案的主管为 0
	AgentID int64 `json:"agentId"`
}
