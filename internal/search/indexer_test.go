package search

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupTestIndexer(t *testing.T) (*BleveIndexer, string) {
	t.Helper()
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "test.bleve")
	indexer := NewBleveIndexer()
	if err := indexer.Open(indexPath); err != nil {
		t.Fatalf("Open index: %v", err)
	}
	return indexer, dir
}

func TestBleveIndexer_OpenAndClose(t *testing.T) {
	indexer, _ := setupTestIndexer(t)
	defer indexer.Close()

	if indexer.GetIndex() == nil {
		t.Fatal("expected non-nil index after Open")
	}
}

func TestBleveIndexer_IndexAndRemove(t *testing.T) {
	indexer, _ := setupTestIndexer(t)
	defer indexer.Close()

	doc := &IndexDocument{
		ID:        "2025-01/20250115-100000",
		Title:     "测试笔记",
		Content:   "这是一段测试内容 #Go #Bleve",
		Tags:      []string{"Go", "Bleve"},
		CreatedAt: time.Now(),
	}

	if err := indexer.Index(doc); err != nil {
		t.Fatalf("Index: %v", err)
	}

	// 验证文档已被索引
	count, err := indexer.GetIndex().DocCount()
	if err != nil {
		t.Fatalf("DocCount: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 doc, got %d", count)
	}

	// 删除
	if err := indexer.Remove(doc.ID); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	count, err = indexer.GetIndex().DocCount()
	if err != nil {
		t.Fatalf("DocCount after remove: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 docs after remove, got %d", count)
	}
}

func TestBleveIndexer_Rebuild(t *testing.T) {
	indexer, dir := setupTestIndexer(t)
	defer indexer.Close()

	// 先索引一条
	doc := &IndexDocument{
		ID:      "old-doc",
		Title:   "旧文档",
		Content: "会被重建清除",
		Tags:    []string{"old"},
	}
	if err := indexer.Index(doc); err != nil {
		t.Fatalf("Index: %v", err)
	}

	// 重建，用 scanFn 写入两条新文档
	scanFn := func(ctx context.Context, callback func(*IndexDocument) error) error {
		for i := 0; i < 2; i++ {
			d := &IndexDocument{
				ID:        "new-" + string(rune('a'+i)),
				Title:     "新文档",
				Content:   "重建后的内容",
				Tags:      []string{"new"},
				CreatedAt: time.Now(),
			}
			if err := callback(d); err != nil {
				return err
			}
		}
		return nil
	}

	if err := indexer.Rebuild(context.Background(), scanFn); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}

	count, err := indexer.GetIndex().DocCount()
	if err != nil {
		t.Fatalf("DocCount: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 docs after rebuild, got %d", count)
	}

	// 确保索引路径仍在 dir 下
	if _, err := os.Stat(filepath.Join(dir, "test.bleve")); err != nil {
		t.Fatalf("index dir should exist: %v", err)
	}
}

func TestBleveIndexer_NeedsRebuild(t *testing.T) {
	indexer := NewBleveIndexer()
	// 没有设置 indexPath
	if !indexer.NeedsRebuild() {
		t.Fatal("expected NeedsRebuild=true when no indexPath")
	}

	// 设置不存在的路径
	dir := t.TempDir()
	indexer.indexPath = filepath.Join(dir, "nonexistent.bleve")
	if !indexer.NeedsRebuild() {
		t.Fatal("expected NeedsRebuild=true for nonexistent path")
	}

	// 正常打开后
	if err := indexer.Open(indexer.indexPath); err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer indexer.Close()

	if indexer.NeedsRebuild() {
		t.Fatal("expected NeedsRebuild=false after Open")
	}
}

func TestBleveIndexer_IndexWithoutOpen(t *testing.T) {
	indexer := NewBleveIndexer()
	err := indexer.Index(&IndexDocument{ID: "x"})
	if err != ErrIndexNotOpen {
		t.Fatalf("expected ErrIndexNotOpen, got %v", err)
	}
}
