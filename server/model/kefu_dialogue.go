package model

import "admin/server/internal/common"

// 对话库条目的来源
const (
	// KefuDialogueSourceManual 是手工录入
	KefuDialogueSourceManual = "manual"
	// KefuDialogueSourceSession 是从会话记录导入
	KefuDialogueSourceSession = "session"
)

// 对话库条目的角色，统一成客户与客服两方
const (
	// KefuDialogueRoleVisitor 是客户
	KefuDialogueRoleVisitor = "visitor"
	// KefuDialogueRoleAgent 是客服
	KefuDialogueRoleAgent = "agent"
)

// 对话库条目状态
const (
	// KefuDialogueStatusNormal 是启用
	KefuDialogueStatusNormal = 0
	// KefuDialogueStatusDisabled 是停用
	KefuDialogueStatusDisabled = 1
)

// KefuDialogue 是对话库中的一段优质对话，对应 kefu_dialogue 表；
// 可绑定关联链，绑定时只作为该链会话的示例，route_id 为 0 时对所有会话通用
type KefuDialogue struct {
	// 对话id
	DialogueID int64 `gorm:"column:dialogue_id;primaryKey;autoIncrement;comment:对话id" json:"dialogueId"`
	// 标题, 如 退款流程标准问答
	Title string `gorm:"column:title;size:128;not null;comment:标题" json:"title"`
	// 客户问题摘要, 列表展示与示例说明用
	Question string `gorm:"column:question;size:512;comment:客户问题摘要" json:"question"`
	// 绑定的关联链id, 0 表示不绑定, 对所有会话通用
	RouteID int64 `gorm:"column:route_id;not null;default:0;index;comment:绑定的关联链id" json:"routeId"`
	// 来源, manual 手工录入, session 会话导入
	Source string `gorm:"column:source;size:16;not null;comment:来源" json:"source"`
	// 导入来源的会话id, 0 表示非会话导入
	SourceSessionID int64 `gorm:"column:source_session_id;not null;default:0;comment:导入来源的会话id" json:"sourceSessionId"`
	// 状态, 0启用, 1停用
	Status int `gorm:"column:status;not null;default:0;comment:状态, 0启用, 1停用" json:"status"`
	// 是否删除, 0否, 1是
	Deleted int `gorm:"column:deleted;not null;default:0;comment:是否删除" json:"deleted"`
	// 创建时间
	CreateTime Time `gorm:"column:create_time;comment:创建时间" json:"createTime"`
	// 更新时间
	UpdateTime Time `gorm:"column:update_time;comment:更新时间" json:"updateTime"`
	// 绑定的关联链名称, 查询时回填
	RouteName string `gorm:"-" json:"routeName"`
	// 对话内容, 查询时回填
	Messages []KefuDialogueMessage `gorm:"-" json:"messages"`
	// 对话轮数, 查询时回填
	MessageCount int `gorm:"-" json:"messageCount"`
}

// TableName 指定表名
func (KefuDialogue) TableName() string {
	return "kefu_dialogue"
}

// KefuDialogueMessage 是对话库的一段对话内容，对应 kefu_dialogue_message 表；
// 修改对话时整体替换，不做软删除
type KefuDialogueMessage struct {
	// 对话内容id
	DialogueMessageID int64 `gorm:"column:dialogue_message_id;primaryKey;autoIncrement;comment:对话内容id" json:"dialogueMessageId"`
	// 对话id
	DialogueID int64 `gorm:"column:dialogue_id;not null;index;comment:对话id" json:"dialogueId"`
	// 排序号, 按对话顺序递增
	SortNumber int `gorm:"column:sort_number;not null;default:0;comment:排序号" json:"sortNumber"`
	// 角色, visitor 客户, agent 客服
	Role string `gorm:"column:role;size:16;not null;comment:角色" json:"role"`
	// 对话内容
	Content string `gorm:"column:content;type:text;not null;comment:对话内容" json:"content"`
	// 创建时间
	CreateTime Time `gorm:"column:create_time;comment:创建时间" json:"createTime"`
	// 更新时间
	UpdateTime Time `gorm:"column:update_time;comment:更新时间" json:"updateTime"`
}

// TableName 指定表名
func (KefuDialogueMessage) TableName() string {
	return "kefu_dialogue_message"
}

// KefuDialogueParam 是对话库的查询参数
type KefuDialogueParam struct {
	common.PageParam
	// 标题
	Title string `form:"title"`
	// 绑定的关联链id, 传 0 表示只查通用对话
	RouteID *int64 `form:"routeId"`
	// 来源
	Source string `form:"source"`
	// 状态, 0启用, 1停用
	Status *int
}

// KefuDialogueForm 是对话库的编辑提交数据；
// 从会话导入时先由接口返回草稿，运营修改后再提交，不会直接入库
type KefuDialogueForm struct {
	// 对话id
	DialogueID int64 `json:"dialogueId"`
	// 标题
	Title string `json:"title"`
	// 客户问题摘要
	Question string `json:"question"`
	// 绑定的关联链id, 0 表示不绑定
	RouteID int64 `json:"routeId"`
	// 导入来源的会话id, 0 表示手工录入
	SourceSessionID int64 `json:"sourceSessionId"`
	// 状态, 0启用, 1停用
	Status int `json:"status"`
	// 对话内容, 按数组顺序保存
	Messages []KefuDialogueMessageForm `json:"messages"`
}

// KefuDialogueMessageForm 是对话库里的一句对话
type KefuDialogueMessageForm struct {
	// 角色, visitor 客户, agent 客服
	Role string `json:"role"`
	// 对话内容
	Content string `json:"content"`
}

// KefuDialogueStatusForm 是对话库状态开关的提交数据
type KefuDialogueStatusForm struct {
	// 对话id
	DialogueID int64 `json:"dialogueId"`
	// 状态, 0启用, 1停用
	Status int `json:"status"`
}
