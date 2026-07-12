package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Config 应用配置。
type Config struct {
	Theme          string   `json:"theme"`           // "dark" | "light"
	RoomID         int64    `json:"room_id"`          // 直播间房间号
	TimeoutMinutes int      `json:"timeout_minutes"`  // 超时分钟数
	WindowOpacity  float64  `json:"window_opacity"`   // 0.5 ~ 1.0
	HelpTypes      []string `json:"help_types"`       // 帮类型: ["排队","帮帮","带带"]
	Servers        []string `json:"servers"`          // 服务器: ["B服","官服"]
}

var defaultConfig = Config{
	Theme:          "dark",
	RoomID:         1926788042,
	TimeoutMinutes: 5,
	WindowOpacity:  0.92,
	HelpTypes:      []string{"排队", "帮帮", "带带", "求带", "上车"},
	Servers:        []string{"B服", "官服"},
}

var (
	cfg     Config
	cfgMu   sync.RWMutex
	cfgPath string
)

// loadConfig 从 JSON 文件加载配置，不存在则创建默认。
func loadConfig() error {
	cfgMu.Lock()
	defer cfgMu.Unlock()

	cfg = defaultConfig
	cfgPath = filepath.Join(exeDir(), "config.json")

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			return saveConfigLocked()
		}
		return err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}
	return nil
}

// saveConfig 持久化配置。
func saveConfig() error {
	cfgMu.Lock()
	defer cfgMu.Unlock()
	return saveConfigLocked()
}

func saveConfigLocked() error {
	cfg.RoomID = clampRoomID(cfg.RoomID)
	cfg.TimeoutMinutes = clamp(cfg.TimeoutMinutes, 1, 60)
	cfg.WindowOpacity = clampFloat(cfg.WindowOpacity, 0.5, 1.0)

	data, _ := json.MarshalIndent(cfg, "", "  ")
	return os.WriteFile(cfgPath, data, 0644)
}

// getConfig 返回配置副本。
func getConfig() Config {
	cfgMu.RLock()
	defer cfgMu.RUnlock()
	return cfg
}

func updateConfig(fn func(*Config)) error {
	cfgMu.Lock()
	fn(&cfg)
	cfgMu.Unlock()
	return saveConfig()
}

func exeDir() string {
	p, _ := os.Executable()
	return filepath.Dir(p)
}

func clampRoomID(id int64) int64 {
	if id < 1 {
		return 1
	}
	return id
}
func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
