package mdutil

import (
	"strings"
	"testing"
)

func TestExtractTitle(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"普通文本", "今天想到了一个好点子", "今天想到了一个好点子"},
		{"含标签", "产品设计灵感 #产品 #灵感", "产品设计灵感"},
		{"仅标签", "#产品 #灵感", "无标题"},
		{"空内容", "", "无标题"},
		{"仅空白", "   \n  \n  ", "无标题"},
		{"多行取首行", "第一行标题\n第二行 #Go", "第一行标题"},
		{"首行空", "\n\n有效标题\n其他", "有效标题"},
		{"超长截断", strings.Repeat("测", 40), strings.Repeat("测", 30) + "…"},
		{"恰好30不截断", strings.Repeat("字", 30), strings.Repeat("字", 30)},
		{"混合中英文", "Hello 世界 #test", "Hello 世界"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractTitle(tt.content)
			if got != tt.want {
				t.Errorf("ExtractTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		maxRunes int
		want     string
	}{
		{"短字符串", "hello", 10, "hello"},
		{"恰好等于", "hello", 5, "hello"},
		{"需截断", "hello world", 5, "hello…"},
		{"中文截断", "你好世界测试", 4, "你好世界…"},
		{"空字符串", "", 10, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.s, tt.maxRunes)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.s, tt.maxRunes, got, tt.want)
			}
		})
	}
}
