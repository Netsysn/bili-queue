package danmaku

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/andybalholm/brotli"
	"github.com/gorilla/websocket"
)

// connectWS 直接连接 B站 WebSocket，发送鉴权包，返回消息通道。
func connectWS(roomID int64, buvid string) (<-chan DanmakuMsg, error) {
	realID, err := ResolveRoomID(roomID)
	if err != nil {
		return nil, err
	}
	token := getToken(realID)

	conn, _, err := websocket.DefaultDialer.Dial("wss://broadcastlv.chat.bilibili.com/sub", http.Header{
		"User-Agent": {"Mozilla/5.0 (Windows NT 10.0; Win64; x64)"},
	})
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}

	// 鉴权包（与 Bilibili_Danmuji 格式一致）
	auth := map[string]any{
		"uid":         0,
		"roomid":      realID,
		"protover":    3,
		"buvid":       buvid,
		"support_ack": true,
		"scene":       "room",
		"platform":    "web",
		"type":        2,
		"key":         token,
	}
	body, _ := json.Marshal(auth)
	pkt := packMsg(7, body)
	if err := conn.WriteMessage(websocket.BinaryMessage, pkt); err != nil {
		conn.Close()
		return nil, fmt.Errorf("auth write: %w", err)
	}
	log.Printf("ws: sent auth to room %d", realID)

	msgCh := make(chan DanmakuMsg, 512)
	SetLive(true)
	go wsReadLoop(conn, msgCh)
	return msgCh, nil
}

func wsReadLoop(conn *websocket.Conn, msgCh chan DanmakuMsg) {
	defer conn.Close()
	defer close(msgCh)

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			SetLive(false)
			return
		}
		if len(data) < 16 {
			continue
		}
		op := binary.BigEndian.Uint32(data[8:12])
		ver := binary.BigEndian.Uint16(data[6:8])
		body := data[16:]

		if op == 5 {
			// 解压
			var jsonData []byte
			switch ver {
			case 0:
				jsonData = body
			case 2:
				jsonData = zlibDecompress(body)
			case 3:
				jsonData = brotliDecompress(body)
			default:
				continue
			}
			parseMessages(jsonData, msgCh)
		} else if op == 8 {
			// Auth reply
			var resp struct{ Code int `json:"code"` }
			json.Unmarshal(body, &resp)
			if resp.Code != 0 {
				log.Printf("ws: auth rejected code=%d", resp.Code)
			}
		}
	}
}

func parseMessages(data []byte, msgCh chan DanmakuMsg) {
	for _, line := range bytes.Split(data, []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var raw struct {
			Cmd  string          `json:"cmd"`
			Info []any           `json:"info"`
			Data json.RawMessage `json:"data"`
		}
		if json.Unmarshal(line, &raw) != nil {
			continue
		}

		switch raw.Cmd {
		case "DANMU_MSG":
			dm := parseDanmu(raw.Info)
			if dm.Content != "" {
				select {
				case msgCh <- dm:
				default:
				}
			}
		case "SEND_GIFT":
			gift := parseGift(raw.Data)
			if gift.GiftName != "" {
				select {
				case msgCh <- gift:
				default:
				}
			}
		case "LIVE":
			SetLive(true)
		case "PREPARING":
			SetLive(false)
		case "GUARD_BUY", "COMBO_SEND":
			// 上舰/连击礼物也解析
			gift := parseGift(raw.Data)
			if gift.GiftName != "" {
				select {
				case msgCh <- gift:
				default:
				}
			}
		}
	}
}

func parseDanmu(info []any) DanmakuMsg {
	if len(info) < 3 {
		return DanmakuMsg{}
	}
	content, _ := info[1].(string)
	if content == "" {
		return DanmakuMsg{}
	}

	var uid int64
	var username string
	if arr, ok := info[2].([]any); ok && len(arr) >= 2 {
		if v, ok := arr[0].(float64); ok {
			uid = int64(v)
		}
		if v, ok := arr[1].(string); ok {
			username = v
		}
	}
	if uid == 0 || strings.Contains(username, "***") {
		username = "匿名用户"
	}

	// 勋章
	var mn string
	var ml int
	if len(info) > 3 {
		if arr, ok := info[3].([]any); ok && len(arr) >= 2 {
			if v, ok := arr[1].(string); ok {
				mn = v
			}
			if v, ok := arr[0].(float64); ok {
				ml = int(v)
			}
		}
	}
	// 用户等级
	var ul int
	if len(info) > 4 {
		if arr, ok := info[4].([]any); ok && len(arr) > 0 {
			if v, ok := arr[0].(float64); ok {
				ul = int(v)
			}
		}
	}

	return DanmakuMsg{
		UID: uid, Username: username, Content: content, FromCurrent: true,
		MedalName: mn, MedalLevel: ml, UserLevel: ul,
	}
}

func parseGift(data json.RawMessage) DanmakuMsg {
	// data 可能是对象或 JSON 字符串
	var gd struct {
		Uname    string `json:"uname"`
		GiftName string `json:"giftName"`
		Num      int    `json:"num"`
		UID      int64  `json:"uid"`
		Action   string `json:"action"`
	}
	if json.Unmarshal(data, &gd) != nil {
		var str string
		if json.Unmarshal(data, &str) == nil {
			json.Unmarshal([]byte(str), &gd)
		}
	}
	if gd.GiftName == "" {
		return DanmakuMsg{}
	}
	if gd.Uname == "" {
		gd.Uname = "匿名"
	}
	if gd.Num < 1 {
		gd.Num = 1
	}
	return DanmakuMsg{
		UID: gd.UID, Username: gd.Uname,
		Content:     fmt.Sprintf("送出 %s x%d", gd.GiftName, gd.Num),
		FromCurrent: true, IsGift: true,
		GiftName: gd.GiftName, GiftNum: gd.Num,
	}
}

func packMsg(op uint32, body []byte) []byte {
	pkt := make([]byte, 16+len(body))
	binary.BigEndian.PutUint32(pkt[0:4], uint32(16+len(body)))
	binary.BigEndian.PutUint16(pkt[4:6], 16)
	binary.BigEndian.PutUint16(pkt[6:8], 1)
	binary.BigEndian.PutUint32(pkt[8:12], op)
	binary.BigEndian.PutUint32(pkt[12:16], 1)
	copy(pkt[16:], body)
	return pkt
}

func brotliDecompress(data []byte) []byte {
	r := brotli.NewReader(bytes.NewReader(data))
	var out bytes.Buffer
	out.ReadFrom(r)
	return out.Bytes()
}

func zlibDecompress(data []byte) []byte {
	// 简化 zlib 解压 — 大部分房间用 brotli
	return data
}
