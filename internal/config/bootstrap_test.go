package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoadBootstrap(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "mydata")
	os.MkdirAll(dataDir, 0o755)

	bc := &BootstrapConfig{DataDir: dataDir}
	if err := SaveBootstrap(tmpDir, bc); err != nil {
		t.Fatalf("SaveBootstrap() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(filepath.Join(tmpDir, bootstrapFileName)); err != nil {
		t.Fatalf("tfo.json not created: %v", err)
	}
}

func TestDefaultDataDir(t *testing.T) {
	dir := DefaultDataDir()
	if dir == "" {
		t.Error("DefaultDataDir() should not be empty")
	}
}

func TestInitDataDir(t *testing.T) {
	tmpDir := filepath.Join(t.TempDir(), "newdata")
	if err := InitDataDir(tmpDir); err != nil {
		t.Fatalf("InitDataDir() error = %v", err)
	}

	// .config.json should exist
	if _, err := os.Stat(filepath.Join(tmpDir, configFileName)); err != nil {
		t.Errorf(".config.json should be created: %v", err)
	}
}
