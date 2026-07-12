package danmaku

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

var (
	avatarCache = make(map[int64]string)
	avatarMu    sync.RWMutex
	httpClient  = &http.Client{Timeout: 5 * time.Second}
)

func GetAvatar(uid int64) string {
	if uid == 0 {
		return ""
	}
	avatarMu.RLock()
	if url, ok := avatarCache[uid]; ok {
		avatarMu.RUnlock()
		return url
	}
	avatarMu.RUnlock()

	url := fetchAvatar(uid)
	if url == "" {
		return ""
	}
	avatarMu.Lock()
	avatarCache[uid] = url
	avatarMu.Unlock()
	return url
}

func fetchAvatar(uid int64) string {
	api := fmt.Sprintf("https://api.bilibili.com/x/web-interface/card?mid=%d", uid)
	req, _ := http.NewRequest("GET", api, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	req.Header.Set("Referer", "https://live.bilibili.com/")
	resp, err := httpClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var r struct {
		Code int `json:"code"`
		Data struct {
			Card struct {
				Face string `json:"face"`
			} `json:"card"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &r); err != nil || r.Code != 0 {
		return ""
	}
	return r.Data.Card.Face
}
