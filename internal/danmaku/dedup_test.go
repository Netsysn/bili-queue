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
