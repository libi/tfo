package note

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/libi/tfo/internal/search"
)

// Service 是笔记业务层，通过 Gin HTTP API 暴露给前端
type Service struct {
	store    Store
	parser   *Parser
	indexer  search.Indexer
	searcher search.Searcher
}

// NewService 创建笔记服务
func NewService(store Store) *Service {
	return &Service{store: store, parser: NewParser()}
}

// SetSearch 注入搜索组件（Phase 2）
func (s *Service) SetSearch(indexer search.Indexer, searcher search.Searcher) {
	s.indexer = indexer
	s.searcher = searcher
}

// Create 创建新碎片笔记
func (s *Service) Create(ctx context.Context, content string) (*Note, error) {
	if content == "" {
		return nil, fmt.Errorf("content cannot be empty")
	}
	n, err := s.store.Save(ctx, content)
	if err != nil {
		return nil, err
	}
	s.indexNote(n)
	return n, nil
}

// Get 获取单条笔记
func (s *Service) Get(ctx context.Context, id string) (*Note, error) {
	if id == "" {
		return nil, fmt.Errorf("note id cannot be empty")
	}
	return s.store.Load(ctx, id)
}

// Update 更新笔记内容
func (s *Service) Update(ctx context.Context, id string, content string) (*Note, error) {
	if id == "" {
		return nil, fmt.Errorf("note id cannot be empty")
	}
	if content == "" {
		return nil, fmt.Errorf("content cannot be empty")
	}
	n, err := s.store.Update(ctx, id, content)
	if err != nil {
		return nil, err
	}
	s.indexNote(n)
	return n, nil
}

// Delete 删除笔记
func (s *Service) Delete(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("note id cannot be empty")
	}
	if err := s.store.Delete(ctx, id); err != nil {
		return err
	}
	s.removeIndex(id)
	return nil
}

// ListByDate 按日期列出笔记
func (s *Service) ListByDate(ctx context.Context, dateStr string) ([]*NoteSummary, error) {
	date, err := time.ParseInLocation("2006-01-02", dateStr, time.Local)
	if err != nil {
		return nil, fmt.Errorf("invalid date %q: %w", dateStr, err)
	}
	return s.store.ListByDate(ctx, date)
}

// ListByMonth 按月列出笔记
func (s *Service) ListByMonth(ctx context.Context, month string) ([]*NoteSummary, error) {
	if _, err := time.ParseInLocation("2006-01", month, time.Local); err != nil {
		return nil, fmt.Errorf("invalid month %q: %w", month, err)
	}
	return s.store.ListByMonth(ctx, month)
}

// GetHeatmap 获取日历热力图数据
func (s *Service) GetHeatmap(ctx context.Context, month string) ([]CalendarHeatmapEntry, error) {
	if _, err := time.ParseInLocation("2006-01", month, time.Local); err != nil {
		return nil, fmt.Errorf("invalid month %q: %w", month, err)
	}
	return s.store.GetHeatmap(ctx, month)
}

// GetAllTags 获取所有标签及计数
func (s *Service) GetAllTags(ctx context.Context) ([]TagCount, error) {
	tagMap := make(map[string]int)
	err := s.store.ScanAll(ctx, func(n *Note) error {
		for _, tag := range n.Tags {
			tagMap[tag]++
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan for tags: %w", err)
	}

	result := make([]TagCount, 0, len(tagMap))
	for tag, count := range tagMap {
		result = append(result, TagCount{Tag: tag, Count: count})
	}

	// 按计数倒序
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].Count > result[i].Count {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	return result, nil
}

// Search 全文搜索
func (s *Service) Search(ctx context.Context, query string, limit int) ([]*search.SearchResult, int, error) {
	if s.searcher == nil {
		return nil, 0, fmt.Errorf("search not initialized")
	}
	if limit <= 0 {
		limit = 20
	}
	return s.searcher.Search(ctx, query, 0, limit)
}

// SearchByTag 按标签搜索
func (s *Service) SearchByTag(ctx context.Context, tags []string, limit int) ([]*search.SearchResult, int, error) {
	if s.searcher == nil {
		return nil, 0, fmt.Errorf("search not initialized")
	}
	if limit <= 0 {
		limit = 20
	}
	return s.searcher.SearchByTag(ctx, tags, 0, limit)
}

// RebuildIndex 全量重建搜索索引
func (s *Service) RebuildIndex(ctx context.Context) error {
	if s.indexer == nil {
		return fmt.Errorf("search not initialized")
	}
	scanFn := func(ctx context.Context, callback func(*search.IndexDocument) error) error {
		return s.store.ScanAll(ctx, func(n *Note) error {
			return callback(&search.IndexDocument{
				ID:        n.ID,
				Title:     n.Title,
				Content:   n.Content,
				Tags:      n.Tags,
				CreatedAt: n.CreatedAt,
			})
		})
	}
	return s.indexer.Rebuild(ctx, scanFn)
}

// indexNote 将笔记同步到搜索索引（best-effort, 失败仅记日志）
func (s *Service) indexNote(n *Note) {
	if s.indexer == nil {
		return
	}
	doc := &search.IndexDocument{
		ID:        n.ID,
		Title:     n.Title,
		Content:   n.Content,
		Tags:      n.Tags,
		CreatedAt: n.CreatedAt,
	}
	if err := s.indexer.Index(doc); err != nil {
		log.Printf("[service] index note %s: %v", n.ID, err)
	}
}

// removeIndex 从搜索索引删除（best-effort）
func (s *Service) removeIndex(id string) {
	if s.indexer == nil {
		return
	}
	if err := s.indexer.Remove(id); err != nil {
		log.Printf("[service] remove index %s: %v", id, err)
	}
}
