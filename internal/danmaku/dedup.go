package danmaku

import (
	"sync"
	"time"
)

// 多来源接收同一条消息时，只在短时间内去重，允许之后重新排队。
const duplicateWindow = 10 * time.Second

type messageKey struct {
	uid     int64
	content string
}
type deduper struct {
	mu   sync.Mutex
	seen map[messageKey]time.Time
}

var messageDeduper = deduper{seen: make(map[messageKey]time.Time)}

func (d *deduper) duplicate(uid int64, content string, now time.Time) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	key := messageKey{uid, content}
	if previous, ok := d.seen[key]; ok && now.Sub(previous) < duplicateWindow {
		return true
	}
	d.seen[key] = now
	if len(d.seen) > 5000 {
		for k, t := range d.seen {
			if now.Sub(t) >= duplicateWindow {
				delete(d.seen, k)
			}
		}
	}
	return false
}

func IsDuplicate(uid int64, content string) bool {
	return messageDeduper.duplicate(uid, content, time.Now())
}
