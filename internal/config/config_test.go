package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.HotkeyQuickCapture != "Alt+S" {
		t.Errorf("HotkeyQuickCapture = %q, want %q", cfg.HotkeyQuickCapture, "Alt+S")
	}
	if cfg.WeChat.Enabled {
		t.Error("WeChat.Enabled should be false by default")
	}
	if cfg.WeChat.PollTimeoutSeconds != 35 {
		t.Errorf("PollTimeoutSeconds = %d, want 35", cfg.WeChat.PollTimeoutSeconds)
	}
}

func TestLoadNonExistent(t *testing.T) {
	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.HotkeyQuickCapture != "Alt+S" {
		t.Errorf("should return defaults")
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := DefaultConfig()
	cfg.DataDir = tmpDir
	cfg.HotkeyQuickCapture = "Ctrl+Shift+N"
	cfg.WeChat.Token = "test-token"

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// DataDir should not appear in the JSON file (json:"-")
	data, _ := os.ReadFile(filepath.Join(tmpDir, configFileName))
	if strings.Contains(string(data), "dataDir") {
		t.Error("DataDir should not be serialized to .config.json")
	}

	loaded, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.HotkeyQuickCapture != "Ctrl+Shift+N" {
		t.Errorf("HotkeyQuickCapture = %q", loaded.HotkeyQuickCapture)
	}
	if loaded.WeChat.Token != "test-token" {
		t.Errorf("Token = %q", loaded.WeChat.Token)
	}
	// DataDir is set by Load(), not from JSON
	if loaded.DataDir != tmpDir {
		t.Errorf("DataDir = %q, want %q", loaded.DataDir, tmpDir)
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, ".config.json"), []byte("{bad}"), 0o644)
	_, err := Load(tmpDir)
	if err == nil {
		t.Fatal("should error on invalid JSON")
	}
}

func TestSaveAtomicity(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultConfig()
	cfg.DataDir = tmpDir
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	tmpPath := filepath.Join(tmpDir, ".config.json.tmp")
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("tmp file should not remain")
	}

	data, _ := os.ReadFile(filepath.Join(tmpDir, ".config.json"))
	var check map[string]interface{}
	if err := json.Unmarshal(data, &check); err != nil {
		t.Errorf("saved JSON invalid: %v", err)
	}
}

func TestSaveEmptyDataDir(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Save(); err == nil {
		t.Error("should error with empty DataDir")
	}
}
