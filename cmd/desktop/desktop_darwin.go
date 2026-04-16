//go:build darwin

package main

import (
	"log/slog"
	"os"
	"os/exec"
	"strings"
)

func openBrowser(url string) error {
	return exec.Command("open", url).Start()
}

func detectSystemLang() Lang {
	for _, key := range []string{"LANGUAGE", "LC_ALL", "LC_MESSAGES", "LANG"} {
		if val := os.Getenv(key); val != "" {
			if strings.HasPrefix(strings.ToLower(val), "zh") {
				return LangZH
			}
		}
	}
	return LangEN
}

// On macOS the recommended approach is a native Swift AppKit wrapper.
// This Go build is a fallback that runs the server headlessly and
// auto-opens the dashboard in the default browser.
func runDesktop(state *appState) {
	go func() {
		url := state.DashboardURL()
		if err := waitForServerReady(url, desktopStartupReadyTimeout); err != nil {
			slog.Warn("dashboard readiness check failed", "url", url, "error", err)
			return
		}
		if err := openBrowser(url); err != nil {
			slog.Warn("failed to open dashboard", "url", url, "error", err)
		}
	}()
	fallbackWaitForExit(state)
}
