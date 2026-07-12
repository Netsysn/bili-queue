package danmaku

import "sync"

var liveStatus = struct {
	live bool
	mu   sync.RWMutex
}{}

func SetLive(v bool) {
	liveStatus.mu.Lock()
	liveStatus.live = v
	liveStatus.mu.Unlock()
}

func IsRoomLive() bool {
	liveStatus.mu.RLock()
	defer liveStatus.mu.RUnlock()
	return liveStatus.live
}
