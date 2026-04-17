package note

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	monthDirFormat = "2006-01"
	fileNameFormat = "20060102-150405"
	dateFormat     = "2006-01-02"
	fileExtension  = ".md"
)

// Store 定义笔记的持久化存储接口
type Store interface {
	Save(ctx context.Context, content string) (*Note, error)
	Load(ctx context.Context, id string) (*Note, error)
	Update(ctx context.Context, id string, content string) (*Note, error)
	Delete(ctx context.Context, id string) error
	ListByMonth(ctx context.Context, month string) ([]*NoteSummary, error)
	ListByDate(ctx context.Context, date time.Time) ([]*NoteSummary, error)
	ListRecent(ctx context.Context, offset, limit int) ([]*NoteSummary, int, error)
	ScanAll(ctx context.Context, callback func(*Note) error) error
	GetHeatmap(ctx context.Context, month string) ([]CalendarHeatmapEntry, error)
}

// FileStore 基于文件系统的 Store 实现
type FileStore struct {
	rootDir string
	parser  *Parser
}

// NewFileStore 创建文件存储实例
func NewFileStore(rootDir string) *FileStore {
	return &FileStore{rootDir: rootDir, parser: NewParser()}
}

// Save 将内容写入新 .md 文件
func (fs *FileStore) Save(ctx context.Context, content string) (*Note, error) {
	now := time.Now()
	monthDir := now.Format(monthDirFormat)
	baseName := now.Format(fileNameFormat)

	dirPath := filepath.Join(fs.rootDir, monthDir)
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		return nil, fmt.Errorf("create month dir: %w", err)
	}

	fileName := baseName + fileExtension
	filePath := filepath.Join(dirPath, fileName)
	id := baseName

	for i := 1; fileExists(filePath); i++ {
		id = fmt.Sprintf("%s-%d", baseName, i)
		fileName = id + fileExtension
		filePath = filepath.Join(dirPath, fileName)
	}

	if err := atomicWriteFile(filePath, []byte(content)); err != nil {
		return nil, fmt.Errorf("write file: %w", err)
	}

	result := fs.parser.Parse(content)
	return &Note{
		ID:        id,
		Title:     result.Title,
		Content:   content,
		Tags:      result.Tags,
		CreatedAt: now,
		FilePath:  filepath.Join(monthDir, fileName),
	}, nil
}

// Load 根据 ID 加载完整笔记
func (fs *FileStore) Load(ctx context.Context, id string) (*Note, error) {
	absPath, relPath, err := fs.findFileByID(id)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	createdAt, _ := parseTimeFromID(id)
	result := fs.parser.Parse(string(data))
	return &Note{
		ID:        id,
		Title:     result.Title,
		Content:   string(data),
		Tags:      result.Tags,
		CreatedAt: createdAt,
		FilePath:  relPath,
	}, nil
}

// Update 覆盖写入已有笔记
func (fs *FileStore) Update(ctx context.Context, id string, content string) (*Note, error) {
	absPath, relPath, err := fs.findFileByID(id)
	if err != nil {
		return nil, err
	}

	if err := atomicWriteFile(absPath, []byte(content)); err != nil {
		return nil, fmt.Errorf("write file: %w", err)
	}

	createdAt, _ := parseTimeFromID(id)
	result := fs.parser.Parse(content)
	return &Note{
		ID:        id,
		Title:     result.Title,
		Content:   content,
		Tags:      result.Tags,
		CreatedAt: createdAt,
		FilePath:  relPath,
	}, nil
}

// Delete 删除指定笔记文件
func (fs *FileStore) Delete(ctx context.Context, id string) error {
	absPath, _, err := fs.findFileByID(id)
	if err != nil {
		return err
	}
	return os.Remove(absPath)
}

// ListByMonth 列出指定月份目录下所有笔记摘要
func (fs *FileStore) ListByMonth(ctx context.Context, month string) ([]*NoteSummary, error) {
	dirPath := filepath.Join(fs.rootDir, month)
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read month dir: %w", err)
	}

	var summaries []*NoteSummary
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), fileExtension) {
			continue
		}
		s, err := fs.loadSummary(month, entry.Name())
		if err != nil {
			continue
		}
		summaries = append(summaries, s)
	}

	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].CreatedAt.After(summaries[j].CreatedAt)
	})
	return summaries, nil
}

// ListByDate 列出指定日期的笔记摘要
func (fs *FileStore) ListByDate(ctx context.Context, date time.Time) ([]*NoteSummary, error) {
	month := date.Format(monthDirFormat)
	datePrefix := date.Format("20060102")

	dirPath := filepath.Join(fs.rootDir, month)
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read month dir: %w", err)
	}

	var summaries []*NoteSummary
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), fileExtension) {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), fileExtension)
		if !strings.HasPrefix(name, datePrefix) {
			continue
		}
		s, err := fs.loadSummary(month, entry.Name())
		if err != nil {
			continue
		}
		summaries = append(summaries, s)
	}

	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].CreatedAt.After(summaries[j].CreatedAt)
	})
	return summaries, nil
}

// ListRecent 按时间倒序列出所有笔记摘要，支持分页
func (fs *FileStore) ListRecent(ctx context.Context, offset, limit int) ([]*NoteSummary, int, error) {
	entries, err := os.ReadDir(fs.rootDir)
	if err != nil {
		return nil, 0, fmt.Errorf("read root dir: %w", err)
	}

	// Collect month dirs in reverse order (newest first)
	var monthDirs []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := time.ParseInLocation(monthDirFormat, e.Name(), time.Local); err == nil {
			monthDirs = append(monthDirs, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(monthDirs)))

	var all []*NoteSummary
	for _, month := range monthDirs {
		monthSummaries, err := fs.ListByMonth(ctx, month)
		if err != nil {
			continue
		}
		all = append(all, monthSummaries...)
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].CreatedAt.After(all[j].CreatedAt)
	})

	total := len(all)
	if offset >= total {
		return nil, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return all[offset:end], total, nil
}

// ScanAll 全量扫描所有 .md 文件
func (fs *FileStore) ScanAll(ctx context.Context, callback func(*Note) error) error {
	type fileJob struct {
		absPath, relPath string
	}

	var jobs []fileJob
	err := filepath.WalkDir(fs.rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && path != fs.rootDir {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), fileExtension) {
			return nil
		}
		rel, _ := filepath.Rel(fs.rootDir, path)
		jobs = append(jobs, fileJob{absPath: path, relPath: rel})
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk dir: %w", err)
	}

	workers := runtime.NumCPU()
	if workers > len(jobs) {
		workers = len(jobs)
	}
	if workers == 0 {
		return nil
	}

	jobCh := make(chan fileJob, len(jobs))
	for _, j := range jobs {
		jobCh <- j
	}
	close(jobCh)

	var (
		mu      sync.Mutex
		scanErr error
		wg      sync.WaitGroup
	)
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for job := range jobCh {
				mu.Lock()
				if scanErr != nil {
					mu.Unlock()
					return
				}
				mu.Unlock()

				select {
				case <-ctx.Done():
					mu.Lock()
					if scanErr == nil {
						scanErr = ctx.Err()
					}
					mu.Unlock()
					return
				default:
				}

				data, err := os.ReadFile(job.absPath)
				if err != nil {
					continue
				}

				content := string(data)
				id := idFromFileName(filepath.Base(job.relPath))
				createdAt, _ := parseTimeFromID(id)
				result := fs.parser.Parse(content)

				n := &Note{
					ID: id, Title: result.Title, Content: content,
					Tags: result.Tags, CreatedAt: createdAt, FilePath: job.relPath,
				}

				mu.Lock()
				if scanErr == nil {
					if err := callback(n); err != nil {
						scanErr = err
					}
				}
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	return scanErr
}

// GetHeatmap 返回指定月份每天的笔记数量统计
func (fs *FileStore) GetHeatmap(ctx context.Context, month string) ([]CalendarHeatmapEntry, error) {
	dirPath := filepath.Join(fs.rootDir, month)
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read month dir: %w", err)
	}

	counts := make(map[string]int)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), fileExtension) {
			continue
		}
		id := idFromFileName(entry.Name())
		t, err := parseTimeFromID(id)
		if err != nil {
			continue
		}
		counts[t.Format(dateFormat)]++
	}

	heatmap := make([]CalendarHeatmapEntry, 0, len(counts))
	for date, count := range counts {
		heatmap = append(heatmap, CalendarHeatmapEntry{Date: date, Count: count})
	}
	sort.Slice(heatmap, func(i, j int) bool {
		return heatmap[i].Date < heatmap[j].Date
	})
	return heatmap, nil
}

// --- helpers ---

func (fs *FileStore) loadSummary(month, fileName string) (*NoteSummary, error) {
	data, err := os.ReadFile(filepath.Join(fs.rootDir, month, fileName))
	if err != nil {
		return nil, err
	}
	id := idFromFileName(fileName)
	createdAt, _ := parseTimeFromID(id)
	result := fs.parser.Parse(string(data))
	return &NoteSummary{
		ID: id, Title: result.Title, Tags: result.Tags,
		CreatedAt: createdAt, Preview: result.Preview,
	}, nil
}

func (fs *FileStore) findFileByID(id string) (string, string, error) {
	t, err := parseTimeFromID(id)
	if err != nil {
		return "", "", fmt.Errorf("invalid note id %q: %w", id, err)
	}
	month := t.Format(monthDirFormat)
	fileName := id + fileExtension
	relPath := filepath.Join(month, fileName)
	absPath := filepath.Join(fs.rootDir, relPath)
	if !fileExists(absPath) {
		return "", "", fmt.Errorf("note not found: %s", id)
	}
	return absPath, relPath, nil
}

func parseTimeFromID(id string) (time.Time, error) {
	s := id
	if len(s) > 15 {
		s = s[:15]
	}
	return time.ParseInLocation(fileNameFormat, s, time.Local)
}

func idFromFileName(name string) string {
	return strings.TrimSuffix(name, fileExtension)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func atomicWriteFile(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// LoadByPath 通过绝对文件路径加载笔记（供 watcher 使用）
func (fs *FileStore) LoadByPath(absPath string) (*Note, error) {
	id := fs.PathToID(absPath)
	if id == "" {
		return nil, fmt.Errorf("cannot derive id from path: %s", absPath)
	}
	return fs.Load(context.Background(), id)
}

// PathToID 将绝对文件路径转换为笔记 ID (格式: "20250115-100000")
func (fs *FileStore) PathToID(absPath string) string {
	rel, err := filepath.Rel(fs.rootDir, absPath)
	if err != nil {
		return ""
	}
	// rel 形如 "2025-01/20250115-100000.md"
	ext := filepath.Ext(rel)
	if ext != fileExtension {
		return ""
	}
	// 去掉扩展名后取文件名部分（不含月份目录前缀）
	return strings.TrimSuffix(filepath.Base(rel), ext)
}
