package note

import (
	"strings"
	"testing"
)

func TestParserParse(t *testing.T) {
	p := NewParser()

	tests := []struct {
		name      string
		content   string
		wantTitle string
		wantTags  []string
	}{
		{
			"普通笔记",
			"今天想到了产品设计灵感 #产品 #灵感\n\n具体内容...",
			"今天想到了产品设计灵感",
			[]string{"产品", "灵感"},
		},
		{
			"仅标签",
			"#Go #Rust",
			"无标题",
			[]string{"Go", "Rust"},
		},
		{
			"空内容",
			"",
			"无标题",
			nil,
		},
		{
			"长文本",
			strings.Repeat("测试", 60),
			strings.Repeat("测试", 15) + "…",
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.Parse(tt.content)
			if result.Title != tt.wantTitle {
				t.Errorf("Title = %q, want %q", result.Title, tt.wantTitle)
			}
			if !sliceEqual(result.Tags, tt.wantTags) {
				t.Errorf("Tags = %v, want %v", result.Tags, tt.wantTags)
			}
		})
	}
}

func sliceEqual(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
