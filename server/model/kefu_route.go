package model

import "admin/server/internal/common"

// 关联链状态
const (
	// KefuRouteStatusNormal 是正常
	KefuRouteStatusNormal = 0
	// KefuRouteStatusDisabled 是停用
	KefuRouteStatusDisabled = 1
)

// KefuRoute 是关联链，对应 kefu_route 表；
// 一条关联链 = 一个提示词模板 + 若干文档库分类 + 若干渠道，
// 会话建立时按渠道快照 route_id，之后提示词与检索范围都取自该快照
type KefuRoute struct {
	// 关联链id
	RouteID int64 `gorm:"column:route_id;primaryKey;autoIncrement;comment:关联链id" json:"routeId"`
	// 关联链名称
	Name string `gorm:"column:name;size:128;not null;comment:关联链名称" json:"name"`
	// 绑定的提示词模板id
	PromptID int64 `gorm:"column:prompt_id;not null;default:0;comment:绑定的提示词模板id" json:"promptId"`
	// 关联链级覆盖提示词, 追加在模板之上
	PromptOverride string `gorm:"column:prompt_override;type:text;comment:关联链级覆盖提示词" json:"promptOverride"`
	// 状态, 0正常, 1停用
	Status int `gorm:"column:status;not null;default:0;comment:状态, 0正常, 1停用" json:"status"`
	// 是否删除, 0否, 1是
	Deleted int `gorm:"column:deleted;not null;default:0;comment:是否删除" json:"deleted"`
	// 创建时间
	CreateTime Time `gorm:"column:create_time;comment:创建时间" json:"createTime"`
	// 更新时间
	UpdateTime Time `gorm:"column:update_time;comment:更新时间" json:"updateTime"`
	// 提示词模板名称, 查询时回填
	PromptName string `gorm:"-" json:"promptName"`
	// 关联的文档库分类id, 查询时回填
	CategoryIDs []int64 `gorm:"-" json:"categoryIds"`
	// 关联的渠道id, 查询时回填
	ChannelIDs []int64 `gorm:"-" json:"channelIds"`
	// 关联的渠道名称, 查询时回填
	ChannelNames []string `gorm:"-" json:"channelNames"`
	// 关联的文档库分类名称, 查询时回填
	CategoryNames []string `gorm:"-" json:"categoryNames"`
}

// TableName 指定表名
func (KefuRoute) TableName() string {
	return "kefu_route"
}

// KefuRouteCategory 是关联链与文档库分类的关联，对应 kefu_route_category 表
type KefuRouteCategory struct {
	// 关联id
	RouteCategoryID int64 `gorm:"column:route_category_id;primaryKey;autoIncrement;comment:关联id" json:"routeCategoryId"`
	// 关联链id
	RouteID int64 `gorm:"column:route_id;not null;uniqueIndex:uk_route_category,priority:1;comment:关联链id" json:"routeId"`
	// 文档库分类id
	CategoryID int64 `gorm:"column:category_id;not null;uniqueIndex:uk_route_category,priority:2;comment:文档库分类id" json:"categoryId"`
}

// TableName 指定表名
func (KefuRouteCategory) TableName() string {
	return "kefu_route_category"
}

// KefuRouteChannel 是关联链与渠道的关联，对应 kefu_route_channel 表；
// channel_id 单独唯一，保证一个渠道只能属于一条关联链
type KefuRouteChannel struct {
	// 关联id
	RouteChannelID int64 `gorm:"column:route_channel_id;primaryKey;autoIncrement;comment:关联id" json:"routeChannelId"`
	// 关联链id
	RouteID int64 `gorm:"column:route_id;not null;comment:关联链id" json:"routeId"`
	// 渠道id, 一个渠道只能绑定一条关联链
	ChannelID int64 `gorm:"column:channel_id;not null;uniqueIndex;comment:渠道id" json:"channelId"`
}

// TableName 指定表名
func (KefuRouteChannel) TableName() string {
	return "kefu_route_channel"
}

// KefuRouteParam 是关联链的查询参数
type KefuRouteParam struct {
	common.PageParam
	// 关联链名称
	Name string `form:"name"`
	// 状态, 0正常, 1停用
	Status *int `form:"status"`
	// 渠道id, 按渠道筛选
	ChannelID *int64 `form:"channelId"`
}

// KefuRouteForm 是关联链的编辑提交数据
type KefuRouteForm struct {
	// 关联链id
	RouteID int64 `json:"routeId"`
	// 关联链名称
	Name string `json:"name"`
	// 绑定的提示词模板id
	PromptID int64 `json:"promptId"`
	// 关联链级覆盖提示词
	PromptOverride string `json:"promptOverride"`
	// 关联的文档库分类id
	CategoryIDs []int64 `json:"categoryIds"`
	// 关联的渠道id
	ChannelIDs []int64 `json:"channelIds"`
	// 状态, 0正常, 1停用
	Status int `json:"status"`
}

// KefuRouteStatusForm 是关联链状态开关的提交数据
type KefuRouteStatusForm struct {
	// 关联链id
	RouteID int64 `json:"routeId"`
	// 状态, 0正常, 1停用
	Status int `json:"status"`
}
