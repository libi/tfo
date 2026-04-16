package note

import (
	"strings"
	"unicode/utf8"

	"github.com/libi/tfo/pkg/mdutil"
)

const maxPreviewLen = 100

// Parser 负责从 Markdown 内容中提取结构化信息
type Parser struct{}

// NewParser 创建解析器实例
func NewParser() *Parser {
	return &Parser{}
}

// ParseResult 解析结果
type ParseResult struct {
	Title   string
	Tags    []string
	Preview string
}

// Parse 解析 Markdown 内容
func (p *Parser) Parse(content string) *ParseResult {
	return &ParseResult{
		Title:   mdutil.ExtractTitle(content),
		Tags:    mdutil.ExtractTags(content),
		Preview: extractPreview(content),
	}
}

func extractPreview(content string) string {
	preview := strings.Join(strings.Fields(content), " ")
	if utf8.RuneCountInString(preview) <= maxPreviewLen {
		return preview
	}
	runes := []rune(preview)
	return string(runes[:maxPreviewLen]) + "…"
}
