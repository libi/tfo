package mdutil

import (
	"strings"
	"unicode/utf8"
)

const (
	// MaxTitleLen 标题最大字符数（rune 级别）
	MaxTitleLen = 30
)

// ExtractTitle 从 Markdown 内容中提取标题。
func ExtractTitle(content string) string {
	firstLine := extractFirstLine(content)
	if firstLine == "" {
		return "无标题"
	}

	cleaned := tagRegexp.ReplaceAllString(firstLine, "")
	cleaned = strings.TrimSpace(cleaned)

	if cleaned == "" {
		return "无标题"
	}

	return truncate(cleaned, MaxTitleLen)
}

func extractFirstLine(content string) string {
	lines := strings.SplitN(content, "\n", -1)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func truncate(s string, maxRunes int) string {
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	runes := []rune(s)
	return string(runes[:maxRunes]) + "…"
}
