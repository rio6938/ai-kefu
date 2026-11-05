package rag

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"admin/server/internal/kefu/vector"
)

// KnowledgeCollection 是知识库在向量库中的集合基础名，
// 实际集合名由 vector 包加上配置的前缀，如 kefu_knowledge
const KnowledgeCollection = "knowledge"

// 知识库向量 payload 的字段名；
// 检索按 category_id 过滤，其余字段用于回显，分块内容仍以数据库为准
const (
	// PayloadDocID 是文档id
	PayloadDocID = "doc_id"
	// PayloadCategoryID 是文档库分类id
	PayloadCategoryID = "category_id"
	// PayloadChunkSeq 是分块序号
	PayloadChunkSeq = "chunk_seq"
	// PayloadTitle 是文档标题
	PayloadTitle = "title"
)

// EmbedFunc 把文本转成向量，由 service 层用模型客户端实现
type EmbedFunc func(ctx context.Context, texts []string) ([][]float32, error)

// Query 是一次知识库检索的条件
type Query struct {
	// 文档库分类id, 只在这些分类内召回
	CategoryIDs []int64
	// 检索文本，通常是客户的提问
	Text string
	// 召回条数
	TopK int
	// 相似度阈值，大于 0 时生效
	Threshold float64
}

// Hit 是一条召回命中，分块内容由调用方按 VectorID 回查数据库
type Hit struct {
	// 向量点id
	VectorID string
	// 文档id
	DocID int64
	// 分块序号
	ChunkSeq int
	// 文档标题
	Title string
	// 相似度得分
	Score float32
}

// NewKnowledgePayload 组装知识库向量点的 payload；
// doc_id 与 category_id 用字符串保存，与向量库的关键词过滤保持一致
func NewKnowledgePayload(docID int64, categoryID int64, title string, seq int) map[string]any {
	return map[string]any{
		PayloadDocID:      strconv.FormatInt(docID, 10),
		PayloadCategoryID: strconv.FormatInt(categoryID, 10),
		PayloadChunkSeq:   seq,
		PayloadTitle:      title,
	}
}

// Retrieve 把检索文本向量化后按文档库分类从知识库集合召回分块
func Retrieve(ctx context.Context, embed EmbedFunc, query Query) ([]Hit, error) {
	if vector.Default == nil {
		return nil, vector.ErrNotInitialized
	}
	if embed == nil {
		return nil, errors.New("知识库检索缺少向量化能力")
	}

	text := strings.TrimSpace(query.Text)
	if text == "" {
		return nil, errors.New("检索内容不能为空")
	}
	ids := categoryIDs(query.CategoryIDs)
	if len(ids) == 0 {
		return nil, errors.New("检索必须指定文档库分类")
	}

	vectors, err := embed(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(vectors) == 0 || len(vectors[0]) == 0 {
		return nil, errors.New("检索文本向量化结果为空")
	}

	results, err := vector.Default.SearchWithFilter(ctx, KnowledgeCollection, vectors[0],
		&vector.Filter{Any: map[string][]string{PayloadCategoryID: ids}},
		query.TopK, query.Threshold)
	if err != nil {
		return nil, err
	}

	hits := make([]Hit, 0, len(results))
	for _, result := range results {
		hits = append(hits, Hit{
			VectorID: result.ID,
			DocID:    payloadInt64(result.Payload, PayloadDocID),
			ChunkSeq: int(payloadInt64(result.Payload, PayloadChunkSeq)),
			Title:    payloadString(result.Payload, PayloadTitle),
			Score:    result.Score,
		})
	}
	return hits, nil
}

// categoryIDs 把分类id去重后转成十进制字符串，与向量库的关键词过滤对齐
func categoryIDs(values []int64) []string {
	list := make([]string, 0, len(values))
	seen := make(map[int64]bool, len(values))
	for _, value := range values {
		if value <= 0 || seen[value] {
			continue
		}
		seen[value] = true
		list = append(list, strconv.FormatInt(value, 10))
	}
	return list
}

// payloadString 读取 payload 里的字符串字段，数值写法也兼容
func payloadString(payload map[string]any, key string) string {
	value, ok := payload[key]
	if !ok {
		return ""
	}
	switch item := value.(type) {
	case string:
		return item
	case int64:
		return strconv.FormatInt(item, 10)
	case int:
		return strconv.Itoa(item)
	case float64:
		return strconv.FormatInt(int64(item), 10)
	default:
		return ""
	}
}

// payloadInt64 读取 payload 里的整数字段，字符串写法也兼容
func payloadInt64(payload map[string]any, key string) int64 {
	value := payloadString(payload, key)
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0
	}
	return parsed
}
