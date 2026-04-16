package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// BootstrapConfig 内部引导配置，保存在 tfo.json 中，仅包含数据目录路径。
type BootstrapConfig struct {
	DataDir string `json:"dataDir"`
}

const bootstrapFileName = "tfo.json"

// BootstrapSearchPaths 返回按优先级排序的 tfo.json 搜索路径列表。
// 优先当前可执行文件目录，其次为平台惯用配置目录。
func BootstrapSearchPaths() []string {
	var paths []string

	// 1. 可执行文件所在目录
	if exe, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Dir(exe))
	}

	// 2. 当前工作目录
	if cwd, err := os.Getwd(); err == nil {
		paths = append(paths, cwd)
	}

	// 3. 平台惯用配置目录
	if home, err := os.UserHomeDir(); err == nil {
		switch runtime.GOOS {
		case "darwin":
			paths = append(paths, filepath.Join(home, "Library", "Application Support", "TFOApp"))
		case "windows":
			if appData := os.Getenv("APPDATA"); appData != "" {
				paths = append(paths, filepath.Join(appData, "TFOApp"))
			}
		default:
			paths = append(paths, filepath.Join(home, ".config", "TFOApp"))
		}
	}

	// 去重
	seen := make(map[string]bool)
	var unique []string
	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			abs = p
		}
		if !seen[abs] {
			seen[abs] = true
			unique = append(unique, abs)
		}
	}
	return unique
}

// LoadBootstrap 按优先级搜索并加载 tfo.json。
// 返回 BootstrapConfig 和 tfo.json 所在目录。
// 如果未找到有效的 tfo.json 或其中 dataDir 指向的目录不存在，返回 ErrBootstrapNotFound。
func LoadBootstrap() (*BootstrapConfig, string, error) {
	// 优先使用环境变量
	if dir := os.Getenv("TFO_DATA_DIR"); dir != "" {
		return &BootstrapConfig{DataDir: dir}, "", nil
	}

	for _, searchDir := range BootstrapSearchPaths() {
		p := filepath.Join(searchDir, bootstrapFileName)
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}

		var bc BootstrapConfig
		if err := json.Unmarshal(data, &bc); err != nil {
			continue
		}

		if bc.DataDir == "" {
			continue
		}

		// 将相对路径转为基于 tfo.json 所在目录的绝对路径
		if !filepath.IsAbs(bc.DataDir) {
			bc.DataDir = filepath.Join(searchDir, bc.DataDir)
		}

		// 检查数据目录是否存在
		if info, err := os.Stat(bc.DataDir); err == nil && info.IsDir() {
			return &bc, searchDir, nil
		}
	}

	return nil, "", ErrBootstrapNotFound
}

// ErrBootstrapNotFound 表示未找到有效的 tfo.json。
var ErrBootstrapNotFound = fmt.Errorf("tfo.json not found or dataDir invalid")

// SaveBootstrap 将引导配置写入指定目录下的 tfo.json。
func SaveBootstrap(dir string, bc *BootstrapConfig) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create bootstrap dir: %w", err)
	}

	data, err := json.MarshalIndent(bc, "", "  ")
	if err != nil {
		return err
	}

	p := filepath.Join(dir, bootstrapFileName)
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// DefaultDataDir 返回当前平台的默认数据目录。
func DefaultDataDir() string {
	switch runtime.GOOS {
	case "darwin":
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, "Documents", "TFOApp")
		}
	case "windows":
		// Windows: 使用可执行文件同级目录下的 TFO_Data
		if exe, err := os.Executable(); err == nil {
			return filepath.Join(filepath.Dir(exe), "TFO_Data")
		}
		return filepath.Join(".", "TFO_Data")
	}
	// Linux fallback
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, "Documents", "TFOApp")
	}
	return filepath.Join(".", "TFO_Data")
}

// DefaultBootstrapDir 返回 tfo.json 应该保存的默认目录。
func DefaultBootstrapDir() string {
	switch runtime.GOOS {
	case "darwin":
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, "Library", "Application Support", "TFOApp")
		}
	case "windows":
		if exe, err := os.Executable(); err == nil {
			return filepath.Dir(exe)
		}
		return "."
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".config", "TFOApp")
	}
	return "."
}

// InitDataDir 初始化数据目录：创建目录并写入默认用户配置。
func InitDataDir(dataDir string) error {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	cfg := DefaultConfig()
	cfg.DataDir = dataDir
	return cfg.Save()
}
