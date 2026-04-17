package search

import (
	"context"
	"testing"
	"time"
)

func seedTestData(t *testing.T) (*BleveIndexer, *BleveSearcher) {
	t.Helper()
	indexer, _ := setupTestIndexer(t)

	docs := []*IndexDocument{
		{
			ID:        "2025-01/20250115-100000",
			Title:     "Go语言学习笔记",
			Content:   "今天学习了 Go 语言的并发编程，goroutine 和 channel 非常强大",
			Tags:      []string{"Go", "学习"},
			CreatedAt: time.Date(2025, 1, 15, 10, 0, 0, 0, time.Local),
		},
		{
			ID:        "2025-01/20250116-140000",
			Title:     "Bleve搜索引擎研究",
			Content:   "Bleve 是一个纯 Go 实现的全文搜索引擎，支持中文分词",
			Tags:      []string{"Go", "Bleve"},
			CreatedAt: time.Date(2025, 1, 16, 14, 0, 0, 0, time.Local),
		},
		{
			ID:        "2025-02/20250210-090000",
			Title:     "读书笔记：设计模式",
			Content:   "观察者模式适合事件驱动的系统设计",
			Tags:      []string{"读书", "设计模式"},
			CreatedAt: time.Date(2025, 2, 10, 9, 0, 0, 0, time.Local),
		},
	}

	for _, doc := range docs {
		if err := indexer.Index(doc); err != nil {
			t.Fatalf("Index: %v", err)
		}
	}

	searcher := NewBleveSearcher(indexer)
	return indexer, searcher
}

func TestBleveSearcher_Search(t *testing.T) {
	indexer, searcher := seedTestData(t)
	defer indexer.Close()

	results, total, err := searcher.Search(context.Background(), "Go", 0, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if total < 1 {
		t.Fatalf("expected at least 1 result for 'Go', got %d", total)
	}
	if len(results) == 0 {
		t.Fatal("expected non-empty results")
	}
}

func TestBleveSearcher_SearchChinese(t *testing.T) {
	indexer, searcher := seedTestData(t)
	defer indexer.Close()

	results, total, err := searcher.Search(context.Background(), "并发编程", 0, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if total < 1 {
		t.Fatalf("expected at least 1 result for '并发编程', got %d", total)
	}
	_ = results
}

func TestBleveSearcher_SearchURLPartial(t *testing.T) {
	indexer, _ := setupTestIndexer(t)
	defer indexer.Close()

	err := indexer.Index(&IndexDocument{
		ID:      "url-test",
		Title:   "链接收藏",
		Content: "推荐访问 baidu.com 和 google.com/search 获取更多信息",
	})
	if err != nil {
		t.Fatalf("Index: %v", err)
	}

	searcher := NewBleveSearcher(indexer)

	// 搜 "baidu" 应能匹配到包含 "baidu.com" 的文档
	results, total, err := searcher.Search(context.Background(), "baidu", 0, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if total < 1 {
		t.Fatalf("expected at least 1 result for 'baidu', got %d", total)
	}
	_ = results
}

func TestBleveSearcher_SearchSingleChar(t *testing.T) {
	indexer, _ := setupTestIndexer(t)
	defer indexer.Close()

	docs := []*IndexDocument{
		{ID: "sc-1", Title: "学习笔记", Content: "今天学习了 Go 语言"},
		{ID: "sc-2", Title: "链接", Content: "推荐 baidu.com"},
	}
	for _, doc := range docs {
		if err := indexer.Index(doc); err != nil {
			t.Fatalf("Index: %v", err)
		}
	}

	searcher := NewBleveSearcher(indexer)

	// 单个汉字 "学" 应能匹配
	results, total, err := searcher.Search(context.Background(), "学", 0, 10)
	if err != nil {
		t.Fatalf("Search single CJK: %v", err)
	}
	if total < 1 {
		t.Fatalf("expected at least 1 result for '学', got %d", total)
	}

	// 单个英文字母 "b" 应能匹配 baidu
	results, total, err = searcher.Search(context.Background(), "b", 0, 10)
	if err != nil {
		t.Fatalf("Search single letter: %v", err)
	}
	if total < 1 {
		t.Fatalf("expected at least 1 result for 'b', got %d", total)
	}
	_ = results
}

func TestBleveSearcher_SearchMultiTermAND(t *testing.T) {
	indexer, _ := setupTestIndexer(t)
	defer indexer.Close()

	docs := []*IndexDocument{
		{ID: "mt-1", Title: "好消息", Content: "今天收到了好消息"},
		{ID: "mt-2", Title: "收藏夹", Content: "收藏了一些链接"},
		{ID: "mt-3", Title: "心情", Content: "感觉好极了"},
	}
	for _, doc := range docs {
		if err := indexer.Index(doc); err != nil {
			t.Fatalf("Index: %v", err)
		}
	}

	searcher := NewBleveSearcher(indexer)

	// "好 收" 应只返回同时包含"好"和"收"的文档（mt-1），而非全部
	results, total, err := searcher.Search(context.Background(), "好 收", 0, 10)
	if err != nil {
		t.Fatalf("Search multi-term: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected exactly 1 result for '好 收', got %d", total)
	}
	if results[0].ID != "mt-1" {
		t.Fatalf("expected mt-1, got %s", results[0].ID)
	}
}

func TestBleveSearcher_SearchByTag(t *testing.T) {
	indexer, searcher := seedTestData(t)
	defer indexer.Close()

	// 单标签
	results, total, err := searcher.SearchByTag(context.Background(), []string{"Go"}, 0, 10)
	if err != nil {
		t.Fatalf("SearchByTag: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected 2 results for tag 'Go', got %d", total)
	}

	// 多标签 (AND)
	results, total, err = searcher.SearchByTag(context.Background(), []string{"Go", "Bleve"}, 0, 10)
	if err != nil {
		t.Fatalf("SearchByTag multi: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected 1 result for tags [Go, Bleve], got %d", total)
	}
	_ = results
}

func TestBleveSearcher_SearchByDateRange(t *testing.T) {
	indexer, searcher := seedTestData(t)
	defer indexer.Close()

	from := time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local)
	to := time.Date(2025, 1, 31, 23, 59, 59, 0, time.Local)

	results, total, err := searcher.SearchByDateRange(context.Background(), from, to, 0, 10)
	if err != nil {
		t.Fatalf("SearchByDateRange: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected 2 results in Jan 2025, got %d", total)
	}
	_ = results
}

func TestBleveSearcher_EmptyQuery(t *testing.T) {
	indexer, searcher := seedTestData(t)
	defer indexer.Close()

	// 空标签
	results, total, err := searcher.SearchByTag(context.Background(), []string{}, 0, 10)
	if err != nil {
		t.Fatalf("SearchByTag empty: %v", err)
	}
	if total != 0 || len(results) != 0 {
		t.Fatalf("expected 0 results for empty tags")
	}
}

func TestBleveSearcher_NotOpen(t *testing.T) {
	searcher := &BleveSearcher{}
	_, _, err := searcher.Search(context.Background(), "test", 0, 10)
	if err != ErrIndexNotOpen {
		t.Fatalf("expected ErrIndexNotOpen, got %v", err)
	}
}
