package watcher

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// EventType 文件变更事件类型
type EventType int

const (
	EventCreated  EventType = iota
	EventModified
	EventDeleted
)

const (
	debounceInterval = 100 * time.Millisecond
	indexDirName     = ".index"
)

// FileEvent 文件变更事件
type FileEvent struct {
	Type     EventType
	FilePath string // 文件的绝对路径
}

// EventHandler 事件处理回调
type EventHandler func(event FileEvent)

// Watcher 文件系统监控器
type Watcher struct {
	rootDir   string
	fsWatcher *fsnotify.Watcher
	handler   EventHandler
	cancel    context.CancelFunc
	done      chan struct{}
	mu        sync.Mutex
}

// New 创建监控器
func New(rootDir string, handler EventHandler) (*Watcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &Watcher{
		rootDir:   rootDir,
		fsWatcher: fw,
		handler:   handler,
		done:      make(chan struct{}),
	}, nil
}

// Start 开始监控（阻塞，建议在 goroutine 中运行）
func (w *Watcher) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	w.mu.Lock()
	w.cancel = cancel
	w.mu.Unlock()

	// 添加根目录及所有现有子目录到监控
	if err := w.addWatchDirs(); err != nil {
		cancel()
		return err
	}

	// 去抖定时器表：文件路径 → 定时器
	pending := make(map[string]*time.Timer)
	var pendingMu sync.Mutex

	defer func() {
		pendingMu.Lock()
		for _, timer := range pending {
			timer.Stop()
		}
		pendingMu.Unlock()
		close(w.done)
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return nil
			}
			w.handleFsEvent(event, pending, &pendingMu)
		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return nil
			}
			log.Printf("[watcher] error: %v", err)
		}
	}
}

// Stop 停止监控
func (w *Watcher) Stop() error {
	w.mu.Lock()
	cancel := w.cancel
	w.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	// 等待 Start 退出
	<-w.done
	return w.fsWatcher.Close()
}

// addWatchDirs 递归添加根目录下所有月份子目录
func (w *Watcher) addWatchDirs() error {
	// 监控根目录（用于发现新建的月份目录）
	if err := w.fsWatcher.Add(w.rootDir); err != nil {
		return err
	}

	entries, err := os.ReadDir(w.rootDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == indexDirName || strings.HasPrefix(name, ".") {
			continue
		}
		dirPath := filepath.Join(w.rootDir, name)
		if err := w.fsWatcher.Add(dirPath); err != nil {
			log.Printf("[watcher] add dir %s: %v", dirPath, err)
		}
	}
	return nil
}

// handleFsEvent 处理单个 fsnotify 事件
func (w *Watcher) handleFsEvent(event fsnotify.Event, pending map[string]*time.Timer, mu *sync.Mutex) {
	path := event.Name

	// 如果是新目录创建，自动加入监控
	if event.Has(fsnotify.Create) {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			base := filepath.Base(path)
			if base != indexDirName && !strings.HasPrefix(base, ".") {
				if err := w.fsWatcher.Add(path); err != nil {
					log.Printf("[watcher] add new dir %s: %v", path, err)
				}
			}
			return // 目录事件不触发 handler
		}
	}

	// 只关注 .md 文件
	if filepath.Ext(path) != ".md" {
		return
	}

	// 忽略 .index 目录下的文件
	rel, err := filepath.Rel(w.rootDir, path)
	if err != nil {
		return
	}
	if strings.HasPrefix(rel, indexDirName+string(filepath.Separator)) || strings.HasPrefix(rel, ".") {
		return
	}

	// 确定事件类型
	var evtType EventType
	if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
		evtType = EventDeleted
	} else if event.Has(fsnotify.Create) {
		evtType = EventCreated
	} else if event.Has(fsnotify.Write) {
		evtType = EventModified
	} else {
		return
	}

	// 去抖：同一文件 100ms 内合并事件
	mu.Lock()
	defer mu.Unlock()

	if timer, exists := pending[path]; exists {
		timer.Stop()
	}

	finalType := evtType
	finalPath := path
	pending[path] = time.AfterFunc(debounceInterval, func() {
		mu.Lock()
		delete(pending, finalPath)
		mu.Unlock()
		w.handler(FileEvent{Type: finalType, FilePath: finalPath})
	})
}
