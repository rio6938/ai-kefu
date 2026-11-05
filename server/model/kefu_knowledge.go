package model

import "admin/server/internal/common"

// 知识库文档状态
const (
	// KefuKnowledgeStatusPending 是待处理
	KefuKnowledgeStatusPending = 0
	// KefuKnowledgeStatusParsing 是解析中
	KefuKnowledgeStatusParsing = 1
	// KefuKnowledgeStatusEmbedding 是向量化中
	KefuKnowledgeStatusEmbedding = 2
	// KefuKnowledgeStatusReady 是就绪, 可被检索
	KefuKnowledgeStatusReady = 3
	// KefuKnowledgeStatusFailed 是失败, 可重试
	KefuKnowledgeStatusFailed = 4
)

// 知识库文档来源
const (
	// KefuKnowledgeSourceUpload 是后台上传
	KefuKnowledgeSourceUpload = "upload"
)

// KefuKnowledge 是知识库文档，对应 kefu_knowledge 表；
// 文档按文档库分类归档, 检索时按关联链绑定的分类集合过滤
type KefuKnowledge struct {
	// 文档id
	DocID int64 `gorm:"column:doc_id;primaryKey;autoIncrement;comment:文档id" json:"docId"`
	// 文档库分类id, 0 表示未分类, 不参与检索
	CategoryID int64 `gorm:"column:category_id;not null;default:0;index;comment:文档库分类id, 0 表示未分类, 不参与检索" json:"categoryId"`
	// 文档标题
	Title string `gorm:"column:title;size:255;not null;comment:文档标题" json:"title"`
	// 关联的文件id, 直接录入文本时为 0
	FileID int64 `gorm:"column:file_id;not null;default:0;comment:关联的文件id" json:"fileId"`
	// 文件名
	FileName string `gorm:"column:file_name;size:255;comment:文件名" json:"fileName"`
	// 文件扩展名
	FileExt string `gorm:"column:file_ext;size:16;comment:文件扩展名" json:"fileExt"`
	// 文件大小, 字节
	FileSize int64 `gorm:"column:file_size;not null;default:0;comment:文件大小, 字节" json:"fileSize"`
	// 分块数量
	ChunkCount int `gorm:"column:chunk_count;not null;default:0;comment:分块数量" json:"chunkCount"`
	// 状态, 0待处理, 1解析中, 2向量化中, 3就绪, 4失败
	Status int `gorm:"column:status;not null;default:0;comment:状态, 0待处理, 1解析中, 2向量化中, 3就绪, 4失败" json:"status"`
	// 失败原因
	Error string `gorm:"column:error;type:text;comment:失败原因" json:"error"`
	// 来源, upload 后台上传
	Source string `gorm:"column:source;size:16;not null;default:upload;comment:来源" json:"source"`
	// 创建人id
	CreateUserID int64 `gorm:"column:create_user_id;not null;default:0;comment:创建人id" json:"createUserId"`
	// 是否删除, 0否, 1是
	Deleted int `gorm:"column:deleted;not null;default:0;comment:是否删除" json:"deleted"`
	// 创建时间
	CreateTime Time `gorm:"column:create_time;comment:创建时间" json:"createTime"`
	// 更新时间
	UpdateTime Time `gorm:"column:update_time;comment:更新时间" json:"updateTime"`
	// 文档库分类名称, 查询时回填
	CategoryName string `gorm:"-" json:"categoryName"`
}

// TableName 指定表名
func (KefuKnowledge) TableName() string {
	return "kefu_knowledge"
}

// KefuKnowledgeParam 是知识库文档的查询参数
type KefuKnowledgeParam struct {
	common.PageParam
	// 文档库分类id
	CategoryID *int64 `form:"categoryId"`
	// 文档标题
	Title string `form:"title"`
	// 状态
	Status *int `form:"status"`
	// 来源
	Source string `form:"source"`
}

// KefuKnowledgeForm 是新增知识库文档的提交数据
type KefuKnowledgeForm struct {
	// 文档id, 修改时使用
	DocID int64 `json:"docId"`
	// 文档库分类id
	CategoryID int64 `json:"categoryId"`
	// 文档标题, 为空时用文件名
	Title string `json:"title"`
	// 关联的文件id
	FileID int64 `json:"fileId"`
	// 文件名
	FileName string `json:"fileName"`
	// 文件扩展名
	FileExt string `json:"fileExt"`
	// 文件大小, 字节
	FileSize int64 `json:"fileSize"`
	// 文档内容, 直接录入文本时使用
	Content string `json:"content"`
	// 来源, 空值为 upload
	Source string `json:"source"`
}

// KefuKnowledgeHit 是一条召回命中，召回测试与 AI 编排共用
type KefuKnowledgeHit struct {
	// 文档id
	DocID int64 `json:"docId"`
	// 文档标题
	Title string `json:"title"`
	// 分块序号
	ChunkSeq int `json:"chunkSeq"`
	// 分块内容
	Content string `json:"content"`
	// 相似度得分
	Score float32 `json:"score"`
}

// KefuKnowledgeSearchForm 是知识库召回测试的提交数据
type KefuKnowledgeSearchForm struct {
	// 文档库分类id, 只在这些分类内召回
	CategoryIDs []int64 `json:"categoryIds"`
	// 检索内容
	Query string `json:"query"`
	// 召回条数, 不传时取配置中心的值
	TopK int `json:"topK"`
	// 相似度阈值, 不传时取配置中心的值
	Threshold float64 `json:"threshold"`
}
