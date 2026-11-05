package model

import "admin/server/internal/common"

// 渠道状态
const (
	// KefuChannelStatusNormal 是正常
	KefuChannelStatusNormal = 0
	// KefuChannelStatusDisabled 是停用
	KefuChannelStatusDisabled = 1
)

// 渠道类型
const (
	// KefuChannelTypeWeb 是网页聊天窗
	KefuChannelTypeWeb = "web"
	// KefuChannelTypeHook 是 Hook 渠道, 供外部系统通过回调对接
	KefuChannelTypeHook = "hook"
	// KefuChannelTypeEmail 是邮箱渠道
	KefuChannelTypeEmail = "email"
)

// 渠道类型字典标识, 取值由字典 kefu_channel_type 约束
const KefuChannelTypeDictCode = "kefu_channel_type"

// KefuChannel 是客服渠道，对应 kefu_channel 表
type KefuChannel struct {
	// 渠道id
	ChannelID int64 `gorm:"column:channel_id;primaryKey;autoIncrement;comment:渠道id" json:"channelId"`
	// 渠道标识，嵌入代码里通过它指定渠道
	ChannelCode string `gorm:"column:channel_code;size:64;not null;uniqueIndex;comment:渠道标识" json:"channelCode"`
	// 渠道类型, web 为网页
	ChannelType string `gorm:"column:channel_type;size:32;not null;comment:渠道类型" json:"channelType"`
	// 渠道名称
	Name string `gorm:"column:name;size:128;not null;comment:渠道名称" json:"name"`
	// 欢迎语
	WelcomeText string `gorm:"column:welcome_text;size:512;comment:欢迎语" json:"welcomeText"`
	// 主题色
	ThemeColor string `gorm:"column:theme_color;size:32;comment:主题色" json:"themeColor"`
	// 是否必须登录后咨询, 0否, 1是
	NeedLogin int `gorm:"column:need_login;not null;default:0;comment:是否必须登录后咨询" json:"needLogin"`
	// 扩展配置, json 文本
	Config string `gorm:"column:config;type:text;comment:扩展配置" json:"config"`
	// 状态, 0正常, 1停用
	Status int `gorm:"column:status;not null;default:0;comment:状态, 0正常, 1停用" json:"status"`
	// 是否删除, 0否, 1是
	Deleted int `gorm:"column:deleted;not null;default:0;comment:是否删除" json:"deleted"`
	// 创建时间
	CreateTime Time `gorm:"column:create_time;comment:创建时间" json:"createTime"`
	// 更新时间
	UpdateTime Time `gorm:"column:update_time;comment:更新时间" json:"updateTime"`
	// 已绑定的关联链id, 查询时回填, 关系存在 kefu_route_channel
	RouteID int64 `gorm:"-" json:"routeId"`
	// 已绑定的关联链名称, 查询时回填
	RouteName string `gorm:"-" json:"routeName"`
	// 绑定的邮箱地址, 邮箱渠道列表回显时使用
	MailboxEmail string `gorm:"-" json:"mailboxEmail"`
}

// TableName 指定表名
func (KefuChannel) TableName() string {
	return "kefu_channel"
}

// KefuChannelParam 是渠道的查询参数
type KefuChannelParam struct {
	common.PageParam
	// 渠道标识
	ChannelCode string `form:"channelCode"`
	// 渠道类型
	ChannelType string `form:"channelType"`
	// 渠道名称
	Name string `form:"name"`
	// 状态
	Status *int `form:"status"`
}

// KefuChannelForm 是渠道的编辑提交数据
type KefuChannelForm struct {
	// 渠道id
	ChannelID int64 `json:"channelId"`
	// 渠道标识
	ChannelCode string `json:"channelCode"`
	// 渠道类型
	ChannelType string `json:"channelType"`
	// 渠道名称
	Name string `json:"name"`
	// 欢迎语
	WelcomeText string `json:"welcomeText"`
	// 主题色
	ThemeColor string `json:"themeColor"`
	// 是否必须登录后咨询, 0否, 1是
	NeedLogin int `json:"needLogin"`
	// 扩展配置, json 文本
	Config string `json:"config"`
}

// KefuChannelStatusForm 是渠道状态开关的提交数据
type KefuChannelStatusForm struct {
	// 渠道id
	ChannelID int64 `json:"channelId"`
	// 状态, 0正常, 1停用
	Status int `json:"status"`
}

// KefuChannelItem 是渠道的精简结构，关联链编辑弹窗多选渠道时使用
type KefuChannelItem struct {
	// 渠道id
	ChannelID int64 `json:"channelId"`
	// 渠道标识
	ChannelCode string `json:"channelCode"`
	// 渠道类型
	ChannelType string `json:"channelType"`
	// 渠道名称
	Name string `json:"name"`
	// 已绑定的关联链id, 0 表示未绑定
	RouteID int64 `json:"routeId"`
}

// KefuChannelConfig 是渠道对外的公开配置，聊天窗只拿这些字段；
// config 等扩展配置不下发，避免泄露敏感项
type KefuChannelConfig struct {
	// 渠道标识
	ChannelCode string `json:"channelCode"`
	// 渠道类型, web 为网页
	ChannelType string `json:"channelType"`
	// 渠道名称
	Name string `json:"name"`
	// 欢迎语
	WelcomeText string `json:"welcomeText"`
	// 主题色
	ThemeColor string `json:"themeColor"`
	// 是否必须登录后咨询, 0否, 1是
	NeedLogin int `json:"needLogin"`
	// 聊天窗地址, 供悬浮脚本拼接面板地址
	ChatURL string `json:"chatUrl"`
	// 访客附件上传开关
	FileEnabled bool `json:"fileEnabled"`
	// 单文件大小上限, 单位 MB
	FileMaxSizeMB int `json:"fileMaxSizeMb"`
	// 允许上传的附件后缀, 不含点号
	FileAllowedExt []string `json:"fileAllowedExt"`
	// 访客端「转人工」按钮开关
	HandoffEnabled bool `json:"handoffEnabled"`
	// 会话评价开关
	RateEnabled bool `json:"rateEnabled"`
}
