//go:build darwin

package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	"github.com/libi/tfo/internal/app"
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

// chooseDataDirDarwin 在 macOS 上弹出原生文件夹选择对话框。
func chooseDataDirDarwin(defaultDir string) (string, error) {
	// 确保默认目录的父目录存在，否则 osascript 无法定位
	if err := os.MkdirAll(defaultDir, 0o755); err != nil {
		slog.Warn("failed to create default dir for picker", "dir", defaultDir, "error", err)
	}

	prompt := "Choose a folder to store TFOApp data"
	if currentLang == LangZH {
		prompt = "请选择 TFOApp 数据保存目录"
	}
	script := fmt.Sprintf(`
		set defaultPath to POSIX file "%s"
		try
			set chosenFolder to choose folder with prompt "%s" default location defaultPath
			return POSIX path of chosenFolder
		on error
			return ""
		end try
	`, defaultDir, prompt)

	out, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		return "", fmt.Errorf("osascript: %w", err)
	}

	result := strings.TrimSpace(string(out))
	// osascript 返回的路径末尾带 /
	result = strings.TrimSuffix(result, "/")
	return result, nil
}

// platformDataDirChooser 返回 macOS 平台的数据目录选择器。
func platformDataDirChooser() app.DataDirChooser {
	return chooseDataDirDarwin
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
