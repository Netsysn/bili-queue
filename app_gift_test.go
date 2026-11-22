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

func TestReplyTargetPreservedInLogsAndDoesNotTriggerQueue(t *testing.T) {
	oldCfg := cfg
	defer func() { cfg = oldCfg }()
	cfg = defaultConfig
	s := &AppService{manager: queue.New(nil, 5), stopCh: make(chan struct{}), avatarSlots: make(chan struct{}, 1)}
	s.processMsg(danmaku.DanmakuMsg{UID: 837242, Username: "观众", Avatar: "provided", FromCurrent: true, ReplyTo: "帮帮", Content: "你好"})
	update := s.buildUpdate()
	if len(update.Queue) != 0 {
		t.Fatal("reply target triggered queue")
	}
	if len(update.Logs) != 1 || update.Logs[0].ReplyTo != "帮帮" {
		t.Fatalf("reply target missing from UI payload: %+v", update.Logs)
	}
}

func TestAnonymousDanmakuIsVisibleButNotQueued(t *testing.T) {
	oldCfg := cfg
	defer func() { cfg = oldCfg }()
	cfg = defaultConfig
	s := &AppService{manager: queue.New(nil, 5), stopCh: make(chan struct{}), avatarSlots: make(chan struct{}, 1)}
	s.processMsg(danmaku.DanmakuMsg{UID: 0, Username: "匿名用户", FromCurrent: true, Content: "排队"})
	got := s.buildUpdate()
	if len(got.Logs) != 1 || got.Logs[0].Content != "排队" {
		t.Fatal("anonymous message was lost")
	}
	if len(got.Queue) != 0 {
		t.Fatal("anonymous viewer must not be automatically queued")
	}
}
