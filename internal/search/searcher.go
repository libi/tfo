package search

import (
	"context"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/highlight/highlighter/ansi"
	"github.com/blevesearch/bleve/v2/search/query"
)

// SearchResult 搜索结果
type SearchResult struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Score     float64  `json:"score"`
	Fragments []string `json:"fragments"`
}

// Searcher 搜索查询接口
type Searcher interface {
	Search(ctx context.Context, query string, offset, limit int) ([]*SearchResult, int, error)
	SearchByTag(ctx context.Context, tags []string, offset, limit int) ([]*SearchResult, int, error)
	SearchByDateRange(ctx context.Context, from, to time.Time, offset, limit int) ([]*SearchResult, int, error)
}

// BleveSearcher 基于 Bleve 的搜索实现
type BleveSearcher struct {
	index bleve.Index
}

// NewBleveSearcher 创建搜索器，需要传入 indexer 的 bleve.Index
func NewBleveSearcher(index bleve.Index) *BleveSearcher {
	return &BleveSearcher{index: index}
}

// Search 全文搜索
func (s *BleveSearcher) Search(ctx context.Context, query string, offset, limit int) ([]*SearchResult, int, error) {
	if s.index == nil {
		return nil, 0, ErrIndexNotOpen
	}
	if limit <= 0 {
		limit = 20
	}

	// 对 title 使用较高权重
	titleQuery := bleve.NewMatchQuery(query)
	titleQuery.SetField("title")
	titleQuery.SetBoost(3.0)

	contentQuery := bleve.NewMatchQuery(query)
	contentQuery.SetField("content")

	combined := bleve.NewDisjunctionQuery(titleQuery, contentQuery)

	req := bleve.NewSearchRequestOptions(combined, limit, offset, false)
	req.Fields = []string{"title"}
	req.Highlight = bleve.NewHighlightWithStyle(ansi.Name)

	result, err := s.index.SearchInContext(ctx, req)
	if err != nil {
		return nil, 0, err
	}

	return mapSearchResults(result), int(result.Total), nil
}

// SearchByTag 按标签搜索
func (s *BleveSearcher) SearchByTag(ctx context.Context, tags []string, offset, limit int) ([]*SearchResult, int, error) {
	if s.index == nil {
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

	result, err := s.index.SearchInContext(ctx, req)
	if err != nil {
		return nil, 0, err
	}

	return mapSearchResults(result), int(result.Total), nil
}

// SearchByDateRange 按日期范围搜索
func (s *BleveSearcher) SearchByDateRange(ctx context.Context, from, to time.Time, offset, limit int) ([]*SearchResult, int, error) {
	if s.index == nil {
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

	result, err := s.index.SearchInContext(ctx, req)
	if err != nil {
		return nil, 0, err
	}

	return mapSearchResults(result), int(result.Total), nil
}

// mapSearchResults 将 bleve 搜索结果转为 SearchResult 列表
func mapSearchResults(result *bleve.SearchResult) []*SearchResult {
	results := make([]*SearchResult, 0, len(result.Hits))
	for _, hit := range result.Hits {
		sr := &SearchResult{
			ID:    hit.ID,
			Score: hit.Score,
		}
		if title, ok := hit.Fields["title"].(string); ok {
			sr.Title = title
		}
		// 收集高亮片段
		for _, fragments := range hit.Fragments {
			sr.Fragments = append(sr.Fragments, fragments...)
		}
		results = append(results, sr)
	}
	return results
}
