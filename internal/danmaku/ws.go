package danmaku

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/gorilla/websocket"
)

func ConnectWS(roomID int64, stop <-chan struct{}) (<-chan DanmakuMsg, error) {
	realID, err := ResolveRoomID(roomID)
	if err != nil {
		return nil, err
	}
	buvid := getBuvid3()
	token, host, wssPort := getDanmuInfo(realID)
	if token == "" {
		token, host, wssPort = getConfInfo(realID)
	}

	// 先试 WSS (443)，失败降级 WS (2244)
	wssURL := fmt.Sprintf("wss://%s:%d/sub", host, wssPort)
	wsURL := fmt.Sprintf("ws://%s:%d/sub", host, wsPortFallback(wssPort))

	log.Printf("ws: dialing %s", wssURL)
	conn, _, err := websocket.DefaultDialer.Dial(wssURL, nil)
	if err != nil {
		log.Printf("ws: WSS failed: %v, trying WS %s", err, wsURL)
		conn, _, err = websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			return nil, fmt.Errorf("websocket dial: %w", err)
		}
	}

	auth := map[string]any{
		"uid": 0, "roomid": realID, "protover": 3,
		"buvid": buvid, "key": token,
		"platform": "danmuji", "type": 2,
	}
	ab, _ := json.Marshal(auth)
	if err := sendWS(conn, 7, ab); err != nil {
		conn.Close()
		return nil, fmt.Errorf("auth: %w", err)
	}
	log.Printf("ws: auth sent room=%d", realID)

	msgCh := make(chan DanmakuMsg, 512)
	go wsLoop(conn, msgCh, stop)
	return msgCh, nil
}

func wsPortFallback(wssPort int) int {
	if wssPort == 443 {
		return 2244
	}
	return wssPort
}

// connectWSOpen 通过身份码连接 B站开放平台（对齐 C# bililive_dm）
func ConnectWSOpen(code string, stop <-chan struct{}) (<-chan DanmakuMsg, int64, string, error) {
	info, err := getRoomInfoByCode(code)
	if err != nil {
		return nil, 0, "", fmt.Errorf("bopen: %w", err)
	}

	log.Printf("bopen: dialing %s", info.Server)
	conn, _, err := websocket.DefaultDialer.Dial(info.Server, nil)
	if err != nil {
		return nil, 0, "", fmt.Errorf("bopen dial: %w", err)
	}

	// auth_body 是 B站返回的预构建认证包，直接发送
	if err := sendWS(conn, 7, []byte(info.Auth)); err != nil {
		conn.Close()
		return nil, 0, "", fmt.Errorf("bopen auth: %w", err)
	}
	log.Printf("bopen: auth sent, room=%d game=%s", info.RoomID, info.GameID)

	msgCh := make(chan DanmakuMsg, 512)
	go wsLoop(conn, msgCh, stop)
	return msgCh, int64(info.RoomID), info.GameID, nil
}

func wsLoop(conn *websocket.Conn, msgCh chan DanmakuMsg, stop <-chan struct{}) {
	wsLoopWithTimeout(conn, msgCh, stop, 20*time.Second, 75*time.Second)
}

func wsLoopWithTimeout(conn *websocket.Conn, msgCh chan DanmakuMsg, stop <-chan struct{}, heartbeatInterval, readTimeout time.Duration) {
	defer conn.Close()
	defer close(msgCh)
	defer log.Printf("[WS] connection closed")

	startTime := time.Now()
	done := make(chan struct{})
	defer close(done)
	go func() {
		ticker := time.NewTicker(heartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
					conn.Close()
					return
				}
				if err := sendWS(conn, 2, []byte("{}")); err != nil {
					conn.Close()
					return
				}
			case <-stop:
				conn.Close()
				return
			case <-done:
				return
			}
		}
	}()

	for {
		if err := conn.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
			return
		}
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Printf("[WS] read error after %v: %v", time.Since(startTime), err)
			return
		}
		if !processFrame(msg, msgCh) {
			return
		}
	}
}

// 一个 WebSocket 帧可能包含多个首尾相连的协议包。
func processFrame(frame []byte, msgCh chan DanmakuMsg) bool {
	for len(frame) >= 16 {
		packLen := int(binary.BigEndian.Uint32(frame[0:4]))
		headerLen := int(binary.BigEndian.Uint16(frame[4:6]))
		if headerLen < 16 || packLen < headerLen || packLen > 1024*1024 || packLen > len(frame) {
			log.Printf("[WS] invalid or truncated packet")
			return true
		}
		ver := binary.BigEndian.Uint16(frame[6:8])
		op := binary.BigEndian.Uint32(frame[8:12])
		body := frame[headerLen:packLen]
		switch op {
		case 5:
			if ver == 3 {
				streamReadBrotli(body, msgCh)
			} else if ver == 2 {
				streamReadZlib(body, msgCh)
			} else {
				parseMessages(body, msgCh)
			}
		case 8:
			var resp struct {
				Code int `json:"code"`
			}
			if err := json.Unmarshal(body, &resp); err != nil || resp.Code != 0 {
				log.Printf("[WS] auth failed or invalid reply")
				return false
			}
		case 3:
			// 心跳回复不代表直播间正在开播。
		}
		frame = frame[packLen:]
	}
	return true

}

func streamReadBrotli(compressed []byte, msgCh chan DanmakuMsg) {
	r := brotli.NewReader(bytes.NewReader(compressed))
	subCount := 0
	for {
		header := make([]byte, 16)
		if _, err := io.ReadFull(r, header); err != nil {
			if subCount > 0 {
				log.Printf("[BROTLI] %d sub-packets from %d compressed bytes", subCount, len(compressed))
			}
			return
		}
		packLen := int(binary.BigEndian.Uint32(header[0:4]))
		if packLen < 16 || packLen > 1024*1024 {
			return
		}
		bodyLen := packLen - 16
		body := make([]byte, bodyLen)
		if _, err := io.ReadFull(r, body); err != nil {
			return
		}
		subCount++
		parseMessages(body, msgCh)
	}
}

func streamReadZlib(compressed []byte, msgCh chan DanmakuMsg) {
	data := compressed
	if len(data) > 2 {
		data = data[2:]
	}
	r, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		r, err = zlib.NewReader(bytes.NewReader(compressed))
		if err != nil {
			log.Printf("[ZLIB] decompress failed: %v (len=%d)", err, len(compressed))
			return
		}
	}
	defer r.Close()
	subCount := 0
	for {
		header := make([]byte, 16)
		if _, err := io.ReadFull(r, header); err != nil {
			if subCount > 0 {
				log.Printf("[ZLIB] %d sub-packets from %d compressed bytes", subCount, len(compressed))
			}
			return
		}
		packLen := int(binary.BigEndian.Uint32(header[0:4]))
		if packLen < 16 || packLen > 1024*1024 {
			return
		}
		bodyLen := packLen - 16
		body := make([]byte, bodyLen)
		if _, err := io.ReadFull(r, body); err != nil {
			return
		}
		subCount++
		parseMessages(body, msgCh)
	}
}

func makePacket(op uint32, body []byte) []byte {
	pkt := make([]byte, 16+len(body))
	binary.BigEndian.PutUint32(pkt[0:4], uint32(16+len(body)))
	binary.BigEndian.PutUint16(pkt[4:6], 16) // header length
	binary.BigEndian.PutUint16(pkt[6:8], 1)  // protocol version
	binary.BigEndian.PutUint32(pkt[8:12], op)
	binary.BigEndian.PutUint32(pkt[12:16], 1) // sequence
	copy(pkt[16:], body)
	return pkt
}

func sendWS(conn *websocket.Conn, op uint32, body []byte) error {
	return conn.WriteMessage(websocket.BinaryMessage, makePacket(op, body))
}

func getBuvid3() string {
	req, _ := http.NewRequest("GET", "https://api.bilibili.com/x/frontend/finger/spi", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Sprintf("%d-infoc", rand.Int63())
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var r struct {
		Data struct {
			B3 string `json:"b_3"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &r) == nil && r.Data.B3 != "" {
		return r.Data.B3
	}
	return fmt.Sprintf("%d-infoc", rand.Int63())
}

func getDanmuInfo(roomID int64) (string, string, int) {
	baseURL := fmt.Sprintf("https://api.live.bilibili.com/xlive/web-room/v1/index/getDanmuInfo?id=%d", roomID)
	wts, wRid := wbiSign(map[string]string{"id": strconv.FormatInt(roomID, 10)})
	if wts != "" {
		baseURL += "&w_rid=" + wRid + "&wts=" + wts
	}
	req, _ := http.NewRequest("GET", baseURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	req.Header.Set("Referer", "https://live.bilibili.com/")
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", "", 0
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var r struct {
		Code int `json:"code"`
		Data struct {
			Token    string `json:"token"`
			HostList []struct {
				Host    string `json:"host"`
				Port    int    `json:"port"`
				WssPort int    `json:"wss_port"`
			} `json:"host_list"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &r) != nil || r.Code != 0 || len(r.Data.HostList) == 0 {
		return "", "", 0
	}
	h := r.Data.HostList[rand.Intn(len(r.Data.HostList))]
	wssPort := h.WssPort
	if wssPort == 0 {
		wssPort = 443
	}
	return r.Data.Token, h.Host, wssPort
}

func getConfInfo(roomID int64) (token, host string, wssPort int) {
	url := fmt.Sprintf("https://api.live.bilibili.com/room/v1/Danmu/getConf?room_id=%d&platform=pc&player=web", roomID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	req.Header.Set("Referer", "https://live.bilibili.com/")
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", "broadcastlv.chat.bilibili.com", 443
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var r struct {
		Data struct {
			Token          string `json:"token"`
			HostServerList []struct {
				Host    string `json:"host"`
				Port    int    `json:"port"`
				WssPort int    `json:"wss_port"`
			} `json:"host_server_list"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &r) != nil || r.Data.Token == "" {
		return "", "broadcastlv.chat.bilibili.com", 443
	}
	if len(r.Data.HostServerList) > 0 {
		h := r.Data.HostServerList[rand.Intn(len(r.Data.HostServerList))]
		wssPort := h.WssPort
		if wssPort == 0 {
			wssPort = 443
		}
		return r.Data.Token, h.Host, wssPort
	}
	return r.Data.Token, "broadcastlv.chat.bilibili.com", 443
}

func parseMessages(data []byte, msgCh chan DanmakuMsg) {
	for _, line := range bytes.Split(data, []byte{'\n'}) {
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
		log.Printf("[CMD] %s", raw.Cmd)
		switch raw.Cmd {
		case "DANMU_MSG":
			dm := parseDM(raw.Info)
			if dm.Content != "" {
				select {
				case msgCh <- dm:
				default:
				}
			}
		case "LIVE_OPEN_PLATFORM_DM":
			dm := parseOpenDM(raw.Data)
			if dm.Content != "" {
				select {
				case msgCh <- dm:
				default:
				}
			}
		case "SEND_GIFT", "GUARD_BUY", "COMBO_SEND":
			gift := parseGift(raw.Data)
			if gift.GiftName != "" {
				log.Printf("[GIFT] %s %s x%d", gift.Username, gift.GiftName, gift.GiftNum)
				select {
				case msgCh <- gift:
				default:
				}
			}
		case "LIVE_OPEN_PLATFORM_SEND_GIFT", "LIVE_OPEN_PLATFORM_GUARD":
			gift := parseOpenGift(raw.Data)
			if gift.GiftName != "" {
				log.Printf("[GIFT] %s %s x%d", gift.Username, gift.GiftName, gift.GiftNum)
				select {
				case msgCh <- gift:
				default:
				}
			}
		case "LIVE":
			SetLive(true)
		case "PREPARING":
			SetLive(false)
		case "LIVE_OPEN_PLATFORM_INTERACTION_END":
			log.Printf("[OPEN] stream ended")
		}
	}
}

func parseDM(info []any) DanmakuMsg {
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
	return DanmakuMsg{UID: uid, Username: username, Content: content, ReplyTo: parseReplyName(info), FromCurrent: true, MedalName: mn, MedalLevel: ml, UserLevel: ul}
}

// B 站将回复对象放在 info[0][15].extra（JSON 字符串）中。
func parseReplyName(info []any) string {
	if len(info) == 0 {
		return ""
	}
	meta, ok := info[0].([]any)
	if !ok || len(meta) <= 15 {
		return ""
	}
	details, ok := meta[15].(map[string]any)
	if !ok {
		return ""
	}
	if extra, ok := details["extra"].(string); ok && extra != "" {
		var reply struct {
			Name string `json:"reply_uname"`
		}
		if json.Unmarshal([]byte(extra), &reply) == nil && strings.TrimSpace(reply.Name) != "" {
			return strings.TrimSpace(reply.Name)
		}
	}
	if name, ok := details["reply_uname"].(string); ok {
		return strings.TrimSpace(name)
	}
	return ""
}

func parseGift(raw json.RawMessage) DanmakuMsg {
	var gd struct {
		Uname    string          `json:"uname"`
		GiftName string          `json:"giftName"`
		Num      int             `json:"num"`
		UID      int64           `json:"uid"`
		TID      json.RawMessage `json:"tid"`
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
	return DanmakuMsg{UID: gd.UID, Username: gd.Uname, Content: fmt.Sprintf("送出 %s x%d", gd.GiftName, gd.Num), FromCurrent: true, IsGift: true, GiftName: gd.GiftName, GiftNum: gd.Num, EventID: giftEventID("ws", eventIDString(gd.TID))}
}

// parseOpenDM 解析开放平台弹幕 LIVE_OPEN_PLATFORM_DM
func parseOpenDM(raw json.RawMessage) DanmakuMsg {
	var d struct {
		UID            int64  `json:"uid"`
		Uname          string `json:"uname"`
		Msg            string `json:"msg"`
		GuardLevel     int    `json:"guard_level"`
		FansMedalLevel int    `json:"fans_medal_level"`
		FansMedalName  string `json:"fans_medal_name"`
	}
	if json.Unmarshal(raw, &d) != nil || d.Msg == "" {
		return DanmakuMsg{}
	}
	return DanmakuMsg{
		UID: d.UID, Username: d.Uname, Content: d.Msg, FromCurrent: true,
		MedalName: d.FansMedalName, MedalLevel: d.FansMedalLevel,
	}
}

// parseOpenGift 解析开放平台礼物 LIVE_OPEN_PLATFORM_SEND_GIFT / LIVE_OPEN_PLATFORM_GUARD
func parseOpenGift(raw json.RawMessage) DanmakuMsg {
	var d struct {
		UID            int64           `json:"uid"`
		Uname          string          `json:"uname"`
		GiftName       string          `json:"gift_name"`
		GiftNum        int             `json:"gift_num"`
		MessageID      json.RawMessage `json:"msg_id"`
		FansMedalLevel int             `json:"fans_medal_level"`
		FansMedalName  string          `json:"fans_medal_name"`
	}
	if json.Unmarshal(raw, &d) != nil || d.GiftName == "" {
		return DanmakuMsg{}
	}
	if d.GiftNum < 1 {
		d.GiftNum = 1
	}
	return DanmakuMsg{
		UID: d.UID, Username: d.Uname,
		Content:     fmt.Sprintf("送出 %s x%d", d.GiftName, d.GiftNum),
		FromCurrent: true, IsGift: true, GiftName: d.GiftName, GiftNum: d.GiftNum,
		EventID:   giftEventID("open", eventIDString(d.MessageID)),
		MedalName: d.FansMedalName, MedalLevel: d.FansMedalLevel,
	}
}

// 平台的事件 ID 可能是字符串或数字，不应因类型变化丢失礼物。
func eventIDString(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var value string
	if json.Unmarshal(raw, &value) == nil {
		return value
	}
	var number json.Number
	if json.Unmarshal(raw, &number) == nil {
		return number.String()
	}
	return ""
}
