package model

import "admin/server/internal/common"

// 对外 API 密钥状态
const (
	// KefuApiKeyStatusNormal 是启用
	KefuApiKeyStatusNormal = 0
	// KefuApiKeyStatusDisabled 是停用
	KefuApiKeyStatusDisabled = 1
)

// KefuApiKey 是对外 OpenAI 兼容接口的访问密钥，对应 kefu_api_key 表；
// 明文只在签发时返回一次, 库里只存前缀与哈希
type KefuApiKey struct {
	// 密钥id
	KeyID int64 `gorm:"column:key_id;primaryKey;autoIncrement;comment:密钥id" json:"keyId"`
	// 密钥名称, 便于区分调用方
	Name string `gorm:"column:name;size:128;not null;comment:密钥名称" json:"name"`
	// 密钥前缀, 用于在列表上认出是哪个密钥
	KeyPrefix string `gorm:"column:key_prefix;size:32;not null;index;comment:密钥前缀" json:"keyPrefix"`
	// 密钥的 sha256 哈希
	KeyHash string `gorm:"column:key_hash;size:64;not null;uniqueIndex;comment:密钥的sha256哈希" json:"keyHash"`
	// 状态, 0启用, 1停用
	Status int `gorm:"column:status;not null;default:0;comment:状态, 0启用, 1停用" json:"status"`
	// 过期时间, 零值表示永不过期
	ExpireTime Time `gorm:"column:expire_time;comment:过期时间" json:"expireTime"`
	// 最后使用时间
	LastUsedTime Time `gorm:"column:last_used_time;comment:最后使用时间" json:"lastUsedTime"`
	// 是否删除, 0否, 1是
	Deleted int `gorm:"column:deleted;not null;default:0;comment:是否删除" json:"deleted"`
	// 创建时间
	CreateTime Time `gorm:"column:create_time;comment:创建时间" json:"createTime"`
	// 更新时间
	UpdateTime Time `gorm:"column:update_time;comment:更新时间" json:"updateTime"`
}

// TableName 指定表名
func (KefuApiKey) TableName() string {
	return "kefu_api_key"
}

// KefuApiKeyParam 是 API 密钥的查询参数
type KefuApiKeyParam struct {
	common.PageParam
	// 密钥名称
	Name string `form:"name"`
	// 状态
	Status *int `form:"status"`
}

// KefuApiKeyForm 是签发 API 密钥的提交数据
type KefuApiKeyForm struct {
	// 密钥名称
	Name string `json:"name"`
	// 过期时间, 格式 2006-01-02 15:04:05, 为空表示永不过期
	ExpireTime string `json:"expireTime"`
}

// KefuApiKeyStatusForm 是 API 密钥状态开关的提交数据
type KefuApiKeyStatusForm struct {
	// 密钥id
	KeyID int64 `json:"keyId"`
	// 状态, 0启用, 1停用
	Status int `json:"status"`
}
