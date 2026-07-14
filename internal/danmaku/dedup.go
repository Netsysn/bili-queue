package danmaku

import (
	"crypto/md5"
	"fmt"
	"sync"
)

var (
	seenHashes = make(map[string]bool)
	seenMu     sync.Mutex
)

// IsDuplicate 检查消息是否重复（UID+内容 hash）。返回 true 表示已见过。
func IsDuplicate(uid int64, content string) bool {
	hash := fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%d:%s", uid, content))))
	seenMu.Lock()
	defer seenMu.Unlock()
	if seenHashes[hash] {
		return true
	}
	seenHashes[hash] = true
	if len(seenHashes) > 5000 {
		seenHashes = make(map[string]bool)
	}
	return false
}
