package note

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func setupSvc(t *testing.T) (*Service, string) {
	t.Helper()
	d := t.TempDir()
	return NewService(NewFileStore(d)), d
}

func TestServiceCreate(t *testing.T) {
	svc, _ := setupSvc(t)
	n, err := svc.Create(context.Background(), "hello #Go")
	if err != nil {
		t.Fatal(err)
	}
	if n.Title != "hello" {
		t.Errorf("Title = %q", n.Title)
	}
}

func TestServiceCreateEmpty(t *testing.T) {
	svc, _ := setupSvc(t)
	_, err := svc.Create(context.Background(), "")
	if err == nil {
		t.Fatal("should reject empty")
	}
}

func TestServiceCRUD(t *testing.T) {
	svc, _ := setupSvc(t)
	ctx := context.Background()

	created, _ := svc.Create(ctx, "original #old")
	got, _ := svc.Get(ctx, created.ID)
	if got.Content != "original #old" {
		t.Errorf("content = %q", got.Content)
	}

	updated, _ := svc.Update(ctx, created.ID, "updated #new")
	if updated.Content != "updated #new" {
		t.Errorf("content = %q", updated.Content)
	}

	if err := svc.Delete(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Get(ctx, created.ID)
	if err == nil {
		t.Fatal("should fail after delete")
	}
}

func TestServiceListByDate(t *testing.T) {
	svc, dir := setupSvc(t)
	dp := filepath.Join(dir, "2026-04")
	os.MkdirAll(dp, 0o755)
	os.WriteFile(filepath.Join(dp, "20260415-100000.md"), []byte("a"), 0o644)
	os.WriteFile(filepath.Join(dp, "20260415-110000.md"), []byte("b"), 0o644)
	os.WriteFile(filepath.Join(dp, "20260416-090000.md"), []byte("c"), 0o644)

	list, err := svc.ListByDate(context.Background(), "2026-04-15")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Errorf("got %d, want 2", len(list))
	}
}

func TestServiceListByDateBadFormat(t *testing.T) {
	svc, _ := setupSvc(t)
	_, err := svc.ListByDate(context.Background(), "2026/04/15")
	if err == nil {
		t.Fatal("should reject bad format")
	}
}

func TestServiceGetHeatmap(t *testing.T) {
	svc, dir := setupSvc(t)
	dp := filepath.Join(dir, "2026-04")
	os.MkdirAll(dp, 0o755)
	os.WriteFile(filepath.Join(dp, "20260415-100000.md"), []byte("a"), 0o644)
	os.WriteFile(filepath.Join(dp, "20260415-110000.md"), []byte("b"), 0o644)

	hm, err := svc.GetHeatmap(context.Background(), "2026-04")
	if err != nil {
		t.Fatal(err)
	}
	if len(hm) != 1 || hm[0].Count != 2 {
		t.Errorf("heatmap = %+v", hm)
	}
}

func TestServiceGetAllTags(t *testing.T) {
	svc, dir := setupSvc(t)
	dp := filepath.Join(dir, "2026-04")
	os.MkdirAll(dp, 0o755)
	os.WriteFile(filepath.Join(dp, "20260415-100000.md"), []byte("#Go #Tech"), 0o644)
	os.WriteFile(filepath.Join(dp, "20260415-110000.md"), []byte("#Go #Rust"), 0o644)
	os.WriteFile(filepath.Join(dp, "20260416-100000.md"), []byte("#Tech"), 0o644)

	tags, err := svc.GetAllTags(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 3 {
		t.Errorf("got %d tags, want 3", len(tags))
	}
	if tags[0].Count < tags[len(tags)-1].Count {
		t.Error("should be sorted desc")
	}
}

func TestServiceValidation(t *testing.T) {
	svc, _ := setupSvc(t)
	ctx := context.Background()

	if _, err := svc.Get(ctx, ""); err == nil {
		t.Error("Get('') should fail")
	}
	if _, err := svc.Update(ctx, "", "x"); err == nil {
		t.Error("Update('','x') should fail")
	}
	if _, err := svc.Update(ctx, "x", ""); err == nil {
		t.Error("Update('x','') should fail")
	}
	if err := svc.Delete(ctx, ""); err == nil {
		t.Error("Delete('') should fail")
	}
	if _, err := svc.ListByMonth(ctx, "bad"); err == nil {
		t.Error("ListByMonth('bad') should fail")
	}
	if _, err := svc.GetHeatmap(ctx, "bad"); err == nil {
		t.Error("GetHeatmap('bad') should fail")
	}
}
