package danmaku

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

func ResolveRoomID(roomID int64) (int64, error) {
	url := fmt.Sprintf("https://api.live.bilibili.com/room/v1/Room/room_init?id=%d", roomID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	resp, err := httpClient.Do(req)
	if err != nil {
		return roomID, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var r struct {
		Data struct {
			RoomID int64 `json:"room_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return roomID, fmt.Errorf("parse: %w", err)
	}
	if r.Data.RoomID == 0 {
		return roomID, fmt.Errorf("room not found")
	}
	return r.Data.RoomID, nil
}

// getToken 从 getConf API 获取 WS 鉴权 token。
func getToken(roomID int64) string {
	url := fmt.Sprintf("https://api.live.bilibili.com/room/v1/Danmu/getConf?room_id=%d&platform=pc&player=web", roomID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	req.Header.Set("Referer", "https://live.bilibili.com/")
	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("[getToken] HTTP fail: %v", err)
		return ""
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var r struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		log.Printf("[getToken] parse fail: %v body=%s", err, string(body))
		return ""
	}
	log.Printf("[getToken] OK: %s", r.Data.Token)
	return r.Data.Token
}
