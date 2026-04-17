package search

import (
	"context"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/query"
)

// HighlightRange 高亮区间（字符偏移，非字节偏移，可直接对应 JS string index）
type HighlightRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// HighlightedFragment 带高亮信息的文本片段
type HighlightedFragment struct {
	Text       string           `json:"text"`
	Highlights []HighlightRange `json:"highlights,omitempty"`
}

// SearchResult 搜索结果
type SearchResult struct {
	ID        string                `json:"id"`
	Title     string                `json:"title"`
	Score     float64               `json:"score"`
	Fragments []HighlightedFragment `json:"fragments"`
}

// Searcher 搜索查询接口
type Searcher interface {
	Search(ctx context.Context, query string, offset, limit int) ([]*SearchResult, int, error)
	SearchByTag(ctx context.Context, tags []string, offset, limit int) ([]*SearchResult, int, error)
	SearchByDateRange(ctx context.Context, from, to time.Time, offset, limit int) ([]*SearchResult, int, error)
}

// BleveSearcher 基于 Bleve 的搜索实现
type BleveSearcher struct {
	indexer *BleveIndexer
}

// NewBleveSearcher 创建搜索器，需要传入 indexer（动态获取最新 index 实例）
func NewBleveSearcher(indexer *BleveIndexer) *BleveSearcher {
	return &BleveSearcher{indexer: indexer}
}

// getIndex 返回当前有效的 bleve.Index
func (s *BleveSearcher) getIndex() bleve.Index {
	if s.indexer == nil {
		return nil
	}
	return s.indexer.GetIndex()
}

// Search 全文搜索
func (s *BleveSearcher) Search(ctx context.Context, queryStr string, offset, limit int) ([]*SearchResult, int, error) {
	idx := s.getIndex()
	if idx == nil {
		return nil, 0, ErrIndexNotOpen
	}
	if limit <= 0 {
		limit = 20
	}

	// 按空格拆分搜索词，每个词独立构造子查询，最终取交集（AND）
	terms := strings.Fields(strings.TrimSpace(queryStr))
	if len(terms) == 0 {
		return nil, 0, nil
	}

	termQueries := make([]query.Query, 0, len(terms))
	for _, term := range terms {
		termRunes := []rune(term)
		var tq query.Query
		if len(termRunes) <= 2 && isAllLetterOrDigit(termRunes) {
			// 短词用 wildcard，实现单字/双字搜索
			pattern := "*" + strings.ToLower(term) + "*"
			titleWild := bleve.NewWildcardQuery(pattern)
			titleWild.SetField("title")
			titleWild.SetBoost(3.0)
			contentWild := bleve.NewWildcardQuery(pattern)
			contentWild.SetField("content")
			tq = bleve.NewDisjunctionQuery(titleWild, contentWild)
		} else {
			titleMatch := bleve.NewMatchQuery(term)
			titleMatch.SetField("title")
			titleMatch.SetBoost(3.0)
			contentMatch := bleve.NewMatchQuery(term)
			contentMatch.SetField("content")
			tq = bleve.NewDisjunctionQuery(titleMatch, contentMatch)
		}
		termQueries = append(termQueries, tq)
	}

	var combined query.Query
	if len(termQueries) == 1 {
		combined = termQueries[0]
	} else {
		// 多词取交集：每个词都必须命中
		combined = bleve.NewConjunctionQuery(termQueries...)
	}

	req := bleve.NewSearchRequestOptions(combined, limit, offset, false)
	req.Fields = []string{"title", "content"}

	result, err := idx.SearchInContext(ctx, req)
	if err != nil {
		return nil, 0, err
	}

	return mapSearchResults(result, terms), int(result.Total), nil
}

// SearchByTag 按标签搜索
func (s *BleveSearcher) SearchByTag(ctx context.Context, tags []string, offset, limit int) ([]*SearchResult, int, error) {
	idx := s.getIndex()
	if idx == nil {
		return nil, 0, ErrIndexNotOpen
	}
	if limit <= 0 {
		limit = 20
	}
	if len(tags) == 0 {
		return nil, 0, nil
	}

	queries := make([]query.Query, 0, len(tags))
	for _, tag := range tags {
		tq := bleve.NewTermQuery(tag)
		tq.SetField("tags")
		queries = append(queries, tq)
	}

	// 多标签用 conjunction (AND)
	combined := bleve.NewConjunctionQuery(queries...)

	req := bleve.NewSearchRequestOptions(combined, limit, offset, false)
	req.Fields = []string{"title"}

	result, err := idx.SearchInContext(ctx, req)
	if err != nil {
		return nil, 0, err
	}

	return mapSearchResults(result, nil), int(result.Total), nil
}

// SearchByDateRange 按日期范围搜索
func (s *BleveSearcher) SearchByDateRange(ctx context.Context, from, to time.Time, offset, limit int) ([]*SearchResult, int, error) {
	idx := s.getIndex()
	if idx == nil {
		return nil, 0, ErrIndexNotOpen
	}
	if limit <= 0 {
		limit = 20
	}

	inclusive := true
	drq := bleve.NewDateRangeInclusiveQuery(from, to, &inclusive, &inclusive)
	drq.SetField("createdAt")

	req := bleve.NewSearchRequestOptions(drq, limit, offset, false)
	req.Fields = []string{"title"}
	req.SortBy([]string{"-createdAt"})

	result, err := idx.SearchInContext(ctx, req)
	if err != nil {
		return nil, 0, err
	}

	return mapSearchResults(result, nil), int(result.Total), nil
}

// mapSearchResults 将 bleve 搜索结果转为 SearchResult 列表
func mapSearchResults(result *bleve.SearchResult, searchTerms []string) []*SearchResult {
	results := make([]*SearchResult, 0, len(result.Hits))
	for _, hit := range result.Hits {
		sr := &SearchResult{
			ID:    hit.ID,
			Score: hit.Score,
		}
		if title, ok := hit.Fields["title"].(string); ok {
			sr.Title = title
		}
		content, _ := hit.Fields["content"].(string)
		sr.Fragments = buildFragments(content, searchTerms)
		results = append(results, sr)
	}
	return results
}

// isAllLetterOrDigit 检查 rune 切片是否全为字母或数字
func isAllLetterOrDigit(runes []rune) bool {
	for _, r := range runes {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

const maxFragmentRunes = 500

// buildFragments 根据原文和搜索词，在原文中做子串匹配生成高亮区间
func buildFragments(content string, searchTerms []string) []HighlightedFragment {
	if content == "" {
		return nil
	}

	contentLower := strings.ToLower(content)
	contentRunes := []rune(content)
	contentLowerRunes := []rune(contentLower)

	// 在 rune 层面做子串匹配，收集高亮区间（rune 偏移）
	var highlights []HighlightRange
	for _, term := range searchTerms {
		if term == "" {
			continue
		}
		termRunes := []rune(strings.ToLower(term))
		termLen := len(termRunes)
		for i := 0; i <= len(contentLowerRunes)-termLen; i++ {
			matched := true
			for j := 0; j < termLen; j++ {
				if contentLowerRunes[i+j] != termRunes[j] {
					matched = false
					break
				}
			}
			if matched {
				highlights = append(highlights, HighlightRange{Start: i, End: i + termLen})
			}
		}
	}

	// 排序并合并重叠区间
	sort.Slice(highlights, func(i, j int) bool { return highlights[i].Start < highlights[j].Start })
	highlights = mergeRanges(highlights)

	// 截断过长文本
	text := content
	if len(contentRunes) > maxFragmentRunes {
		text = string(contentRunes[:maxFragmentRunes]) + "…"
		filtered := make([]HighlightRange, 0, len(highlights))
		for _, h := range highlights {
			if h.Start < maxFragmentRunes {
				if h.End > maxFragmentRunes {
					h.End = maxFragmentRunes
				}
				filtered = append(filtered, h)
			}
		}
		highlights = filtered
	}

	return []HighlightedFragment{{Text: text, Highlights: highlights}}
}

// mergeRanges 合并排序后的重叠/相邻区间
func mergeRanges(ranges []HighlightRange) []HighlightRange {
	if len(ranges) == 0 {
		return nil
	}
	merged := []HighlightRange{ranges[0]}
	for _, r := range ranges[1:] {
		last := &merged[len(merged)-1]
		if r.Start <= last.End {
			if r.End > last.End {
				last.End = r.End
			}
		} else {
			merged = append(merged, r)
		}
	}
	return merged
}
