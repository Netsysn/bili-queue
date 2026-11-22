package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveConfigPreservesOpacityAndEmptyLists(t *testing.T) {
	oldCfg, oldPath := cfg, cfgPath
	defer func() { cfg, cfgPath = oldCfg, oldPath }()
	cfgPath = filepath.Join(t.TempDir(), "config.json")
	cfg = defaultConfig
	cfg.WindowOpacity = 0.83
	s := &AppService{}
	msg := s.SaveConfig(Config{RoomID: 12, Theme: "dark", TimeoutMinutes: 5, HelpTypes: []string{}, Servers: []string{}, GiftQueue: []string{}})
	if msg != "" {
		t.Fatal(msg)
	}
	if cfg.WindowOpacity != 0.83 || len(cfg.HelpTypes) != 0 || len(cfg.GiftQueue) != 0 {
		t.Fatalf("unexpected saved settings: %+v", cfg)
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil || len(data) == 0 {
		t.Fatal("settings were not written")
	}
}

func TestSaveConfigFailureKeepsPreviousSettings(t *testing.T) {
	oldCfg, oldPath := cfg, cfgPath
	defer func() { cfg, cfgPath = oldCfg, oldPath }()
	cfg = defaultConfig
	cfgPath = filepath.Join(t.TempDir(), "missing", "config.json")
	if msg := (&AppService{}).SaveConfig(Config{RoomID: 12}); msg == "" {
		t.Fatal("expected save error")
	}
	if cfg.RoomID != defaultConfig.RoomID {
		t.Fatal("failed save changed active settings")
	}
}
