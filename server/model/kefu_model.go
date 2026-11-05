package model

import "admin/server/internal/common"

// 模型服务商标识
const (
	// KefuModelProviderOpenAICompatible 是 OpenAI 兼容接口
	KefuModelProviderOpenAICompatible = "openai_compatible"
)

// 模型配置状态
const (
	// KefuModelStatusNormal 是启用
	KefuModelStatusNormal = 0
	// KefuModelStatusDisabled 是停用
	KefuModelStatusDisabled = 1
)

// 是否为默认模型
const (
	// KefuModelNotDefault 是非默认模型
	KefuModelNotDefault = 0
	// KefuModelIsDefault 是默认模型
	KefuModelIsDefault = 1
)

// 是否用作翻译模型
const (
	// KefuModelNotTranslate 是不用作翻译模型
	KefuModelNotTranslate = 0
	// KefuModelIsTranslate 是用作翻译模型
	KefuModelIsTranslate = 1
)

// KefuModel 是模型配置，对应 kefu_model 表；
// api_key 存 pkg/encrypt 的密文, 接口输出时只给掩码
type KefuModel struct {
	// 模型id
	ModelID int64 `gorm:"column:model_id;primaryKey;autoIncrement;comment:模型id" json:"modelId"`
	// 模型名称, 展示用
	Name string `gorm:"column:name;size:128;not null;comment:模型名称" json:"name"`
	// 服务商, 目前只有 openai_compatible
	Provider string `gorm:"column:provider;size:32;not null;comment:服务商" json:"provider"`
	// 接口地址, 如 https://api.deepseek.com/v1
	BaseURL string `gorm:"column:base_url;size:255;not null;comment:接口地址" json:"baseUrl"`
	// 接口密钥密文
	ApiKey string `gorm:"column:api_key;type:text;comment:接口密钥密文" json:"apiKey"`
	// 模型标识, 如 deepseek-chat
	Model string `gorm:"column:model;size:128;not null;comment:模型标识" json:"model"`
	// 采样温度
	Temperature float64 `gorm:"column:temperature;not null;default:0.7;comment:采样温度" json:"temperature"`
	// 核采样
	TopP float64 `gorm:"column:top_p;not null;default:1;comment:核采样" json:"topP"`
	// 最大回复 token 数
	MaxTokens int `gorm:"column:max_tokens;not null;default:2048;comment:最大回复token数" json:"maxTokens"`
	// embedding 模型标识, 为空表示不使用知识库向量检索
	EmbeddingModel string `gorm:"column:embedding_model;size:128;comment:embedding模型标识" json:"embeddingModel"`
	// embedding 接口地址, 为空时沿用 base_url
	EmbeddingBaseURL string `gorm:"column:embedding_base_url;size:255;comment:embedding接口地址" json:"embeddingBaseUrl"`
	// embedding 接口密钥密文, 为空时沿用 api_key
	EmbeddingApiKey string `gorm:"column:embedding_api_key;type:text;comment:embedding接口密钥密文" json:"embeddingApiKey"`
	// 是否为默认模型, 0否, 1是
	IsDefault int `gorm:"column:is_default;not null;default:0;comment:是否为默认模型" json:"isDefault"`
	// 是否用作翻译模型, 0否, 1是
	TranslateEnabled int `gorm:"column:translate_enabled;not null;default:0;comment:是否用作翻译模型" json:"translateEnabled"`
	// 状态, 0启用, 1停用
	Status int `gorm:"column:status;not null;default:0;comment:状态, 0启用, 1停用" json:"status"`
	// 是否删除, 0否, 1是
	Deleted int `gorm:"column:deleted;not null;default:0;comment:是否删除" json:"deleted"`
	// 创建时间
	CreateTime Time `gorm:"column:create_time;comment:创建时间" json:"createTime"`
	// 更新时间
	UpdateTime Time `gorm:"column:update_time;comment:更新时间" json:"updateTime"`
	// 密钥掩码, 不由数据库存储
	ApiKeyMask string `gorm:"-" json:"apiKeyMask,omitempty"`
	// embedding 密钥掩码, 不由数据库存储
	EmbeddingApiKeyMask string `gorm:"-" json:"embeddingApiKeyMask,omitempty"`
}

// TableName 指定表名
func (KefuModel) TableName() string {
	return "kefu_model"
}

// KefuModelParam 是模型配置的查询参数
type KefuModelParam struct {
	common.PageParam
	// 模型名称
	Name string `form:"name"`
	// 服务商
	Provider string `form:"provider"`
	// 状态
	Status *int `form:"status"`
}

// KefuModelForm 是模型配置的编辑提交数据
type KefuModelForm struct {
	// 模型id
	ModelID int64 `json:"modelId"`
	// 模型名称
	Name string `json:"name"`
	// 服务商
	Provider string `json:"provider"`
	// 接口地址
	BaseURL string `json:"baseUrl"`
	// 接口密钥, 提交空值表示不修改
	ApiKey string `json:"apiKey"`
	// 模型标识
	Model string `json:"model"`
	// 采样温度
	Temperature float64 `json:"temperature"`
	// 核采样
	TopP float64 `json:"topP"`
	// 最大回复 token 数
	MaxTokens int `json:"maxTokens"`
	// embedding 模型标识, 留空表示不使用知识库向量检索
	EmbeddingModel string `json:"embeddingModel"`
	// embedding 接口地址, 留空时沿用接口地址
	EmbeddingBaseURL string `json:"embeddingBaseUrl"`
	// embedding 接口密钥, 提交空值表示不修改且沿用接口密钥
	EmbeddingApiKey string `json:"embeddingApiKey"`
	// 是否为默认模型, 0否, 1是
	IsDefault int `json:"isDefault"`
	// 是否用作翻译模型, 0否, 1是
	TranslateEnabled int `json:"translateEnabled"`
}

// KefuTranslateForm 是翻译的提交数据
type KefuTranslateForm struct {
	// 待翻译的文本
	Text string `json:"text"`
	// 源语言, 留空时由模型自动识别
	Source string `json:"source"`
	// 目标语言, 留空时取配置的默认目标语言
	Target string `json:"target"`
}

// KefuTranslateResult 是翻译返回的结构
type KefuTranslateResult struct {
	// 译文
	Text string `json:"text"`
	// 源语言
	Source string `json:"source"`
	// 目标语言
	Target string `json:"target"`
	// 使用的模型标识
	Model string `json:"model"`
}

// KefuModelStatusForm 是模型状态开关的提交数据
type KefuModelStatusForm struct {
	// 模型id
	ModelID int64 `json:"modelId"`
	// 状态, 0启用, 1停用
	Status int `json:"status"`
}
