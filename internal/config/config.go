package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config 应用全局配置
type Config struct {
	DataDir             string       `json:"dataDir"`
	HotkeyQuickCapture  string       `json:"hotkeyQuickCapture"`
	WeChat              WeChatConfig `json:"wechat"`
	IndexRebuildOnStart bool         `json:"indexRebuildOnStart"`
}

// WeChatConfig 微信 iLink Bot 连接配置
type WeChatConfig struct {
	Enabled              bool   `json:"enabled"`
	BaseURL              string `json:"baseUrl"`
	Token                string `json:"token"`
	CDNBaseURL           string `json:"cdnBaseUrl,omitempty"`
	AutoConnect          bool   `json:"autoConnect"`
	PollTimeoutSeconds   int    `json:"pollTimeoutSeconds"`
	ReconnectIntervalSec int    `json:"reconnectIntervalSec"`
}

const configFileName = ".config.json"

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		DataDir:            "",
		HotkeyQuickCapture: "Alt+S",
		WeChat: WeChatConfig{
			Enabled:              false,
			BaseURL:              "",
			Token:                "",
			AutoConnect:          true,
			PollTimeoutSeconds:   35,
			ReconnectIntervalSec: 5,
		},
		IndexRebuildOnStart: false,
	}
}

// Load 从 dataDir/.config.json 加载配置。文件不存在则返回默认值。
func Load(dataDir string) (*Config, error) {
	cfg := DefaultConfig()
	cfg.DataDir = dataDir

	configPath := filepath.Join(dataDir, configFileName)
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	cfg.DataDir = dataDir
	return cfg, nil
}

// Save 将配置持久化到 dataDir/.config.json
func (c *Config) Save() error {
	if c.DataDir == "" {
		return os.ErrInvalid
	}

	if err := os.MkdirAll(c.DataDir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	configPath := filepath.Join(c.DataDir, configFileName)
	tmpPath := configPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return err
	}

	return os.Rename(tmpPath, configPath)
}

// ConfigPath 返回配置文件完整路径
func (c *Config) ConfigPath() string {
	return filepath.Join(c.DataDir, configFileName)
}
