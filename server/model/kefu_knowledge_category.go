package model

import "admin/server/internal/common"

// 文档库分类状态
const (
	// KefuKnowledgeCategoryStatusNormal 是正常
	KefuKnowledgeCategoryStatusNormal = 0
	// KefuKnowledgeCategoryStatusDisabled 是停用
	KefuKnowledgeCategoryStatusDisabled = 1
)

// KefuKnowledgeCategory 是文档库分类，对应 kefu_knowledge_category 表；
// 分类是单层结构，一篇文档只属于一个分类，关联链按分类集合限定检索范围
type KefuKnowledgeCategory struct {
	// 分类id
	CategoryID int64 `gorm:"column:category_id;primaryKey;autoIncrement;comment:分类id" json:"categoryId"`
	// 分类名称
	Name string `gorm:"column:name;size:128;not null;comment:分类名称" json:"name"`
	// 排序号
	SortNumber int `gorm:"column:sort_number;not null;default:0;comment:排序号" json:"sortNumber"`
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
func (KefuKnowledgeCategory) TableName() string {
	return "kefu_knowledge_category"
}

// KefuKnowledgeCategoryParam 是文档库分类的查询参数
type KefuKnowledgeCategoryParam struct {
	common.PageParam
	// 分类名称
	Name string `form:"name"`
	// 状态, 0正常, 1停用
	Status *int `form:"status"`
}

// KefuKnowledgeCategoryForm 是文档库分类的编辑提交数据
type KefuKnowledgeCategoryForm struct {
	// 分类id
	CategoryID int64 `json:"categoryId"`
	// 分类名称
	Name string `json:"name"`
	// 排序号
	SortNumber int `json:"sortNumber"`
	// 状态, 0正常, 1停用
	Status int `json:"status"`
}

// KefuKnowledgeCategoryStatusForm 是文档库分类状态开关的提交数据
type KefuKnowledgeCategoryStatusForm struct {
	// 分类id
	CategoryID int64 `json:"categoryId"`
	// 状态, 0正常, 1停用
	Status int `json:"status"`
}
