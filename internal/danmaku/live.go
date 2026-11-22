package danmaku

import "sync"

var liveStatus = struct {
	live  bool
	known bool
	mu    sync.RWMutex
}{}

func SetLive(v bool) {
	liveStatus.mu.Lock()
	liveStatus.live, liveStatus.known = v, true
	liveStatus.mu.Unlock()
}

func LiveStatus() (bool, bool) {
	liveStatus.mu.RLock()
	defer liveStatus.mu.RUnlock()
	return liveStatus.live, liveStatus.known
}

func IsRoomLive() bool { live, _ := LiveStatus(); return live }
