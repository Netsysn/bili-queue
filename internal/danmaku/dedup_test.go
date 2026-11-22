package danmaku

import (
	"testing"
	"time"
)

func TestDeduperExpires(t *testing.T) {
	d := deduper{seen: make(map[messageKey]time.Time)}
	now := time.Now()
	if d.duplicate(1, "排队", now) {
		t.Fatal("first message was duplicate")
	}
	if !d.duplicate(1, "排队", now.Add(time.Second)) {
		t.Fatal("duplicate was not detected")
	}
	if d.duplicate(2, "排队", now.Add(time.Second)) {
		t.Fatal("different user was duplicate")
	}
	if d.duplicate(1, "排队", now.Add(duplicateWindow)) {
		t.Fatal("expired message was duplicate")
	}
}

func TestGiftDedupUsesEventID(t *testing.T) {
	d := deduper{seen: make(map[messageKey]time.Time)}
	now := time.Now()
	msg := DanmakuMsg{UID: 1, IsGift: true, Content: "送出小花 x1", EventID: "ws:one"}
	if d.duplicateMessage(msg, now) || !d.duplicateMessage(msg, now.Add(time.Second)) {
		t.Fatal("same event should be deduplicated")
	}
	msg.EventID = "ws:two"
	if d.duplicateMessage(msg, now.Add(time.Second)) {
		t.Fatal("different event should be retained")
	}
	msg.EventID = ""
	if d.duplicateMessage(msg, now) || d.duplicateMessage(msg, now) {
		t.Fatal("gift without ID must be retained")
	}
}
