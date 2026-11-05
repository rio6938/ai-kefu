package model

import "admin/server/internal/common"

// KefuVisitor 是网页访客，对应 kefu_visitor 表；
// 访客可以是匿名的，登录后才有 user_id
type KefuVisitor struct {
	// 访客id
	VisitorID int64 `gorm:"column:visitor_id;primaryKey;autoIncrement;comment:访客id" json:"visitorId"`
	// 访客标识, 前端存在 localStorage 里
	VisitorKey string `gorm:"column:visitor_key;size:64;not null;uniqueIndex;comment:访客标识" json:"visitorKey"`
	// 首次进入的渠道标识
	ChannelCode string `gorm:"column:channel_code;size:64;comment:首次进入的渠道标识" json:"channelCode"`
	// 关联的前台用户id, 0 表示匿名访客
	UserID int64 `gorm:"column:user_id;not null;default:0;comment:关联的前台用户id, 0表示匿名访客" json:"userId"`
	// 昵称
	Nickname string `gorm:"column:nickname;size:64;comment:昵称" json:"nickname"`
	// 头像
	Avatar string `gorm:"column:avatar;size:255;comment:头像" json:"avatar"`
	// 手机号
	Phone string `gorm:"column:phone;size:32;comment:手机号" json:"phone"`
	// 邮箱，选填
	Email string `gorm:"column:email;size:128;comment:邮箱，选填" json:"email"`
	// 邮箱是否无效, 0否, 1是, 收到退信后置为 1 并停止自动回复
	MailInvalid int `gorm:"column:mail_invalid;not null;default:0;comment:邮箱是否无效" json:"mailInvalid"`
	// 访客ip
	IP string `gorm:"column:ip;size:64;comment:访客ip" json:"ip"`
	// 来源页面地址
	SourceURL string `gorm:"column:source_url;size:512;comment:来源页面地址" json:"sourceUrl"`
	// 最后活跃时间
	LastActiveTime Time `gorm:"column:last_active_time;comment:最后活跃时间" json:"lastActiveTime"`
	// 浏览轨迹, json 数组, 只存页面标题与地址, 最多 10 条
	BrowseTrail string `gorm:"column:browse_trail;type:text;comment:浏览轨迹" json:"browseTrail"`
	// 是否删除, 0否, 1是
	Deleted int `gorm:"column:deleted;not null;default:0;comment:是否删除" json:"deleted"`
	// 创建时间
	CreateTime Time `gorm:"column:create_time;comment:创建时间" json:"createTime"`
	// 更新时间
	UpdateTime Time `gorm:"column:update_time;comment:更新时间" json:"updateTime"`
}

// TableName 指定表名
func (KefuVisitor) TableName() string {
	return "kefu_visitor"
}

// KefuVisitorParam 是访客的查询参数
type KefuVisitorParam struct {
	common.PageParam
	// 访客标识
	VisitorKey string `form:"visitorKey"`
	// 渠道标识
	ChannelCode string `form:"channelCode"`
	// 前台用户id
	UserID *int64 `form:"userId"`
	// 昵称
	Nickname string `form:"nickname"`
	// 手机号
	Phone string `form:"phone"`
}

// KefuBrowseTrackForm 是访客上报浏览轨迹的提交数据
type KefuBrowseTrackForm struct {
	// 访客标识
	VisitorKey string `json:"visitorKey"`
	// 页面标题
	Title string `json:"title"`
	// 页面地址
	URL string `json:"url"`
}

// KefuVisitorForm 是访客资料的编辑提交数据
type KefuVisitorForm struct {
	// 访客id
	VisitorID int64 `json:"visitorId"`
	// 关联的前台用户id, 0 表示匿名访客
	UserID int64 `json:"userId"`
	// 昵称
	Nickname string `json:"nickname"`
	// 头像
	Avatar string `json:"avatar"`
	// 手机号
	Phone string `json:"phone"`
	// 邮箱
	Email string `json:"email"`
}

// KefuBootstrapForm 是访客接入的提交数据，
// visitorKey 为空表示首次进入，由服务端生成后返回
type KefuBootstrapForm struct {
	// 渠道标识
	ChannelCode string `json:"channelCode"`
	// 访客标识, 前端存在 localStorage 里
	VisitorKey string `json:"visitorKey"`
	// 昵称, 选填
	Nickname string `json:"nickname"`
	// 邮箱, 选填
	Email string `json:"email"`
	// 来源页面地址
	SourceURL string `json:"sourceUrl"`
}

// KefuBootstrapResult 是访客接入的返回结果，
// 前端用 uid/token/wsUrl 连接 IM，用 sessionId 收发消息
type KefuBootstrapResult struct {
	// 访客的 IM 用户标识
	UID string `json:"uid"`
	// 访客的 IM token
	Token string `json:"token"`
	// IM gateway 地址
	WsURL string `json:"wsUrl"`
	// AI 服务号的 UID
	AiUID string `json:"aiUid"`
	// 访客id
	VisitorID int64 `json:"visitorId"`
	// 访客标识, 首次接入时由服务端生成
	VisitorKey string `json:"visitorKey"`
	// 访客昵称
	Nickname string `json:"nickname"`
	// 访客邮箱, 未填时为空
	Email string `json:"email"`
	// 当前会话id
	SessionID int64 `json:"sessionId"`
	// 会话状态
	SessionStatus int `json:"sessionStatus"`
	// 接待客服的名称, 未分配时为空
	AgentName string `json:"agentName"`
	// AI 自动回复开关
	AiMode int `json:"aiMode"`
	// 欢迎语
	WelcomeText string `json:"welcomeText"`
}
