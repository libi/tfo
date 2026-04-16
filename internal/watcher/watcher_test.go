package watcher

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestWatcher_DetectsNewFile(t *testing.T) {
	dir := t.TempDir()

	// 创建月份子目录
	monthDir := filepath.Join(dir, "2025-01")
	os.MkdirAll(monthDir, 0o755)

	var mu sync.Mutex
	var events []FileEvent

	w, err := New(dir, func(event FileEvent) {
		mu.Lock()
		events = append(events, event)
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go w.Start(ctx)

	// 等待 watcher 启动
	time.Sleep(50 * time.Millisecond)

	// 写入新文件
	testFile := filepath.Join(monthDir, "20250115-100000.md")
	os.WriteFile(testFile, []byte("# test"), 0o644)

	// 等待去抖
	time.Sleep(300 * time.Millisecond)

	cancel()
	// 不调用 w.Stop() 因为 cancel 已经退出了 Start

	mu.Lock()
	defer mu.Unlock()

	if len(events) == 0 {
		t.Fatal("expected at least 1 event")
	}

	found := false
	for _, e := range events {
		if e.Type == EventCreated && filepath.Base(e.FilePath) == "20250115-100000.md" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected EventCreated for test file, got events: %+v", events)
	}
}

func TestWatcher_IgnoresNonMdFiles(t *testing.T) {
	dir := t.TempDir()
	monthDir := filepath.Join(dir, "2025-01")
	os.MkdirAll(monthDir, 0o755)

	var mu sync.Mutex
	var events []FileEvent

	w, err := New(dir, func(event FileEvent) {
		mu.Lock()
		events = append(events, event)
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go w.Start(ctx)

	time.Sleep(50 * time.Millisecond)

	// 写入非 .md 文件
	os.WriteFile(filepath.Join(monthDir, "test.txt"), []byte("not markdown"), 0o644)

	time.Sleep(300 * time.Millisecond)
	cancel()

	mu.Lock()
	defer mu.Unlock()

	if len(events) != 0 {
		t.Fatalf("expected 0 events for .txt file, got %d: %+v", len(events), events)
	}
}

func TestWatcher_IgnoresIndexDir(t *testing.T) {
	dir := t.TempDir()
	indexDir := filepath.Join(dir, ".index")
	os.MkdirAll(indexDir, 0o755)

	var mu sync.Mutex
	var events []FileEvent

	w, err := New(dir, func(event FileEvent) {
		mu.Lock()
		events = append(events, event)
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go w.Start(ctx)

	time.Sleep(50 * time.Millisecond)

	// 在 .index 目录写入 .md 文件
	os.WriteFile(filepath.Join(indexDir, "test.md"), []byte("index file"), 0o644)

	time.Sleep(300 * time.Millisecond)
	cancel()

	mu.Lock()
	defer mu.Unlock()

	if len(events) != 0 {
		t.Fatalf("expected 0 events for .index dir, got %d: %+v", len(events), events)
	}
}

func TestWatcher_DetectsNewMonthDir(t *testing.T) {
	dir := t.TempDir()

	var mu sync.Mutex
	var events []FileEvent

	w, err := New(dir, func(event FileEvent) {
		mu.Lock()
		events = append(events, event)
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go w.Start(ctx)

	time.Sleep(50 * time.Millisecond)

	// 创建新的月份目录并写入文件
	newMonthDir := filepath.Join(dir, "2025-03")
	os.MkdirAll(newMonthDir, 0o755)

	// 等待目录被添加到监控
	time.Sleep(200 * time.Millisecond)

	// 在新目录中写入文件
	testFile := filepath.Join(newMonthDir, "20250310-120000.md")
	os.WriteFile(testFile, []byte("# new month note"), 0o644)

	time.Sleep(300 * time.Millisecond)
	cancel()

	mu.Lock()
	defer mu.Unlock()

	found := false
	for _, e := range events {
		if e.Type == EventCreated && filepath.Base(e.FilePath) == "20250310-120000.md" {
			found = true
			break
		}
	}
	if !found {
		t.Logf("events: %+v", events)
		t.Fatal("expected EventCreated for file in new month dir")
	}
}

func TestWatcher_DetectsDelete(t *testing.T) {
	dir := t.TempDir()
	monthDir := filepath.Join(dir, "2025-01")
	os.MkdirAll(monthDir, 0o755)

	testFile := filepath.Join(monthDir, "20250115-100000.md")
	os.WriteFile(testFile, []byte("# to delete"), 0o644)

	var mu sync.Mutex
	var events []FileEvent

	w, err := New(dir, func(event FileEvent) {
		mu.Lock()
		events = append(events, event)
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go w.Start(ctx)

	time.Sleep(50 * time.Millisecond)

	os.Remove(testFile)

	time.Sleep(300 * time.Millisecond)
	cancel()

	mu.Lock()
	defer mu.Unlock()

	found := false
	for _, e := range events {
		if e.Type == EventDeleted {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected EventDeleted, got events: %+v", events)
	}
}
