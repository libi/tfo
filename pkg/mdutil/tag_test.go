package mdutil

import "testing"

func TestExtractTags(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{"单个标签", "今天学了 #Go", []string{"Go"}},
		{"多个标签", "关于 #产品 和 #技术 的思考", []string{"产品", "技术"}},
		{"重复标签去重", "#Go 好语言 #Go #Rust", []string{"Go", "Rust"}},
		{"无标签", "没有标签的文字", nil},
		{"空内容", "", nil},
		{"含下划线连字符", "#user_guide #rust-lang 测试", []string{"rust-lang", "user_guide"}},
		{"中文标签", "#灵感 #阅读笔记 想法", []string{"灵感", "阅读笔记"}},
		{"数字标签", "版本 #v2 发布", []string{"v2"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractTags(tt.content)
			if !strSliceEqual(got, tt.want) {
				t.Errorf("ExtractTags() = %v, want %v", got, tt.want)
			}
		})
	}
}

func strSliceEqual(a, b []string) bool {
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
