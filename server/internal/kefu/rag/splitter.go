package rag

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/tmc/langchaingo/textsplitter"

	"admin/server/pkg/setting"
)

// 分块参数未配置时使用的默认值，与迁移种子的 kefu.rag.chunk_size / chunk_overlap 保持一致
const (
	defaultChunkSize    = 600
	defaultChunkOverlap = 80
)

// 分块参数在配置中心的配置键
const (
	configKeyChunkSize    = "kefu.rag.chunk_size"
	configKeyChunkOverlap = "kefu.rag.chunk_overlap"
)

// Chunk 是一个分块结果
type Chunk struct {
	// 分块序号, 从 1 开始
	Seq int
	// 分块内容
	Content string
	// 内容长度, 按字符数估算 token 数
	Tokens int
}

// Split 按扩展名切分文本：md 按标题切，其余按段落递归切分、超长再硬切；
// 分块参数取配置中心，空白分块会被丢弃
func Split(text, ext string) ([]Chunk, error) {
	content := normalizeText(text)
	if content == "" {
		return nil, nil
	}

	parts, err := newSplitter(ext).SplitText(content)
	if err != nil {
		return nil, fmt.Errorf("切分文档失败: %w", err)
	}

	chunks := make([]Chunk, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		chunks = append(chunks, Chunk{
			Seq:     len(chunks) + 1,
			Content: value,
			Tokens:  utf8.RuneCountInString(value),
		})
	}
	return chunks, nil
}

// newSplitter 创建切分器，分块大小与重叠取配置中心，未配置时用默认值
func newSplitter(ext string) textsplitter.TextSplitter {
	size := setting.GetInt(configKeyChunkSize, defaultChunkSize)
	if size <= 0 {
		size = defaultChunkSize
	}
	overlap := setting.GetInt(configKeyChunkOverlap, defaultChunkOverlap)
	if overlap < 0 {
		overlap = defaultChunkOverlap
	}
	// 重叠不能大于分块大小，配置错了按分块的五分之一兜底
	if overlap >= size {
		overlap = size / 5
	}

	options := []textsplitter.Option{
		textsplitter.WithChunkSize(size),
		textsplitter.WithChunkOverlap(overlap),
	}
	if NormalizeExt(ext) == ExtMarkdown {
		return textsplitter.NewMarkdownTextSplitter(options...)
	}
	return textsplitter.NewRecursiveCharacter(options...)
}

// normalizeText 归一化文档文本：统一换行符并去掉行尾空白，避免切出大量空白分块
func normalizeText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	lines := strings.Split(text, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
