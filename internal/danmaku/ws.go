package danmaku

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/gorilla/websocket"
)

func getBuvid(roomID int64) string {
	api := fmt.Sprintf("https://api.live.bilibili.com/room/v1/Room/room_init?id=%d", roomID)
	req, _ := http.NewRequest("GET", api, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Sprintf("%d-infoc", time.Now().UnixNano())
	}
	defer resp.Body.Close()
	for _, c := range resp.Cookies() {
		if c.Name == "buvid3" && c.Value != "" {
			return c.Value
		}
	}
	return fmt.Sprintf("%d-infoc", time.Now().UnixNano())
}

func connectWS(roomID int64) (<-chan DanmakuMsg, error) {
	realID, err := ResolveRoomID(roomID)
	if err != nil {
		return nil, err
	}
	token := getToken(realID)
	buvid := getBuvid(roomID)

	conn, _, err := websocket.DefaultDialer.Dial("wss://broadcastlv.chat.bilibili.com/sub", http.Header{
		"User-Agent": {"Mozilla/5.0 (Windows NT 10.0; Win64; x64)"},
	})
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}

	auth := map[string]any{
		"uid": 0, "roomid": realID, "protover": 3,
		"buvid": buvid, "support_ack": true, "scene": "room",
		"platform": "web", "type": 2, "key": token,
	}
	ab, _ := json.Marshal(auth)
	if err := conn.WriteMessage(websocket.BinaryMessage, packWS(7, ab)); err != nil {
		conn.Close()
		return nil, fmt.Errorf("auth write: %w", err)
	}
	log.Printf("ws: auth sent room=%d token=%s", realID, token)

	go wsPing(conn)

	msgCh := make(chan DanmakuMsg, 512)
	SetLive(true)
	go wsReadLoop(conn, msgCh)
	return msgCh, nil
}

func wsPing(conn *websocket.Conn) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		conn.WriteMessage(websocket.BinaryMessage, packWS(2, []byte{}))
	}
}

func wsReadLoop(conn *websocket.Conn, msgCh chan DanmakuMsg) {
	defer conn.Close()
	defer close(msgCh)
	defer SetLive(false)

	msgCount := 0
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			log.Printf("ws: read err=%v", err)
			return
		}
		if len(data) < 16 {
			continue
		}
		op := binary.BigEndian.Uint32(data[8:12])
		ver := binary.BigEndian.Uint16(data[6:8])
		body := data[16:]
		msgCount++

		if msgCount <= 8 {
			log.Printf("ws: msg#%d op=%d ver=%d bodyLen=%d", msgCount, op, ver, len(body))
		}

		switch op {
		case 5:
			payload := body
			if ver == 3 {
				payload = brotliDecompress(body)
				if msgCount <= 5 {
					preview := string(payload)
					if len(preview) > 200 {
						preview = preview[:200]
					}
					log.Printf("ws: msg#%d decomp[0:200]=%s", msgCount, preview)
				}
			} else if msgCount <= 3 {
				preview := string(body)
				if len(preview) > 200 {
					preview = preview[:200]
				}
				log.Printf("ws: msg#%d body[0:200]=%s", msgCount, preview)
			}
			parsePayload(payload, msgCh, msgCount <= 8)
		case 8:
			var resp struct{ Code int `json:"code"` }
			json.Unmarshal(body, &resp)
			log.Printf("ws: auth reply code=%d", resp.Code)
		}
	}
}

func parsePayload(payload []byte, msgCh chan DanmakuMsg, debug bool) {
	for offset := 0; offset+16 <= len(payload); {
		packLen := int(binary.BigEndian.Uint32(payload[offset : offset+4]))
		if packLen < 16 || offset+packLen > len(payload) {
			// 没子包头，直接当 JSON 解析
			parseMessages(payload[offset:], msgCh, debug)
			return
		}
		subBody := payload[offset+16 : offset+packLen]
		parseMessages(subBody, msgCh, debug)
		offset += packLen
	}
}

func parseMessages(data []byte, msgCh chan DanmakuMsg, debug bool) int {
	count := 0
	for _, line := range bytes.Split(data, []byte{'\n'}) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var raw struct {
			Cmd  string          `json:"cmd"`
			Info []any           `json:"info"`
			Data json.RawMessage `json:"data"`
		}
		if json.Unmarshal(line, &raw) != nil {
			if debug && len(line) > 2 {
				preview := string(line)
				if len(preview) > 60 {
					preview = preview[:60]
				}
				log.Printf("ws: unparseable: %s", preview)
			}
			continue
		}

		switch raw.Cmd {
		case "DANMU_MSG":
			dm := parseWSdanmu(raw.Info)
			if dm.Content != "" {
				select {
				case msgCh <- dm:
					count++
				default:
				}
			}
		case "SEND_GIFT", "GUARD_BUY", "COMBO_SEND":
			gift := parseWSgift(raw.Data)
			if gift.GiftName != "" {
				log.Printf("ws: GIFT! %s %s x%d", gift.Username, gift.GiftName, gift.GiftNum)
				select {
				case msgCh <- gift:
					count++
				default:
				}
			}
		case "LIVE":
			SetLive(true)
		case "PREPARING":
			SetLive(false)
		}
	}
	return count
}

func parseWSdanmu(info []any) DanmakuMsg {
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

func parseWSgift(raw json.RawMessage) DanmakuMsg {
	var gd struct {
		Uname    string `json:"uname"`
		GiftName string `json:"giftName"`
		Num      int    `json:"num"`
		UID      int64  `json:"uid"`
	}
	if json.Unmarshal(raw, &gd) != nil {
		var str string
		if json.Unmarshal(raw, &str) == nil {
			json.Unmarshal([]byte(str), &gd)
		}
	}
	if gd.GiftName == "" || gd.Uname == "" {
		return DanmakuMsg{}
	}
	if gd.Num < 1 {
		gd.Num = 1
	}
	return DanmakuMsg{
		UID: gd.UID, Username: gd.Uname,
		Content:     fmt.Sprintf("送出 %s x%d", gd.GiftName, gd.Num),
		FromCurrent: true, IsGift: true, GiftName: gd.GiftName, GiftNum: gd.Num,
	}
}

func packWS(op uint32, body []byte) []byte {
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
