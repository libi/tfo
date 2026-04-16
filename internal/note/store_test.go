package note

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func setupStore(t *testing.T) (*FileStore, string) {
	t.Helper()
	d := t.TempDir()
	return NewFileStore(d), d
}

func TestSave(t *testing.T) {
	store, dir := setupStore(t)
	ctx := context.Background()

	n, err := store.Save(ctx, "测试 #Go #技术")
	if err != nil {
		t.Fatal(err)
	}
	if n.ID == "" || n.Content != "测试 #Go #技术" {
		t.Errorf("unexpected note: %+v", n)
	}

	abs := filepath.Join(dir, n.FilePath)
	data, _ := os.ReadFile(abs)
	if string(data) != "测试 #Go #技术" {
		t.Errorf("file content mismatch")
	}
}

func TestSaveConflict(t *testing.T) {
	store, _ := setupStore(t)
	ctx := context.Background()

	n1, _ := store.Save(ctx, "a")
	n2, _ := store.Save(ctx, "b")
	if n1.ID == n2.ID {
		t.Error("IDs should differ")
	}
}

func TestLoadAndUpdate(t *testing.T) {
	store, _ := setupStore(t)
	ctx := context.Background()

	saved, _ := store.Save(ctx, "原始 #old")
	loaded, err := store.Load(ctx, saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Content != "原始 #old" {
		t.Errorf("content = %q", loaded.Content)
	}

	updated, err := store.Update(ctx, saved.ID, "新内容 #new")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Content != "新内容 #new" {
		t.Errorf("updated content = %q", updated.Content)
	}
}

func TestDelete(t *testing.T) {
	store, dir := setupStore(t)
	ctx := context.Background()

	saved, _ := store.Save(ctx, "delete me")
	abs := filepath.Join(dir, saved.FilePath)
	if _, err := os.Stat(abs); os.IsNotExist(err) {
		t.Fatal("file should exist")
	}

	if err := store.Delete(ctx, saved.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(abs); !os.IsNotExist(err) {
		t.Error("file should be deleted")
	}
}

func TestListByMonth(t *testing.T) {
	store, dir := setupStore(t)
	ctx := context.Background()

	month := "2026-04"
	dp := filepath.Join(dir, month)
	os.MkdirAll(dp, 0o755)
	os.WriteFile(filepath.Join(dp, "20260415-100000.md"), []byte("n1"), 0o644)
	os.WriteFile(filepath.Join(dp, "20260415-110000.md"), []byte("n2"), 0o644)
	os.WriteFile(filepath.Join(dp, "20260416-090000.md"), []byte("n3"), 0o644)

	list, err := store.ListByMonth(ctx, month)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Errorf("got %d, want 3", len(list))
	}
}

func TestListByDate(t *testing.T) {
	store, dir := setupStore(t)
	ctx := context.Background()

	dp := filepath.Join(dir, "2026-04")
	os.MkdirAll(dp, 0o755)
	os.WriteFile(filepath.Join(dp, "20260415-100000.md"), []byte("a"), 0o644)
	os.WriteFile(filepath.Join(dp, "20260415-110000.md"), []byte("b"), 0o644)
	os.WriteFile(filepath.Join(dp, "20260416-090000.md"), []byte("c"), 0o644)

	date := time.Date(2026, 4, 15, 0, 0, 0, 0, time.Local)
	list, err := store.ListByDate(ctx, date)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Errorf("got %d, want 2", len(list))
	}
}

func TestScanAll(t *testing.T) {
	store, dir := setupStore(t)
	ctx := context.Background()

	for _, m := range []string{"2026-03", "2026-04"} {
		os.MkdirAll(filepath.Join(dir, m), 0o755)
	}
	os.WriteFile(filepath.Join(dir, "2026-03", "20260315-100000.md"), []byte("a"), 0o644)
	os.WriteFile(filepath.Join(dir, "2026-04", "20260415-100000.md"), []byte("b"), 0o644)
	os.WriteFile(filepath.Join(dir, "2026-04", "20260416-100000.md"), []byte("c"), 0o644)

	var count int64
	err := store.ScanAll(ctx, func(n *Note) error {
		atomic.AddInt64(&count, 1)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Errorf("scanned %d, want 3", count)
	}
}

func TestScanAllSkipsHidden(t *testing.T) {
	store, dir := setupStore(t)
	ctx := context.Background()

	os.MkdirAll(filepath.Join(dir, ".index"), 0o755)
	os.WriteFile(filepath.Join(dir, ".index", "x.md"), []byte("skip"), 0o644)
	os.MkdirAll(filepath.Join(dir, "2026-04"), 0o755)
	os.WriteFile(filepath.Join(dir, "2026-04", "20260415-100000.md"), []byte("keep"), 0o644)

	var count int64
	store.ScanAll(ctx, func(n *Note) error {
		atomic.AddInt64(&count, 1)
		return nil
	})
	if count != 1 {
		t.Errorf("scanned %d, want 1", count)
	}
}

func TestGetHeatmap(t *testing.T) {
	store, dir := setupStore(t)
	ctx := context.Background()

	dp := filepath.Join(dir, "2026-04")
	os.MkdirAll(dp, 0o755)
	os.WriteFile(filepath.Join(dp, "20260415-100000.md"), []byte("a"), 0o644)
	os.WriteFile(filepath.Join(dp, "20260415-110000.md"), []byte("b"), 0o644)
	os.WriteFile(filepath.Join(dp, "20260415-120000.md"), []byte("c"), 0o644)
	os.WriteFile(filepath.Join(dp, "20260416-090000.md"), []byte("d"), 0o644)

	hm, err := store.GetHeatmap(ctx, "2026-04")
	if err != nil {
		t.Fatal(err)
	}
	if len(hm) != 2 {
		t.Fatalf("got %d entries, want 2", len(hm))
	}

	sort.Slice(hm, func(i, j int) bool { return hm[i].Date < hm[j].Date })
	if hm[0].Date != "2026-04-15" || hm[0].Count != 3 {
		t.Errorf("[0] = %+v", hm[0])
	}
	if hm[1].Date != "2026-04-16" || hm[1].Count != 1 {
		t.Errorf("[1] = %+v", hm[1])
	}
}

func TestParseTimeFromID(t *testing.T) {
	if _, err := parseTimeFromID("20260415-203000"); err != nil {
		t.Error(err)
	}
	if _, err := parseTimeFromID("20260415-203000-1"); err != nil {
		t.Error(err)
	}
	if _, err := parseTimeFromID("invalid"); err == nil {
		t.Error("should fail")
	}
}

func TestMonthDirCreated(t *testing.T) {
	store, dir := setupStore(t)
	store.Save(context.Background(), "test")

	entries, _ := os.ReadDir(dir)
	found := false
	for _, e := range entries {
		if e.IsDir() && strings.Contains(e.Name(), "-") {
			found = true
		}
	}
	if !found {
		t.Error("month dir not created")
	}
}
