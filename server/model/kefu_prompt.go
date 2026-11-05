package model

import "admin/server/internal/common"

// 提示词模板状态
const (
	// KefuPromptStatusNormal 是正常
	KefuPromptStatusNormal = 0
	// KefuPromptStatusDisabled 是停用
	KefuPromptStatusDisabled = 1
)

// 是否为默认模板
const (
	// KefuPromptNotDefault 是非默认模板
	KefuPromptNotDefault = 0
	// KefuPromptIsDefault 是默认模板, 全系统只能有一个
	KefuPromptIsDefault = 1
)

// KefuPrompt 是提示词模板，对应 kefu_prompt 表
type KefuPrompt struct {
	// 模板id
	PromptID int64 `gorm:"column:prompt_id;primaryKey;autoIncrement;comment:模板id" json:"promptId"`
	// 模板名称
	Name string `gorm:"column:name;size:128;not null;comment:模板名称" json:"name"`
	// 系统提示词
	SystemPrompt string `gorm:"column:system_prompt;type:text;comment:系统提示词" json:"systemPrompt"`
	// 角色提示词, 描述客服的知识边界与话术
	RolePrompt string `gorm:"column:role_prompt;type:text;comment:角色提示词" json:"rolePrompt"`
	// 模板里用到的变量, json 数组
	Variables string `gorm:"column:variables;type:text;comment:模板里用到的变量" json:"variables"`
	// 是否为默认模板, 0否, 1是
	IsDefault int `gorm:"column:is_default;not null;default:0;comment:是否为默认模板" json:"isDefault"`
	// 版本号
	Version int `gorm:"column:version;not null;default:1;comment:版本号" json:"version"`
	// 状态, 0正常, 1停用
	Status int `gorm:"column:status;not null;default:0;comment:状态, 0正常, 1停用" json:"status"`
	// 是否删除, 0否, 1是
	Deleted int `gorm:"column:deleted;not null;default:0;comment:是否删除" json:"deleted"`
	// 创建时间
	CreateTime Time `gorm:"column:create_time;comment:创建时间" json:"createTime"`
	// 更新时间
	UpdateTime Time `gorm:"column:update_time;comment:更新时间" json:"updateTime"`
}

// TableName 指定表名
func (KefuPrompt) TableName() string {
	return "kefu_prompt"
}

// KefuPromptParam 是提示词模板的查询参数
type KefuPromptParam struct {
	common.PageParam
	// 模板名称
	Name string `form:"name"`
	// 状态
	Status *int `form:"status"`
	// 是否为默认模板
	IsDefault *int `form:"isDefault"`
}

// KefuPromptForm 是提示词模板的编辑提交数据
type KefuPromptForm struct {
	// 模板id
	PromptID int64 `json:"promptId"`
	// 模板名称
	Name string `json:"name"`
	// 系统提示词
	SystemPrompt string `json:"systemPrompt"`
	// 角色提示词
	RolePrompt string `json:"rolePrompt"`
	// 是否为默认模板, 0否, 1是
	IsDefault int `json:"isDefault"`
	// 状态, 0正常, 1停用
	Status int `json:"status"`
}
