package app

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"github.com/libi/tfo/internal/channel"
	"github.com/libi/tfo/internal/channel/wechat"
	"github.com/libi/tfo/internal/config"
	"github.com/libi/tfo/internal/note"
	"github.com/libi/tfo/internal/search"
	"github.com/libi/tfo/internal/server"
	"github.com/libi/tfo/internal/watcher"
)

// DataDirChooser 是一个回调函数类型，用于在未找到 tfo.json 时让用户选择数据目录。
// 参数 defaultDir 为建议的默认目录，返回用户选择的目录。
// 返回空字符串表示用户取消。
type DataDirChooser func(defaultDir string) (string, error)

// App 是应用的主结构体，管理所有子模块的生命周期
type App struct {
	ctx           context.Context
	config        *config.Config
	noteService   *note.Service
	store         *note.FileStore
	indexer       *search.BleveIndexer
	searcher      *search.BleveSearcher
	watcher       *watcher.Watcher
	receiver      *channel.Receiver
	wechatAdapter *wechat.WeChatAdapter

	// DataDirChooser 可由平台层设置，用于首次运行时让用户选择数据目录。
	// 若为 nil，则直接使用默认数据目录。
	DataDirChooser DataDirChooser
}

// New 创建应用实例
func New() *App {
	return &App{}
}

// Startup 初始化所有后端服务
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx

	dataDir, err := a.resolveDataDir()
	if err != nil {
		log.Fatalf("resolve data dir: %v", err)
	}

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatalf("create data dir: %v", err)
	}

	cfg, err := config.Load(dataDir)
	if err != nil {
		log.Printf("warning: config load failed, using defaults: %v", err)
		cfg = config.DefaultConfig()
		cfg.DataDir = dataDir
	}
	a.config = cfg
	a.store = note.NewFileStore(cfg.DataDir)
	a.noteService = note.NewService(a.store)

	a.initSearch(cfg)
	a.initWatcher(cfg)
	a.initChannel(cfg)

	log.Printf("TFO started, data dir: %s", cfg.DataDir)
}

// Shutdown 应用关闭时调用
func (a *App) Shutdown(ctx context.Context) {
	log.Println("TFO shutting down...")
	if a.receiver != nil {
		if err := a.receiver.StopAll(); err != nil {
			log.Printf("channel stop: %v", err)
		}
	}
	if a.watcher != nil {
		if err := a.watcher.Stop(); err != nil {
			log.Printf("watcher stop: %v", err)
		}
	}
	if a.indexer != nil {
		if err := a.indexer.Close(); err != nil {
			log.Printf("indexer close: %v", err)
		}
	}
}

// ServerDependencies 构建 HTTP 服务器所需的依赖
func (a *App) ServerDependencies(frontendFS fs.FS) *server.Dependencies {
	return &server.Dependencies{
		NoteService:   a.noteService,
		Config:        a.config,
		Receiver:      a.receiver,
		WeChatAdapter: a.wechatAdapter,
		ConfigSaver: func(cfg *config.Config) error {
			return cfg.Save()
		},
		WeChatInit: func() (*channel.Receiver, *wechat.WeChatAdapter) {
			nc := &noteCreatorAdapter{svc: a.noteService}
			r := channel.NewReceiver(nc)
			wa := wechat.NewWeChatAdapter(a.config.WeChat)
			r.RegisterAdapter(wa)
			return r, wa
		},
		FrontendFS: frontendFS,
	}
}

// resolveDataDir 通过 tfo.json 引导确定数据目录。
// 如果未找到有效的 tfo.json，则通过 DataDirChooser（若有）让用户选择，
// 否则使用平台默认目录。最终写入 tfo.json 并初始化数据目录。
func (a *App) resolveDataDir() (string, error) {
	// 1. 尝试加载已有的 tfo.json
	bc, _, err := config.LoadBootstrap()
	if err == nil {
		return bc.DataDir, nil
	}

	// 2. 未找到有效 tfo.json，确定数据目录
	dataDir := config.DefaultDataDir()

	if a.DataDirChooser != nil {
		chosen, err := a.DataDirChooser(dataDir)
		if err != nil {
			return "", fmt.Errorf("choose data dir: %w", err)
		}
		if chosen == "" {
			return "", fmt.Errorf("user cancelled data dir selection")
		}
		dataDir = chosen
	}

	// 3. 初始化数据目录（创建 + 写入默认 .config.json）
	if err := config.InitDataDir(dataDir); err != nil {
		return "", fmt.Errorf("init data dir: %w", err)
	}

	// 4. 保存 tfo.json
	bootstrapDir := config.DefaultBootstrapDir()
	bc = &config.BootstrapConfig{DataDir: dataDir}
	if err := config.SaveBootstrap(bootstrapDir, bc); err != nil {
		log.Printf("warning: save tfo.json to %s failed: %v", bootstrapDir, err)
	}

	return dataDir, nil
}

// GetConfig 获取当前配置
func (a *App) GetConfig() *config.Config {
	return a.config
}

// UpdateConfig 更新并保存配置
func (a *App) UpdateConfig(cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("config cannot be nil")
	}
	cfg.DataDir = a.config.DataDir
	a.config = cfg
	return a.config.Save()
}

// GetDataDir 返回数据目录
func (a *App) GetDataDir() string {
	return a.config.DataDir
}

func (a *App) initSearch(cfg *config.Config) {
	indexPath := filepath.Join(cfg.DataDir, ".index", "bleve.db")
	if err := os.MkdirAll(filepath.Dir(indexPath), 0o755); err != nil {
		log.Printf("warning: create index dir: %v", err)
		return
	}

	a.indexer = search.NewBleveIndexer()

	needsRebuild := cfg.IndexRebuildOnStart
	if !needsRebuild {
		a.indexer.SetIndexPath(indexPath)
		needsRebuild = a.indexer.NeedsRebuild()
	}

	if err := a.indexer.Open(indexPath); err != nil {
		log.Printf("warning: open search index: %v", err)
		return
	}

	if needsRebuild {
		log.Println("rebuilding search index...")
		scanFn := func(ctx context.Context, callback func(*search.IndexDocument) error) error {
			return a.store.ScanAll(ctx, func(n *note.Note) error {
				return callback(&search.IndexDocument{
					ID:        n.ID,
					Title:     n.Title,
					Content:   n.Content,
					Tags:      n.Tags,
					CreatedAt: n.CreatedAt,
				})
			})
		}
		if err := a.indexer.Rebuild(a.ctx, scanFn); err != nil {
			log.Printf("warning: rebuild index: %v", err)
		} else {
			log.Println("search index rebuilt successfully")
		}
	}

	a.searcher = search.NewBleveSearcher(a.indexer.GetIndex())
	a.noteService.SetSearch(a.indexer, a.searcher)
}

type noteCreatorAdapter struct {
	svc *note.Service
}

func (n *noteCreatorAdapter) Create(ctx context.Context, content string) (interface{}, error) {
	return n.svc.Create(ctx, content)
}

func (a *App) initChannel(cfg *config.Config) {
	if !cfg.WeChat.Enabled {
		log.Println("[channel] wechat disabled, skipping")
		return
	}

	a.receiver = channel.NewReceiver(&noteCreatorAdapter{svc: a.noteService})
	a.wechatAdapter = wechat.NewWeChatAdapter(cfg.WeChat)
	a.receiver.RegisterAdapter(a.wechatAdapter)

	if cfg.WeChat.AutoConnect {
		if err := a.receiver.StartAll(a.ctx); err != nil {
			log.Printf("[channel] start: %v", err)
		}
	}
}

// GetChannelStates 返回所有通道适配器的状态
func (a *App) GetChannelStates() map[string]string {
	if a.receiver == nil {
		return map[string]string{}
	}
	raw := a.receiver.GetAdapterStates()
	result := make(map[string]string, len(raw))
	for k, v := range raw {
		result[k] = v.String()
	}
	return result
}

// StartWeChat 启动微信通道
func (a *App) StartWeChat() error {
	if a.wechatAdapter == nil {
		if a.config.WeChat.BaseURL == "" || a.config.WeChat.Token == "" {
			return fmt.Errorf("wechat not configured: baseUrl and token are required")
		}
		a.receiver = channel.NewReceiver(&noteCreatorAdapter{svc: a.noteService})
		a.wechatAdapter = wechat.NewWeChatAdapter(a.config.WeChat)
		a.receiver.RegisterAdapter(a.wechatAdapter)
	}
	return a.wechatAdapter.Start(a.ctx)
}

// StopWeChat 停止微信通道
func (a *App) StopWeChat() error {
	if a.wechatAdapter == nil {
		return nil
	}
	return a.wechatAdapter.Stop()
}

// GetWeChatQRCode 获取微信登录二维码
func (a *App) GetWeChatQRCode() (*wechat.QRCodeResponse, error) {
	if a.wechatAdapter == nil {
		a.wechatAdapter = wechat.NewWeChatAdapter(a.config.WeChat)
	}
	baseURL := a.config.WeChat.BaseURL
	if baseURL == "" {
		return nil, fmt.Errorf("wechat baseUrl not configured")
	}
	return a.wechatAdapter.GetQRCode(a.ctx, baseURL)
}

// LoginWeChatWithQR 使用二维码扫描结果登录微信
func (a *App) LoginWeChatWithQR(result *wechat.QRStatusResponse) error {
	if a.wechatAdapter == nil {
		a.wechatAdapter = wechat.NewWeChatAdapter(a.config.WeChat)
		if a.receiver == nil {
			a.receiver = channel.NewReceiver(&noteCreatorAdapter{svc: a.noteService})
		}
		a.receiver.RegisterAdapter(a.wechatAdapter)
	}
	if err := a.wechatAdapter.LoginWithQRResult(a.ctx, result); err != nil {
		return err
	}
	a.config.WeChat.Enabled = true
	a.config.WeChat.BaseURL = result.BaseURL
	a.config.WeChat.Token = result.BotToken
	return a.config.Save()
}

func (a *App) initWatcher(cfg *config.Config) {
	handler := func(event watcher.FileEvent) {
		if a.indexer == nil {
			return
		}
		switch event.Type {
		case watcher.EventCreated, watcher.EventModified:
			n, err := a.store.LoadByPath(event.FilePath)
			if err != nil {
				log.Printf("[watcher] load %s: %v", event.FilePath, err)
				return
			}
			doc := &search.IndexDocument{
				ID:        n.ID,
				Title:     n.Title,
				Content:   n.Content,
				Tags:      n.Tags,
				CreatedAt: n.CreatedAt,
			}
			if err := a.indexer.Index(doc); err != nil {
				log.Printf("[watcher] index %s: %v", n.ID, err)
			}
		case watcher.EventDeleted:
			id := a.store.PathToID(event.FilePath)
			if id != "" {
				if err := a.indexer.Remove(id); err != nil {
					log.Printf("[watcher] remove index %s: %v", id, err)
				}
			}
		}
	}

	w, err := watcher.New(cfg.DataDir, handler)
	if err != nil {
		log.Printf("warning: create watcher: %v", err)
		return
	}
	a.watcher = w

	go func() {
		if err := w.Start(a.ctx); err != nil && err != context.Canceled {
			log.Printf("watcher stopped: %v", err)
		}
	}()
}
