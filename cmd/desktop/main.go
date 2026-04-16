// Package main provides the desktop (tray/menubar) entry point for TFO.
// On Windows/Linux it shows a system-tray icon; on macOS it falls back to
// headless server + auto-open browser.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/libi/tfo/internal/app"
	"github.com/libi/tfo/internal/server"
)

// appState holds the running server state so that platform-specific tray code
// can restart or stop it.
type appState struct {
	mu     sync.Mutex
	app    *app.App
	cancel context.CancelFunc
	addr   string
}

// Addr returns the current server listen address.
func (a *appState) Addr() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.addr
}

// startServer boots the TFO app + HTTP gateway in the background.
func (a *appState) startServer() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	application := app.New()

	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel

	application.Startup(ctx)
	a.app = application

	addr := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}
	a.addr = addr

	router := server.New(application.ServerDependencies(nil))

	go func() {
		if err := server.Serve(ctx, addr, router); err != nil {
			slog.Error("server error", "error", err)
		}
	}()

	return nil
}

// stopServer gracefully shuts down the running server.
func (a *appState) stopServer() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.app != nil {
		a.app.Shutdown(context.Background())
		a.app = nil
	}
	if a.cancel != nil {
		a.cancel()
		a.cancel = nil
	}
}

// restartServer stops then starts the server again.
func (a *appState) restartServer() error {
	a.stopServer()
	return a.startServer()
}

// DashboardURL returns the full dashboard URL.
func (a *appState) DashboardURL() string {
	return dashboardURL(a.Addr())
}

func main() {
	state := &appState{}

	if err := state.startServer(); err != nil {
		slog.Error("failed to start TFO", "error", err)
		os.Exit(1)
	}

	slog.Info("TFO server started", "addr", state.Addr())

	// Platform-specific tray / menubar integration.
	runDesktop(state)
}

// fallbackWaitForExit blocks until SIGINT/SIGTERM is received.
func fallbackWaitForExit(state *appState) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	slog.Info("shutting down")
	state.stopServer()
}
