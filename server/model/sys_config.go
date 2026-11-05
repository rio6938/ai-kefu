package model

// 配置项的值类型
const (
	// ConfigTypeString 是字符串
	ConfigTypeString = "string"
	// ConfigTypeInt 是整数
	ConfigTypeInt = "int"
	// ConfigTypeFloat 是浮点数
	ConfigTypeFloat = "float"
	// ConfigTypeBool 是布尔值，取值为 true / false
	ConfigTypeBool = "bool"
	// ConfigTypeJSON 是 json 文本，用于模板映射这类结构化配置
	ConfigTypeJSON = "json"
)

// Config 是数据库配置项，对应 sys_config 表；配置项由各业务模块在迁移时写入，
// 后台只能改值，新增配置项需要对应模块补种子数据
type Config struct {
	// 配置id
	ConfigID int64 `gorm:"column:config_id;primaryKey;autoIncrement;comment:配置id" json:"configId"`
	// 配置键
	ConfigKey string `gorm:"column:config_key;size:128;not null;uniqueIndex;comment:配置键" json:"configKey"`
	// 配置值，密钥类配置存的是密文
	ConfigValue string `gorm:"column:config_value;type:text;comment:配置值" json:"configValue"`
	// 值类型, string/int/float/bool/json
	ValueType string `gorm:"column:value_type;size:16;not null;comment:值类型" json:"valueType"`
	// 配置名称
	ConfigName string `gorm:"column:config_name;size:128;not null;comment:配置名称" json:"configName"`
	// 分组标识
	GroupCode string `gorm:"column:group_code;size:64;not null;comment:分组标识" json:"groupCode"`
	// 分组名称
	GroupName string `gorm:"column:group_name;size:64;not null;comment:分组名称" json:"groupName"`
	// 是否密钥类配置, 0否, 1是
	IsSecret int `gorm:"column:is_secret;not null;default:0;comment:是否密钥类配置" json:"isSecret"`
	// 是否允许下发到前台, 0否, 1是
	IsPublic int `gorm:"column:is_public;not null;default:0;comment:是否允许下发到前台" json:"isPublic"`
	// 排序号
	SortNumber int `gorm:"column:sort_number;not null;default:0;comment:排序号" json:"sortNumber"`
	// 备注
	Comments string `gorm:"column:comments;size:255;comment:备注" json:"comments"`
	// 最后修改人id
	UpdateUserID int64 `gorm:"column:update_user_id;comment:最后修改人id" json:"updateUserId"`
	// 是否删除, 0否, 1是
	Deleted int `gorm:"column:deleted;not null;default:0;comment:是否删除" json:"deleted"`
	// 创建时间
	CreateTime Time `gorm:"column:create_time;comment:创建时间" json:"createTime"`
	// 更新时间
	UpdateTime Time `gorm:"column:update_time;comment:更新时间" json:"updateTime"`
	// 密钥类配置的掩码值, 不由数据库存储
	SecretMask string `gorm:"-" json:"secretMask,omitempty"`
	// 最后修改人账号, 查询时回填
	UpdateUsername string `gorm:"-" json:"updateUsername,omitempty"`
}

// TableName 指定表名
func (Config) TableName() string {
	return "sys_config"
}

// ConfigParam 是配置项的查询参数
type ConfigParam struct {
	// 分组标识
	GroupCode string `form:"groupCode"`
}

// ConfigItemForm 是按分组保存配置时的单个配置项
type ConfigItemForm struct {
	// 配置键
	ConfigKey string `json:"configKey"`
	// 配置值, 密钥类配置提交空值时表示不修改
	ConfigValue string `json:"configValue"`
}

// ConfigGroupForm 是按分组保存配置的提交数据
type ConfigGroupForm struct {
	// 分组标识
	GroupCode string `json:"groupCode"`
	// 分组下的配置项
	Items []ConfigItemForm `json:"items"`
}

// ConfigPublicParam 是前台读取公开配置的查询参数
type ConfigPublicParam struct {
	// 配置键, 多个用逗号分隔, 为空时返回全部公开配置
	Keys string `form:"keys"`
}
