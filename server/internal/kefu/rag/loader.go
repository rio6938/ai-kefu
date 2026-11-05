// Package rag 是知识库的文档解析、切分与向量召回；
// 向量读写交给 vector 包，入库编排与落库由 service 层完成
package rag

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fumiama/go-docx"
	"github.com/tmc/langchaingo/documentloaders"
)

// 支持的文档扩展名
const (
	// ExtPDF 是 pdf 文档
	ExtPDF = "pdf"
	// ExtDocx 是 docx 文档
	ExtDocx = "docx"
	// ExtMarkdown 是 md 文档
	ExtMarkdown = "md"
	// ExtText 是纯文本
	ExtText = "txt"
	// ExtHTML 是网页
	ExtHTML = "html"
	// ExtCSV 是表格文本
	ExtCSV = "csv"
)

// ErrUnsupportedExt 是不支持的文档格式
var ErrUnsupportedExt = errors.New("暂不支持的文档格式，仅支持 pdf、docx、md、txt、html、csv")

// NormalizeExt 归一化扩展名：去掉前缀点并转小写，兼容 htm 与 markdown 的写法
func NormalizeExt(ext string) string {
	value := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(ext), "."))
	switch value {
	case "htm":
		return ExtHTML
	case "markdown":
		return ExtMarkdown
	}
	return value
}

// Supported 判断扩展名是否支持入库
func Supported(ext string) bool {
	switch NormalizeExt(ext) {
	case ExtPDF, ExtDocx, ExtMarkdown, ExtText, ExtHTML, ExtCSV:
		return true
	default:
		return false
	}
}

// Load 解析文档并返回纯文本内容，ext 为空时按文件后缀判断
func Load(ctx context.Context, path, ext string) (string, error) {
	ext = NormalizeExt(ext)
	if ext == "" {
		ext = NormalizeExt(filepath.Ext(path))
	}
	if !Supported(ext) {
		return "", ErrUnsupportedExt
	}

	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("打开文档失败: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	stat, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("读取文档信息失败: %w", err)
	}
	if stat.Size() <= 0 {
		return "", nil
	}

	switch ext {
	case ExtPDF:
		return loadDocuments(ctx, documentloaders.NewPDF(file, stat.Size()))
	case ExtHTML:
		return loadDocuments(ctx, documentloaders.NewHTML(file))
	case ExtCSV:
		return loadDocuments(ctx, documentloaders.NewCSV(file))
	case ExtText, ExtMarkdown:
		return loadDocuments(ctx, documentloaders.NewText(file))
	case ExtDocx:
		return loadDocx(file, stat.Size())
	}
	return "", ErrUnsupportedExt
}

// loadDocuments 用 langchaingo 的加载器解析文档并按段落拼接文本
func loadDocuments(ctx context.Context, loader documentloaders.Loader) (string, error) {
	docs, err := loader.Load(ctx)
	if err != nil {
		return "", fmt.Errorf("解析文档失败: %w", err)
	}

	parts := make([]string, 0, len(docs))
	for _, doc := range docs {
		if text := strings.TrimSpace(doc.PageContent); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n\n"), nil
}

// loadDocx 用 go-docx 提取 docx 的段落与表格文本；
// 这里自己遍历节点，不调 Paragraph.String，避免文档缺少关联信息时取超链接目标报错
func loadDocx(file *os.File, size int64) (string, error) {
	doc, err := docx.Parse(file, size)
	if err != nil {
		return "", fmt.Errorf("解析 docx 失败: %w", err)
	}

	parts := make([]string, 0, len(doc.Document.Body.Items))
	for _, item := range doc.Document.Body.Items {
		switch value := item.(type) {
		case *docx.Paragraph:
			if text := paragraphText(value); text != "" {
				parts = append(parts, text)
			}
		case *docx.Table:
			parts = append(parts, tableText(value)...)
		}
	}
	return strings.Join(parts, "\n"), nil
}

// paragraphText 提取一个段落里的文字，超链接里的文字一并取出
func paragraphText(paragraph *docx.Paragraph) string {
	var builder strings.Builder
	for _, child := range paragraph.Children {
		switch value := child.(type) {
		case *docx.Run:
			writeRunText(&builder, value)
		case *docx.Hyperlink:
			writeRunText(&builder, &value.Run)
		}
	}
	return strings.TrimSpace(builder.String())
}

// tableText 提取表格里的文字，每个单元格一行，嵌套表格递归展开
func tableText(table *docx.Table) []string {
	parts := make([]string, 0, len(table.TableRows))
	for _, row := range table.TableRows {
		cells := make([]string, 0, len(row.TableCells))
		for _, cell := range row.TableCells {
			lines := make([]string, 0, len(cell.Paragraphs))
			for _, paragraph := range cell.Paragraphs {
				if text := paragraphText(paragraph); text != "" {
					lines = append(lines, text)
				}
			}
			for _, nested := range cell.Tables {
				lines = append(lines, tableText(nested)...)
			}
			cells = append(cells, strings.Join(lines, " "))
		}
		if text := strings.TrimSpace(strings.Join(cells, " | ")); text != "" {
			parts = append(parts, text)
		}
	}
	return parts
}

// writeRunText 把一个文本片段写进缓冲区
func writeRunText(builder *strings.Builder, run *docx.Run) {
	for _, child := range run.Children {
		switch value := child.(type) {
		case *docx.Text:
			builder.WriteString(value.Text)
		case *docx.Tab:
			builder.WriteByte('\t')
		case *docx.BarterRabbet:
			builder.WriteByte('\n')
		}
	}
}
