package rag

import (
	"context"
	"testing"
)

// TestNewKnowledgePayload 校验知识库向量的 payload 字段, doc_id 与 category_id 用字符串便于关键词过滤
func TestNewKnowledgePayload(t *testing.T) {
	payload := NewKnowledgePayload(12, 7, "退换货政策", 3)
	if got := payload[PayloadDocID]; got != "12" {
		t.Fatalf("expect doc_id 12, got %v", got)
	}
	if got := payload[PayloadCategoryID]; got != "7" {
		t.Fatalf("expect category_id 7, got %v", got)
	}
	if got := payload[PayloadTitle]; got != "退换货政策" {
		t.Fatalf("unexpected title %v", got)
	}
	if got := payload[PayloadChunkSeq]; got != 3 {
		t.Fatalf("expect chunk_seq 3, got %v", got)
	}
}

// TestPayloadHelpers 校验 payload 字段在字符串与数值两种写法下都能读出
func TestPayloadHelpers(t *testing.T) {
	payload := map[string]any{
		PayloadDocID:      "8",
		PayloadChunkSeq:   int64(2),
		PayloadTitle:      "发票说明",
		PayloadCategoryID: float64(5),
	}
	if got := payloadInt64(payload, PayloadDocID); got != 8 {
		t.Fatalf("expect doc id 8, got %d", got)
	}
	if got := payloadInt64(payload, PayloadChunkSeq); got != 2 {
		t.Fatalf("expect chunk seq 2, got %d", got)
	}
	if got := payloadString(payload, PayloadTitle); got != "发票说明" {
		t.Fatalf("unexpected title %q", got)
	}
	if got := payloadString(payload, PayloadCategoryID); got != "5" {
		t.Fatalf("expect category id 5, got %q", got)
	}
	if got := payloadString(payload, "unknown"); got != "" {
		t.Fatalf("expect empty for unknown key, got %q", got)
	}
	if got := payloadInt64(payload, "unknown"); got != 0 {
		t.Fatalf("expect 0 for unknown key, got %d", got)
	}
}

// TestCategoryIDs 校验分类id去重去 0 后转成十进制字符串
func TestCategoryIDs(t *testing.T) {
	if got := categoryIDs(nil); len(got) != 0 {
		t.Fatalf("expect empty ids, got %v", got)
	}
	got := categoryIDs([]int64{3, 0, 3, 5, -1})
	if len(got) != 2 || got[0] != "3" || got[1] != "5" {
		t.Fatalf("unexpected ids %v", got)
	}
}

// TestRetrieveValidation 校验检索参数的必填判断与向量库未初始化的提示
func TestRetrieveValidation(t *testing.T) {
	embed := func(_ context.Context, texts []string) ([][]float32, error) {
		return [][]float32{{1, 2, 3}}, nil
	}

	if _, err := Retrieve(context.Background(), nil, Query{CategoryIDs: []int64{1}, Text: "发货"}); err == nil {
		t.Fatal("expect error when embed is nil")
	}

	if _, err := Retrieve(context.Background(), embed, Query{Text: "发货"}); err == nil {
		t.Fatal("expect error when category ids is empty")
	}

	// 测试进程里没有初始化向量库, 直接返回未初始化
	_, err := Retrieve(context.Background(), embed, Query{CategoryIDs: []int64{1}, Text: "发货"})
	if err != nil && err.Error() != "向量库未初始化" {
		t.Fatalf("unexpected error %v", err)
	}
}
