package model

import "admin/server/internal/common"

// KefuKnowledgeChunk 是知识库文档的分块，对应 kefu_knowledge_chunk 表；
// 向量库只存向量与 payload, 分块内容落库便于重建与审计
type KefuKnowledgeChunk struct {
	// 分块id
	ChunkID int64 `gorm:"column:chunk_id;primaryKey;autoIncrement;comment:分块id" json:"chunkId"`
	// 文档id
	DocID int64 `gorm:"column:doc_id;not null;index;comment:文档id" json:"docId"`
	// 分块序号, 从 1 开始
	Seq int `gorm:"column:seq;not null;default:0;comment:分块序号" json:"seq"`
	// 分块内容
	Content string `gorm:"column:content;type:text;comment:分块内容" json:"content"`
	// 分块内容的 token 数
	Tokens int `gorm:"column:tokens;not null;default:0;comment:分块内容的token数" json:"tokens"`
	// 向量库中的 point id
	VectorID string `gorm:"column:vector_id;size:64;comment:向量库中的point id" json:"vectorId"`
	// 是否删除, 0否, 1是
	Deleted int `gorm:"column:deleted;not null;default:0;comment:是否删除" json:"deleted"`
	// 创建时间
	CreateTime Time `gorm:"column:create_time;comment:创建时间" json:"createTime"`
	// 更新时间
	UpdateTime Time `gorm:"column:update_time;comment:更新时间" json:"updateTime"`
}

// TableName 指定表名
func (KefuKnowledgeChunk) TableName() string {
	return "kefu_knowledge_chunk"
}

// KefuKnowledgeChunkParam 是分块的查询参数
type KefuKnowledgeChunkParam struct {
	common.PageParam
	// 文档id
	DocID *int64 `form:"docId"`
}

// KefuKnowledgeChunkForm 是分块的编辑提交数据
type KefuKnowledgeChunkForm struct {
	// 分块id
	ChunkID int64 `json:"chunkId"`
	// 文档id
	DocID int64 `json:"docId"`
	// 分块序号
	Seq int `json:"seq"`
	// 分块内容
	Content string `json:"content"`
}
