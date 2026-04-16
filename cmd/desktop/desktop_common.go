package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// i18n
// ---------------------------------------------------------------------------

// Lang represents a supported UI language.
type Lang string

const (
	LangEN Lang = "en"
	LangZH Lang = "zh"
)

// currentLang is the detected UI language, set once at startup.
var currentLang = detectSystemLang()

// T returns the localized string for the given key.
func T(key string) string {
	if m, ok := translations[currentLang]; ok {
		if s, ok := m[key]; ok {
			return s
		}
	}
	if s, ok := translations[LangEN][key]; ok {
		return s
	}
	return key
}

var translations = map[Lang]map[string]string{
	LangEN: {
		"placeholder.capture": "Capture a fleeting thought…",
		"button.submit":       "Save",
	},
	LangZH: {
		"placeholder.capture": "输入一些碎片想法…",
		"button.submit":       "保存",
	},
}

// ---------------------------------------------------------------------------
// Server readiness & dashboard URL
// ---------------------------------------------------------------------------

const desktopStartupReadyTimeout = 20 * time.Second

func openDashboardWhenReady(state *appState, timeout time.Duration, autoOpen bool, onReady func(string), onError func(error)) {
	url := state.DashboardURL()
	go func() {
		if err := waitForServerReady(url, timeout); err != nil {
			slog.Warn("dashboard readiness check failed", "url", url, "error", err)
			if onError != nil {
				onError(err)
			}
			return
		}

		slog.Info("dashboard is ready", "url", url)
		if onReady != nil {
			onReady(url)
		}
		if autoOpen {
			if err := openBrowser(url); err != nil {
				slog.Warn("failed to open dashboard", "url", url, "error", err)
			}
		}
	}()
}

func dashboardURL(addr string) string {
	trimmed := strings.TrimSpace(addr)
	if trimmed == "" {
		return "http://127.0.0.1:8080"
	}

	host, port, err := net.SplitHostPort(trimmed)
	if err != nil {
		return "http://" + trimmed
	}

	host = strings.Trim(host, "[]")
	switch host {
	case "", "0.0.0.0", "::":
		host = "127.0.0.1"
	}

	return "http://" + net.JoinHostPort(host, port)
}

func waitForServerReady(baseURL string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client := &http.Client{
		Timeout: 800 * time.Millisecond,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	target := strings.TrimRight(baseURL, "/") + "/"
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		if err := probeServerReady(ctx, client, target); err == nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for %s: %w", target, ctx.Err())
		case <-ticker.C:
		}
	}
}

func probeServerReady(ctx context.Context, client *http.Client, target string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

// submitNote posts a note to the TFO API.
func submitNote(state *appState, content string) error {
	apiURL := strings.TrimRight(state.DashboardURL(), "/") + "/api/notes"
	body, err := json.Marshal(map[string]string{"content": content})
	if err != nil {
		return fmt.Errorf("marshal note: %w", err)
	}
	resp, err := http.Post(apiURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("post note: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}
	return nil
}
