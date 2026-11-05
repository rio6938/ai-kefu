package service

import (
	"strings"
	"testing"

	"admin/server/model"
)

// TestGenerateVectorID 校验向量点 id 是标准 uuid, 向量库只接受这种格式
func TestGenerateVectorID(t *testing.T) {
	first := generateVectorID()
	second := generateVectorID()
	if first == second {
		t.Fatalf("vector id should be unique, got %q twice", first)
	}

	parts := strings.Split(first, "-")
	if len(parts) != 5 {
		t.Fatalf("unexpected uuid format %q", first)
	}
	for _, part := range parts {
		if part == "" {
			t.Fatalf("unexpected uuid format %q", first)
		}
	}
	if len(parts[0]) != 8 || len(parts[1]) != 4 || len(parts[2]) != 4 ||
		len(parts[3]) != 4 || len(parts[4]) != 12 {
		t.Fatalf("unexpected uuid format %q", first)
	}
	if len(first) != 36 {
		t.Fatalf("unexpected uuid length %d", len(first))
	}
}

// TestKnowledgeText 校验召回命中拼成提示词知识内容, 空内容被跳过
func TestKnowledgeText(t *testing.T) {
	if got := knowledgeText(nil); got != "" {
		t.Fatalf("expect empty text, got %q", got)
	}

	hits := []model.KefuKnowledgeHit{
		{DocID: 1, Title: "退换货政策", Content: "七天无理由退换"},
		{DocID: 2, Title: "空文档", Content: "   "},
		{DocID: 3, Title: "发票说明", Content: "下单时可申请电子发票"},
	}
	want := "【退换货政策】七天无理由退换\n\n【发票说明】下单时可申请电子发票"
	if got := knowledgeText(hits); got != want {
		t.Fatalf("expect %q, got %q", want, got)
	}
}

// TestTruncateText 校验失败原因按上限截断
func TestTruncateText(t *testing.T) {
	if got := truncateText(" 短原因 ", 10); got != "短原因" {
		t.Fatalf("unexpected text %q", got)
	}
	if got := truncateText(strings.Repeat("错", 10), 4); got != "错错错错..." {
		t.Fatalf("unexpected text %q", got)
	}
	if got := truncateText("原因", 0); got != "原因" {
		t.Fatalf("limit 0 should keep text, got %q", got)
	}
}

// TestRagParams 校验召回参数未提交时用默认值, 提交了用提交值
func TestRagParams(t *testing.T) {
	s := NewKefuKnowledgeService()
	if got := s.topK(0); got != defaultRagTopK {
		t.Fatalf("expect default topK, got %d", got)
	}
	if got := s.topK(3); got != 3 {
		t.Fatalf("expect topK 3, got %d", got)
	}
	if got := s.threshold(0); got != defaultRagThreshold {
		t.Fatalf("expect default threshold, got %v", got)
	}
	if got := s.threshold(0.6); got != 0.6 {
		t.Fatalf("expect threshold 0.6, got %v", got)
	}
}

// TestRagHitsJSON 校验命中片段的序列化, 没有命中时存空数组
func TestRagHitsJSON(t *testing.T) {
	if got := ragHitsJSON(nil); got != "[]" {
		t.Fatalf("expect empty array, got %q", got)
	}

	hits := []model.KefuKnowledgeHit{{DocID: 1, Title: "退换货政策", Content: "七天无理由退换", Score: 0.8}}
	got := ragHitsJSON(hits)
	want := `[{"docId":1,"title":"退换货政策","chunkSeq":0,"content":"七天无理由退换","score":0.8}]`
	if got != want {
		t.Fatalf("expect %q, got %q", want, got)
	}
}
