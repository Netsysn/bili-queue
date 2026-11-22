package main

import (
	"bili-queue-overlay/internal/danmaku"
	"bili-queue-overlay/internal/queue"
	"testing"
)

func TestConfiguredGiftEntersQueue(t *testing.T) {
	oldCfg := cfg
	defer func() { cfg = oldCfg }()
	cfg = defaultConfig
	s := &AppService{manager: queue.New(nil, 5), stopCh: make(chan struct{}), avatarSlots: make(chan struct{}, 1)}
	s.processMsg(danmaku.DanmakuMsg{UID: 837241, Username: "测试观众", Avatar: "provided", FromCurrent: true, IsGift: true, GiftName: "小花花", GiftNum: 1, EventID: "test:flower-1", Content: "送出 小花花 x1"})
	entries := s.manager.List()
	if len(entries) != 1 || entries[0].HelpType != "礼物插队" {
		t.Fatalf("configured gift did not enter queue: %+v", entries)
	}
}
