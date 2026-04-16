package mdutil

import (
	"regexp"
	"sort"
)

// tagRegexp 匹配 Markdown 正文中的 #标签 语法
// 支持 Unicode 字母、数字、下划线、连字符
var tagRegexp = regexp.MustCompile(`#([\p{L}\p{N}_-]+)`)

// ExtractTags 从 Markdown 内容中提取所有标签，返回去重且排序的标签列表。
func ExtractTags(content string) []string {
	matches := tagRegexp.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(matches))
	tags := make([]string, 0, len(matches))
	for _, m := range matches {
		tag := m[1]
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		tags = append(tags, tag)
	}

	sort.Strings(tags)
	return tags
}
