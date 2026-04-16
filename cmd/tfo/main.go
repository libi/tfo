package main

import (
	"context"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"syscall"

	tfo "github.com/libi/tfo"
	"github.com/libi/tfo/internal/app"
	"github.com/libi/tfo/internal/server"
)

func main() {
	a := app.New()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	a.Startup(ctx)
	defer a.Shutdown(ctx)

	// Prepare embedded frontend. If frontend/out contains index.html,
	// the binary serves the SPA; otherwise only the API is available.
	var frontendFS fs.FS
	sub, err := fs.Sub(tfo.FrontendAssets, "frontend/out")
	if err == nil {
		if _, err := sub.Open("index.html"); err == nil {
			frontendFS = sub
			log.Println("Serving embedded frontend")
		}
	}

	addr := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}

	router := server.New(a.ServerDependencies(frontendFS))
	if err := server.Serve(ctx, addr, router); err != nil {
		log.Fatalf("server: %v", err)
	}
}
