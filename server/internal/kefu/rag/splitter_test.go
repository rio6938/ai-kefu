package rag

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fumiama/go-docx"
)

// TestNormalizeExt 校验扩展名归一化: 去掉前缀点、转小写并兼容常见别名
func TestNormalizeExt(t *testing.T) {
	cases := map[string]string{
		".PDF":     ExtPDF,
		"htm":      ExtHTML,
		"Markdown": ExtMarkdown,
		" docx ":   ExtDocx,
		"unknown":  "unknown",
		"":         "",
	}
	for ext, want := range cases {
		if got := NormalizeExt(ext); got != want {
			t.Fatalf("ext %q expect %q, got %q", ext, want, got)
		}
	}
}

// TestSupported 校验支持与不支持的扩展名
func TestSupported(t *testing.T) {
	for _, ext := range []string{"pdf", "docx", "md", "txt", "html", "csv"} {
		if !Supported(ext) {
			t.Fatalf("ext %q should be supported", ext)
		}
	}
	if Supported("doc") || Supported("xlsx") {
		t.Fatal("unsupported ext should be rejected")
	}
}

// TestLoadText 校验纯文本文档能解析出内容
func TestLoadText(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kb.txt")
	if err := os.WriteFile(path, []byte("售前咨询请拨打 400-000-0000"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	text, err := Load(context.Background(), path, "")
	if err != nil {
		t.Fatalf("load text: %v", err)
	}
	if !strings.Contains(text, "400-000-0000") {
		t.Fatalf("unexpected text %q", text)
	}
}

// TestLoadDocx 校验 docx 能解析出段落文本
func TestLoadDocx(t *testing.T) {
	doc := docx.New()
	doc.AddParagraph().AddText("退换货政策")
	doc.AddParagraph().AddText("七天无理由退换，需保持包装完整。")

	path := filepath.Join(t.TempDir(), "kb.docx")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	if _, err := doc.WriteTo(file); err != nil {
		_ = file.Close()
		t.Fatalf("write docx: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close file: %v", err)
	}

	text, err := Load(context.Background(), path, "docx")
	if err != nil {
		t.Fatalf("load docx: %v", err)
	}
	if !strings.Contains(text, "退换货政策") ||
		!strings.Contains(text, "七天无理由退换") {
		t.Fatalf("unexpected docx text %q", text)
	}
}

// TestLoadHTMLAndCSV 校验网页与表格文档能解析出文本
func TestLoadHTMLAndCSV(t *testing.T) {
	dir := t.TempDir()

	htmlPath := filepath.Join(dir, "kb.html")
	if err := os.WriteFile(htmlPath,
		[]byte("<html><body><p>售后请提供订单号</p></body></html>"), 0o600); err != nil {
		t.Fatalf("write html: %v", err)
	}
	htmlText, err := Load(context.Background(), htmlPath, "html")
	if err != nil {
		t.Fatalf("load html: %v", err)
	}
	if !strings.Contains(htmlText, "售后请提供订单号") {
		t.Fatalf("unexpected html text %q", htmlText)
	}

	csvPath := filepath.Join(dir, "kb.csv")
	if err := os.WriteFile(csvPath,
		[]byte("问题,答案\n发货时间,付款后 48 小时内发货\n"), 0o600); err != nil {
		t.Fatalf("write csv: %v", err)
	}
	csvText, err := Load(context.Background(), csvPath, "csv")
	if err != nil {
		t.Fatalf("load csv: %v", err)
	}
	if !strings.Contains(csvText, "付款后 48 小时内发货") {
		t.Fatalf("unexpected csv text %q", csvText)
	}
}

// TestLoadUnsupportedExt 校验不支持的格式直接报错
func TestLoadUnsupportedExt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kb.xlsx")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if _, err := Load(context.Background(), path, "xlsx"); err == nil {
		t.Fatal("expect error for unsupported ext")
	}
}

// TestSplitMarkdown 校验 md 按标题切分, 且没有空白分块
func TestSplitMarkdown(t *testing.T) {
	text := "# 退换货政策\n\n七天无理由退换，需保持包装完整。\n\n# 发票说明\n\n下单时可申请电子发票。"
	chunks, err := Split(text, "md")
	if err != nil {
		t.Fatalf("split markdown: %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("expect at least 2 chunks, got %d", len(chunks))
	}
	for i, chunk := range chunks {
		if chunk.Seq != i+1 {
			t.Fatalf("chunk %d seq expect %d, got %d", i, i+1, chunk.Seq)
		}
		if strings.TrimSpace(chunk.Content) == "" {
			t.Fatalf("chunk %d is blank", i)
		}
		if chunk.Tokens <= 0 {
			t.Fatalf("chunk %d tokens should be positive", i)
		}
	}
}

// TestSplitText 校验普通文本切分并清理空白分块
func TestSplitText(t *testing.T) {
	chunks, err := Split("\r\n\n  第一段  \r\n\r\n\r\n\r\n第二段\r\n", "txt")
	if err != nil {
		t.Fatalf("split text: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expect 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Content != "第一段\n\n\n\n第二段" {
		t.Fatalf("unexpected content %q", chunks[0].Content)
	}
}

// TestSplitEmpty 校验空文档返回空结果而不是错误
func TestSplitEmpty(t *testing.T) {
	chunks, err := Split("   \r\n\t ", "txt")
	if err != nil {
		t.Fatalf("split empty text: %v", err)
	}
	if len(chunks) != 0 {
		t.Fatalf("expect no chunk, got %d", len(chunks))
	}
}
