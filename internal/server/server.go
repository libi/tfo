package server

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
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
		api.GET("/config", getConfig(deps))
		api.PUT("/config", updateConfig(deps))
		api.GET("/wechat/states", getChannelStates(deps))
		api.POST("/wechat/start", startWeChat(deps))
		api.POST("/wechat/stop", stopWeChat(deps))
		api.GET("/wechat/qrcode", getWeChatQRCode(deps))
	}

	// Serve embedded frontend static files (production mode).
	// In dev mode FrontendFS is nil and frontend runs separately via `npm run dev`.
	if deps.FrontendFS != nil {
		fileServer := http.FileServer(http.FS(deps.FrontendFS))
		r.NoRoute(func(c *gin.Context) {
			path := c.Request.URL.Path
			// Don't serve static files for API routes
			if strings.HasPrefix(path, "/api/") {
				c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
				return
			}
			// Try to serve the exact file first
			f, err := deps.FrontendFS.Open(strings.TrimPrefix(path, "/"))
			if err == nil {
				f.Close()
				fileServer.ServeHTTP(c.Writer, c.Request)
				return
			}
			// Fallback to index.html for client-side routing
			c.Request.URL.Path = "/index.html"
			fileServer.ServeHTTP(c.Writer, c.Request)
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
		c.JSON(http.StatusOK, []interface{}{})
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
		results, total, err := deps.NoteService.Search(c.Request.Context(), q, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"results": results, "total": total})
	}
}

func getConfig(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, deps.Config)
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
		if err := deps.WeChatAdapter.Start(c.Request.Context()); err != nil {
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
		baseURL := deps.Config.WeChat.BaseURL
		if baseURL == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "wechat baseUrl not configured"})
			return
		}
		qr, err := deps.WeChatAdapter.GetQRCode(c.Request.Context(), baseURL)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, qr)
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
