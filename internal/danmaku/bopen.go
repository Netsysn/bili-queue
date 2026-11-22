package danmaku

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// BOpenAPI 响应体
type bOpenRoomInfo struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		AnchorInfo struct {
			RoomID int    `json:"room_id"`
			Uface  string `json:"uface"`
			UID    int64  `json:"uid"`
			Uname  string `json:"uname"`
		} `json:"anchor_info"`
		GameInfo struct {
			GameID string `json:"game_id"`
		} `json:"game_info"`
		WebsocketInfo struct {
			AuthBody string   `json:"auth_body"`
			WssLink  []string `json:"wss_link"`
		} `json:"websocket_info"`
	} `json:"data"`
}

type bOpenApiResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type openRoomInfo struct {
	Auth   string
	Server string // 取第一个 WSS URL
	RoomID int
	GameID string
}

const bOpenAppID = "1651388990835" // 来自 bililive_dm

var bOpenSignServers = []string{
	"https://soft.ceve-market.org/bopen/sign",
	"https://bopen.ceve-market.org/sign",
}

// getRoomInfoByCode 通过身份码获取房间信息（对齐 C# BOpen.GetRoomInfoByCode）
func getRoomInfoByCode(code string) (*openRoomInfo, error) {
	if code == "" {
		return nil, fmt.Errorf("未输入身份码")
	}

	param, _ := json.Marshal(map[string]string{"code": code, "app_id": bOpenAppID})

	// 1. 调用签名服务器
	signHeaders, err := bOpenSign(param)
	if err != nil {
		return nil, fmt.Errorf("签名服务器离线: %w", err)
	}

	// 2. 用签名后的 headers 调用 B站开放平台
	req, _ := http.NewRequest("POST", "https://live-open.biliapi.com/v2/app/start",
		bytes.NewReader(param))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	for k, v := range signHeaders {
		req.Header.Set(k, v)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("B站直播中心离线: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var info bOpenRoomInfo
	if json.Unmarshal(body, &info) != nil {
		return nil, fmt.Errorf("B站返回解析失败")
	}
	if info.Code != 0 {
		return nil, fmt.Errorf("B站返回错误: %s (code=%d)", info.Message, info.Code)
	}
	if info.Data.AnchorInfo.RoomID <= 0 || info.Data.WebsocketInfo.AuthBody == "" {
		return nil, fmt.Errorf("B站返回了无效的房间号")
	}
	if len(info.Data.WebsocketInfo.WssLink) == 0 {
		return nil, fmt.Errorf("B站未返回 WebSocket 地址")
	}

	log.Printf("bopen: room=%d game_id=%s", info.Data.AnchorInfo.RoomID, info.Data.GameInfo.GameID)

	return &openRoomInfo{
		Auth:   info.Data.WebsocketInfo.AuthBody,
		Server: info.Data.WebsocketInfo.WssLink[0],
		RoomID: info.Data.AnchorInfo.RoomID,
		GameID: info.Data.GameInfo.GameID,
	}, nil
}

// bOpenSign 调用签名服务器获取签名 headers
func bOpenSign(param []byte) (map[string]string, error) {
	var lastErr error
	for _, url := range bOpenSignServers {
		req, _ := http.NewRequest("POST", url, bytes.NewReader(param))
		req.Header.Set("Content-Type", "application/json")
		resp, err := httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 200 {
			lastErr = fmt.Errorf("sign server returned %d", resp.StatusCode)
			continue
		}
		var headers map[string]string
		if json.Unmarshal(body, &headers) != nil {
			lastErr = fmt.Errorf("sign server returned invalid json")
			continue
		}
		return headers, nil
	}
	return nil, lastErr
}

// bOpenHeartbeat 发送项目心跳（对齐 C# BOpen.HeartBeat）
func bOpenHeartbeat(gameID string) {
	if gameID == "" {
		return
	}
	param, _ := json.Marshal(map[string]string{"game_id": gameID})

	signHeaders, err := bOpenSign(param)
	if err != nil {
		log.Printf("bopen: heartbeat sign failed: %v", err)
		return
	}

	req, _ := http.NewRequest("POST", "https://live-open.biliapi.com/v2/app/heartbeat",
		bytes.NewReader(param))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	for k, v := range signHeaders {
		req.Header.Set(k, v)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("bopen: heartbeat failed: %v", err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var r bOpenApiResponse
	if json.Unmarshal(body, &r) == nil && r.Code == 0 {
		return
	}
	log.Printf("bopen: heartbeat error code=%d msg=%s", r.Code, r.Message)
}

// ResolveIdentityCode 通过身份码获取房间信息（不建立 WebSocket 连接）
func ResolveIdentityCode(code string) (*openRoomInfo, error) {
	return getRoomInfoByCode(code)
}

// StartBOpenHeartbeat 启动项目心跳（每 20 秒一次）
func StartBOpenHeartbeat(gameID string, stopCh <-chan struct{}) {
	go func() {
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				bOpenHeartbeat(gameID)
			case <-stopCh:
				return
			}
		}
	}()
}
