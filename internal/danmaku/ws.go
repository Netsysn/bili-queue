package danmaku

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/andybalholm/brotli"
)

func connectWS(roomID int64) (<-chan DanmakuMsg, error) {
	realID, err := ResolveRoomID(roomID)
	if err != nil {
		return nil, err
	}
	buvid := getBuvid3()
	token, host, port := getDanmuInfo(realID)
	if token == "" {
		token, host, port = getConfInfo(realID)
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}

	auth := map[string]any{
		"uid": 0, "roomid": realID, "protover": 3,
		"buvid": buvid, "key": token,
		"platform": "danmuji", "type": 2,
	}
	ab, _ := json.Marshal(auth)
	if err := sendTCP(conn, 16, 1, 7, 1, ab); err != nil {
		conn.Close()
		return nil, fmt.Errorf("auth: %w", err)
	}
	log.Printf("ws: auth sent room=%d host=%s", realID, addr)

	SetLive(true)
	msgCh := make(chan DanmakuMsg, 512)
	go tcpLoop(conn, msgCh)
	return msgCh, nil
}

func tcpLoop(conn net.Conn, msgCh chan DanmakuMsg) {
	defer conn.Close()
	defer close(msgCh)
	defer SetLive(false)

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			sendTCP(conn, 16, 1, 2, 1, []byte{})
		}
	}()

	for {
		header := make([]byte, 16)
		if _, err := io.ReadFull(conn, header); err != nil {
			return
		}
		packLen := int(binary.BigEndian.Uint32(header[0:4]))
		ver := binary.BigEndian.Uint16(header[6:8])
		op := binary.BigEndian.Uint32(header[8:12])
		bodyLen := packLen - 16
		if bodyLen < 0 || bodyLen > 1024*1024 {
			continue
		}
		body := make([]byte, bodyLen)
		if bodyLen > 0 {
			if _, err := io.ReadFull(conn, body); err != nil {
				return
			}
		}
		switch op {
		case 5:
			payload := body
			if ver == 3 {
				payload = brotliDecompress(body)
			}
			processPayload(payload, msgCh)
		case 8:
			var resp struct{ Code int `json:"code"` }
			json.Unmarshal(body, &resp)
			if resp.Code != 0 {
				log.Printf("ws: auth rejected code=%d", resp.Code)
			}
		case 3:
			SetLive(true)
		}
	}
}

func sendTCP(conn net.Conn, magic, ver uint16, op, seq uint32, body []byte) error {
	pkt := make([]byte, 16+len(body))
	binary.BigEndian.PutUint32(pkt[0:4], uint32(16+len(body)))
	binary.BigEndian.PutUint16(pkt[4:6], magic)
	binary.BigEndian.PutUint16(pkt[6:8], ver)
	binary.BigEndian.PutUint32(pkt[8:12], op)
	binary.BigEndian.PutUint32(pkt[12:16], seq)
	copy(pkt[16:], body)
	_, err := conn.Write(pkt)
	return err
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
		Data struct{ B3 string `json:"b_3"` } `json:"data"`
	}
	if json.Unmarshal(body, &r) == nil && r.Data.B3 != "" {
		return r.Data.B3
	}
	return fmt.Sprintf("%d-infoc", rand.Int63())
}

func getDanmuInfo(roomID int64) (string,string,int) {
	baseURL := fmt.Sprintf("https://api.live.bilibili.com/xlive/web-room/v1/index/getDanmuInfo?id=%d", roomID)
	wts, wRid := wbiSign(map[string]string{"id": strconv.FormatInt(roomID, 10)})
	log.Printf("getDanmuInfo: wbiSign wts=%s wRid=%s key=%s", wts, wRid, wbiKey)
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
			Token string `json:"token"`
			HostList []struct {
				Host string `json:"host"`
				Port int `json:"port"`
			} `json:"host_list"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &r) != nil || r.Code != 0 || len(r.Data.HostList) == 0 {
		log.Printf("getDanmuInfo: code=%d body=%s", r.Code, string(body))
		return "", "", 0
	}
	h := r.Data.HostList[rand.Intn(len(r.Data.HostList))]
	return r.Data.Token, h.Host, h.Port
}

func getConfInfo(roomID int64) (token, host string, port int) {
	url := fmt.Sprintf("https://api.live.bilibili.com/room/v1/Danmu/getConf?room_id=%d&platform=pc&player=web", roomID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	req.Header.Set("Referer", "https://live.bilibili.com/")
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", "broadcastlv.chat.bilibili.com", 2243
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var r struct {
		Data struct {
			Token          string `json:"token"`
			HostServerList []struct {
				Host string `json:"host"`
				Port int    `json:"port"`
			} `json:"host_server_list"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &r) != nil || r.Data.Token == "" {
		return "", "broadcastlv.chat.bilibili.com", 2243
	}
	if len(r.Data.HostServerList) > 0 {
		h := r.Data.HostServerList[rand.Intn(len(r.Data.HostServerList))]
		return r.Data.Token, h.Host, h.Port
	}
	return r.Data.Token, "broadcastlv.chat.bilibili.com", 2243
}

func processPayload(payload []byte, msgCh chan DanmakuMsg) {
	for offset := 0; offset+16 <= len(payload); {
		packLen := int(binary.BigEndian.Uint32(payload[offset : offset+4]))
		if packLen < 16 || offset+packLen > len(payload) {
			// packLen invalid, skip header and try remaining
			if len(payload) > offset+16 {
				parseMessages(payload[offset+16:], msgCh)
			}
			return
		}
		subVer := binary.BigEndian.Uint16(payload[offset+6 : offset+8])
		subData := payload[offset+16 : offset+packLen]
		if subVer == 3 {
			subData = brotliDecompress(subData)
		}
		parseMessages(subData, msgCh)
		offset += packLen
	}
}

func parseSingle(data []byte, msgCh chan DanmakuMsg) {
	var raw struct {
		Cmd  string          `json:"cmd"`
		Info []any           `json:"info"`
		Data json.RawMessage `json:"data"`
	}
	if json.Unmarshal(data, &raw) != nil { return }
	log.Printf("[CMD] %s", raw.Cmd)
	switch raw.Cmd {
	case "DANMU_MSG":
		dm := parseDM(raw.Info)
		if dm.Content != "" { select { case msgCh <- dm: default: } }
	case "SEND_GIFT", "GUARD_BUY", "COMBO_SEND":
		gift := parseGift(raw.Data)
		if gift.GiftName != "" {
			log.Printf("[GIFT] %s %s x%d", gift.Username, gift.GiftName, gift.GiftNum)
			select { case msgCh <- gift: default: }
		}
	case "LIVE": SetLive(true)
	case "PREPARING": SetLive(false)
	}
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
		if raw.Cmd != "ONLINE_RANK_COUNT" && raw.Cmd != "ONLINE_RANK_V2" && raw.Cmd != "WATCHED_CHANGE" {
			log.Printf("[CMD] %s", raw.Cmd)
		}
		switch raw.Cmd {
		case "DANMU_MSG":
			dm := parseDM(raw.Info)
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
		case "LIVE":
			SetLive(true)
		case "PREPARING":
			SetLive(false)
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
	return DanmakuMsg{UID: uid, Username: username, Content: content, FromCurrent: true, MedalName: mn, MedalLevel: ml, UserLevel: ul}
}

func parseGift(raw json.RawMessage) DanmakuMsg {
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
	return DanmakuMsg{UID: gd.UID, Username: gd.Uname, Content: fmt.Sprintf("送出 %s x%d", gd.GiftName, gd.Num), FromCurrent: true, IsGift: true, GiftName: gd.GiftName, GiftNum: gd.Num}
}

func brotliDecompress(data []byte) []byte {
	r := brotli.NewReader(bytes.NewReader(data))
	var out bytes.Buffer
	out.ReadFrom(r)
	return out.Bytes()
}
