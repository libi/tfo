package server

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	goqrcode "github.com/skip2/go-qrcode"

	"github.com/libi/tfo/internal/channel"
	"github.com/libi/tfo/internal/channel/wechat"
	"github.com/libi/tfo/internal/config"
	"github.com/libi/tfo/internal/note"
	"github.com/libi/tfo/internal/search"
)

// Dependencies holds all backend services needed by the HTTP server.
type Dependencies struct {
	NoteService   *note.Service
	Config        *config.Config
	Receiver      *channel.Receiver
	WeChatAdapter *wechat.WeChatAdapter
	Indexer       search.Indexer
	Searcher      search.Searcher
	ConfigSaver   func(*config.Config) error
	WeChatInit    func() (*channel.Receiver, *wechat.WeChatAdapter)

	// FrontendFS is the embedded frontend static files (frontend/out/).
	// If nil, no static file serving is configured (dev mode).
	FrontendFS fs.FS
}

// New creates a gin.Engine with all API routes registered.
func New(deps *Dependencies) *gin.Engine {
	r := gin.Default()

	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	api := r.Group("/api")
	{
		api.POST("/notes", createNote(deps))
		api.GET("/notes/:id", getNote(deps))
		api.PUT("/notes/:id", updateNote(deps))
		api.DELETE("/notes/:id", deleteNote(deps))
		api.GET("/notes", listNotes(deps))
		api.GET("/heatmap", getHeatmap(deps))
		api.GET("/tags", getAllTags(deps))
		api.GET("/search", searchNotes(deps))
		api.POST("/search/rebuild", rebuildIndex(deps))
		api.GET("/config", getConfig(deps))
		api.PUT("/config", updateConfig(deps))
		api.PUT("/bootstrap", updateBootstrap(deps))
		api.POST("/browse-directory", browseDirectory())
		api.POST("/request-dir-picker", requestNativeDirPicker(deps))
		api.GET("/wechat/states", getChannelStates(deps))
		api.POST("/wechat/start", startWeChat(deps))
		api.POST("/wechat/stop", stopWeChat(deps))
		api.GET("/wechat/qrcode", getWeChatQRCode(deps))
		api.POST("/wechat/qrcode/poll", pollWeChatQRCode(deps))
		api.POST("/wechat/qrcode/login", loginWithQRCode(deps))
	}

	// Serve embedded frontend static files (production mode).
	// In dev mode FrontendFS is nil and frontend runs separately via `npm run dev`.
	if deps.FrontendFS != nil {
		// serveFile reads a file from the embedded FS and writes it directly to the
		// response. Unlike http.ServeFileFS, this does not depend on the request URL
		// path, so we can serve settings.html when the request path is /settings.
		serveFile := func(c *gin.Context, name string) {
			data, err := fs.ReadFile(deps.FrontendFS, name)
			if err != nil {
				c.Status(http.StatusNotFound)
				return
			}
			contentType := "application/octet-stream"
			switch {
			case strings.HasSuffix(name, ".html"):
				contentType = "text/html; charset=utf-8"
			case strings.HasSuffix(name, ".js"):
				contentType = "application/javascript"
			case strings.HasSuffix(name, ".css"):
				contentType = "text/css"
			case strings.HasSuffix(name, ".json"):
				contentType = "application/json"
			case strings.HasSuffix(name, ".svg"):
				contentType = "image/svg+xml"
			case strings.HasSuffix(name, ".png"):
				contentType = "image/png"
			case strings.HasSuffix(name, ".woff2"):
				contentType = "font/woff2"
			}
			c.Data(http.StatusOK, contentType, data)
		}

		r.NoRoute(func(c *gin.Context) {
			path := c.Request.URL.Path
			// Don't serve static files for API routes
			if strings.HasPrefix(path, "/api/") {
				c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
				return
			}
			trimmedPath := strings.TrimPrefix(path, "/")
			trimmedPath = strings.TrimSuffix(trimmedPath, "/")
			if trimmedPath != "" {
				// Try to serve the exact file first, but avoid handing directories
				// to the static handler because directory/index redirects break SPA routing.
				if info, err := fs.Stat(deps.FrontendFS, trimmedPath); err == nil && !info.IsDir() {
					http.ServeFileFS(c.Writer, c.Request, deps.FrontendFS, trimmedPath)
					return
				}
				// Next.js static export produces e.g. settings.html for /settings routes.
				// Try appending .html so that direct navigation works.
				htmlPath := trimmedPath + ".html"
				if _, err := fs.Stat(deps.FrontendFS, htmlPath); err == nil {
					serveFile(c, htmlPath)
					return
				}
			}
			// Fallback to index.html for client-side routing
			serveFile(c, "index.html")
		})
	}

	return r
}

func createNote(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Content string `json:"content"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		n, err := deps.NoteService.Create(c.Request.Context(), req.Content)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, n)
	}
}

func getNote(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		n, err := deps.NoteService.Get(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, n)
	}
}

func updateNote(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var req struct {
			Content string `json:"content"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		n, err := deps.NoteService.Update(c.Request.Context(), id, req.Content)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, n)
	}
}

func deleteNote(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if err := deps.NoteService.Delete(c.Request.Context(), id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusNoContent, nil)
	}
}

func listNotes(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		date := c.Query("date")
		month := c.Query("month")

		if date != "" {
			notes, err := deps.NoteService.ListByDate(ctx, date)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, notes)
			return
		}
		if month != "" {
			notes, err := deps.NoteService.ListByMonth(ctx, month)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, notes)
			return
		}
		// Default: return recent notes with pagination
		limitStr := c.DefaultQuery("limit", "20")
		limit, _ := strconv.Atoi(limitStr)
		offsetStr := c.DefaultQuery("offset", "0")
		offset, _ := strconv.Atoi(offsetStr)
		notes, total, err := deps.NoteService.ListRecent(ctx, offset, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": notes, "total": total})
	}
}

func getHeatmap(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		month := c.Query("month")
		if month == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "month parameter required (e.g. 2026-04)"})
			return
		}
		data, err := deps.NoteService.GetHeatmap(c.Request.Context(), month)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, data)
	}
}

func getAllTags(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		tags, err := deps.NoteService.GetAllTags(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, tags)
	}
}

func searchNotes(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		q := c.Query("q")
		limitStr := c.DefaultQuery("limit", "20")
		limit, _ := strconv.Atoi(limitStr)
		offsetStr := c.DefaultQuery("offset", "0")
		offset, _ := strconv.Atoi(offsetStr)
		results, total, err := deps.NoteService.Search(c.Request.Context(), q, offset, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"results": results, "total": total})
	}
}

func rebuildIndex(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := deps.NoteService.RebuildIndex(c.Request.Context()); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "index rebuilt successfully"})
	}
}

func getConfig(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// DataDir has json:"-" in Config, so wrap it for the API response
		type configResponse struct {
			DataDir   string `json:"dataDir"`
			Sandboxed bool   `json:"sandboxed"`
			*config.Config
		}
		c.JSON(http.StatusOK, configResponse{
			DataDir:   deps.Config.DataDir,
			Sandboxed: os.Getenv("APP_SANDBOX_CONTAINER_ID") != "",
			Config:    deps.Config,
		})
	}
}

func updateConfig(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		var cfg config.Config
		if err := c.ShouldBindJSON(&cfg); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		cfg.DataDir = deps.Config.DataDir
		*deps.Config = cfg
		if deps.ConfigSaver != nil {
			if err := deps.ConfigSaver(deps.Config); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}
		c.JSON(http.StatusOK, deps.Config)
	}
}

func updateBootstrap(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			DataDir string `json:"dataDir" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		bootstrapDir := config.DefaultBootstrapDir()
		bc := &config.BootstrapConfig{DataDir: req.DataDir}
		if err := config.SaveBootstrap(bootstrapDir, bc); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"dataDir": req.DataDir, "message": "Bootstrap updated. Restart the application to use the new data directory."})
	}
}

func getChannelStates(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Receiver == nil {
			c.JSON(http.StatusOK, map[string]string{})
			return
		}
		raw := deps.Receiver.GetAdapterStates()
		result := make(map[string]string, len(raw))
		for k, v := range raw {
			result[k] = v.String()
		}
		c.JSON(http.StatusOK, result)
	}
}

func startWeChat(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.WeChatAdapter == nil {
			if deps.Config.WeChat.BaseURL == "" || deps.Config.WeChat.Token == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "wechat not configured"})
				return
			}
			if deps.WeChatInit != nil {
				deps.Receiver, deps.WeChatAdapter = deps.WeChatInit()
			}
		}
		if err := deps.WeChatAdapter.Start(context.Background()); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "started"})
	}
}

func stopWeChat(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.WeChatAdapter == nil {
			c.JSON(http.StatusOK, gin.H{"status": "not running"})
			return
		}
		if err := deps.WeChatAdapter.Stop(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "stopped"})
	}
}

func getWeChatQRCode(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.WeChatAdapter == nil {
			deps.WeChatAdapter = wechat.NewWeChatAdapter(deps.Config.WeChat)
		}
		baseURL := c.Query("baseUrl")
		if baseURL == "" {
			baseURL = deps.Config.WeChat.BaseURL
		}
		if baseURL == "" {
			baseURL = "https://ilinkai.weixin.qq.com"
		}
		qr, err := deps.WeChatAdapter.GetQRCode(c.Request.Context(), baseURL)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}

		// Generate QR code image from the URL so frontend can display it directly.
		imgContent := ""
		if qr.QRCodeImgContent != "" {
			png, err := goqrcode.Encode(qr.QRCodeImgContent, goqrcode.Medium, 256)
			if err == nil {
				imgContent = "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"qrcode":           qr.QRCode,
			"qrcodeImgContent": imgContent,
		})
	}
}

func pollWeChatQRCode(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			BaseURL string `json:"baseUrl"`
			QRCode  string `json:"qrcode"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.QRCode == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "qrcode is required"})
			return
		}
		if req.BaseURL == "" {
			req.BaseURL = deps.Config.WeChat.BaseURL
		}
		if req.BaseURL == "" {
			req.BaseURL = "https://ilinkai.weixin.qq.com"
		}

		if deps.WeChatAdapter == nil {
			deps.WeChatAdapter = wechat.NewWeChatAdapter(deps.Config.WeChat)
		}
		status, err := deps.WeChatAdapter.PollQRStatus(c.Request.Context(), req.BaseURL, req.QRCode)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status":   status.Status,
			"botToken": status.BotToken,
			"botId":    status.ILinkBotID,
			"baseUrl":  status.BaseURL,
		})
	}
}

func loginWithQRCode(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			BotToken string `json:"botToken"`
			BotID    string `json:"botId"`
			BaseURL  string `json:"baseUrl"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.BotToken == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "botToken is required"})
			return
		}

		// Update config with the new credentials
		deps.Config.WeChat.Token = req.BotToken
		if req.BaseURL != "" {
			deps.Config.WeChat.BaseURL = req.BaseURL
		}
		deps.Config.WeChat.Enabled = true

		// Persist config
		if deps.ConfigSaver != nil {
			if err := deps.ConfigSaver(deps.Config); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save config: " + err.Error()})
				return
			}
		}

		// Re-init and start adapter
		if deps.WeChatInit != nil {
			deps.Receiver, deps.WeChatAdapter = deps.WeChatInit()
		}
		if deps.WeChatAdapter != nil {
			if err := deps.WeChatAdapter.Start(context.Background()); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start wechat: " + err.Error()})
				return
			}
		}

		c.JSON(http.StatusOK, gin.H{"status": "connected"})
	}
}

// requestNativeDirPicker creates a signal file that the native Swift host app
// watches. When detected, the host shows an NSOpenPanel, saves a security-scoped
// bookmark, and writes the chosen path to a response file. This endpoint polls
// for that response and returns it to the frontend caller.
func requestNativeDirPicker(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		dataDir := deps.Config.DataDir
		if dataDir == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "data dir not configured"})
			return
		}

		reqFile := filepath.Join(dataDir, ".dir-pick-request")
		respFile := filepath.Join(dataDir, ".dir-pick-response")

		// Clean up any stale files
		os.Remove(reqFile)
		os.Remove(respFile)

		// Write request signal
		if err := os.WriteFile(reqFile, []byte("pick"), 0o644); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create request signal"})
			return
		}

		// Poll for response (max 120s — user needs time to interact with the dialog)
		deadline := time.Now().Add(120 * time.Second)
		for time.Now().Before(deadline) {
			data, err := os.ReadFile(respFile)
			if err == nil {
				// Clean up signal files
				os.Remove(reqFile)
				os.Remove(respFile)

				result := strings.TrimSpace(string(data))
				if result == "" || result == "__cancelled__" {
					c.JSON(http.StatusOK, gin.H{"path": "", "cancelled": true})
					return
				}
				c.JSON(http.StatusOK, gin.H{"path": result, "cancelled": false})
				return
			}
			time.Sleep(200 * time.Millisecond)
		}

		// Timed out
		os.Remove(reqFile)
		os.Remove(respFile)
		c.JSON(http.StatusGatewayTimeout, gin.H{"error": "native directory picker timed out"})
	}
}

func browseDirectory() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Path string `json:"path"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			req.Path = ""
		}

		dir := req.Path
		if dir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			dir = home
		}

		// Resolve to absolute path
		absDir, err := filepath.Abs(dir)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path"})
			return
		}
		dir = absDir

		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "path is not a valid directory"})
			return
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		type dirEntry struct {
			Name string `json:"name"`
			Path string `json:"path"`
		}
		dirs := make([]dirEntry, 0)
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				dirs = append(dirs, dirEntry{
					Name: e.Name(),
					Path: filepath.Join(dir, e.Name()),
				})
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			parent = ""
		}

		c.JSON(http.StatusOK, gin.H{
			"current": dir,
			"parent":  parent,
			"dirs":    dirs,
		})
	}
}

// Serve starts the HTTP server and blocks until the context is cancelled.
func Serve(ctx context.Context, addr string, handler http.Handler) error {
	srv := &http.Server{Addr: addr, Handler: handler}
	go func() {
		<-ctx.Done()
		_ = srv.Close()
	}()
	log.Printf("HTTP server listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("http server: %w", err)
	}
	return nil
}
