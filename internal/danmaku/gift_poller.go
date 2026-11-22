package danmaku

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

// GiftPoller 通过 HTTP API 轮询收到的礼物列表（需要主播 Cookie）。
type GiftPoller struct {
	roomID    int64
	lastID    string
	seen      map[string]bool
	mu        sync.Mutex
	msgCh     chan DanmakuMsg
	stopCh    chan struct{}
	startedAt time.Time
}

func NewGiftPoller(roomID int64) *GiftPoller {
	return &GiftPoller{
		roomID:    roomID,
		seen:      make(map[string]bool),
		msgCh:     make(chan DanmakuMsg, 64),
		stopCh:    make(chan struct{}),
		startedAt: time.Now(),
	}
}

func (g *GiftPoller) Messages() <-chan DanmakuMsg { return g.msgCh }
func (g *GiftPoller) Stop() {
	select {
	case <-g.stopCh:
	default:
		close(g.stopCh)
	}
}

func (g *GiftPoller) Poll(interval time.Duration) {
	g.fetch()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			g.fetch()
		case <-g.stopCh:
			return
		}
	}
}

func (g *GiftPoller) fetch() {
	// 主播收益接口属于账号而非任意直播间，必须确认账号是当前房间主播。
	if !g.isRoomOwner() {
		return
	}
	params := url.Values{}
	params.Set("limit", "50")
	params.Set("coin_type", "0") // 0=all
	params.Set("begin_time", strconv.FormatInt(g.startedAt.Unix(), 10))
	if g.lastID != "" {
		params.Set("last_id", g.lastID)
	}

	api := fmt.Sprintf("https://api.live.bilibili.com/xlive/revenue/v1/giftStream/getReceivedGiftStreamNextList?%s",
		params.Encode())
	req, _ := http.NewRequest("GET", api, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	req.Header.Set("Referer", "https://link.bilibili.com/p/center/index")
	if !addBiliCookie(req) {
		return
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("giftPoller: request err=%v", err)
		return
	}
	log.Printf("giftPoller: status=%d", resp.StatusCode)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var r struct {
		Code int `json:"code"`
		Data struct {
			List    []giftItem `json:"list"`
			HasMore bool       `json:"has_more"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &r) != nil || r.Code != 0 {
		return
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	for _, item := range r.Data.List {
		key := fmt.Sprintf("%s:%s", item.ID, item.Time)
		if g.seen[key] {
			continue
		}
		g.seen[key] = true

		select {
		case g.msgCh <- DanmakuMsg{
			UID: item.UID, Username: item.Uname,
			Content:     fmt.Sprintf("送出 %s x%d", item.GiftName, item.GiftNum),
			FromCurrent: true, IsGift: true,
			GiftName: item.GiftName, GiftNum: item.GiftNum,
			EventID: giftEventID("gift-poll", item.ID),
		}:
		default:
		}
	}

	if len(r.Data.List) > 0 {
		g.lastID = r.Data.List[len(r.Data.List)-1].ID
	}
}

// 只有主播本人的账号可以使用收到礼物的收益接口。
func (g *GiftPoller) isRoomOwner() bool {
	req, _ := http.NewRequest("GET", "https://api.bilibili.com/x/web-interface/nav", nil)
	if !addBiliCookie(req) {
		return false
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	var user struct {
		Code int `json:"code"`
		Data struct {
			MID     int64 `json:"mid"`
			IsLogin bool  `json:"isLogin"`
		} `json:"data"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&user) != nil || user.Code != 0 || !user.Data.IsLogin || user.Data.MID <= 0 {
		return false
	}
	roomReq, _ := http.NewRequest("GET", fmt.Sprintf("https://api.live.bilibili.com/room/v1/Room/room_init?id=%d", g.roomID), nil)
	roomResp, err := httpClient.Do(roomReq)
	if err != nil {
		return false
	}
	defer roomResp.Body.Close()
	var room struct {
		Code int `json:"code"`
		Data struct {
			UID int64 `json:"uid"`
		} `json:"data"`
	}
	return json.NewDecoder(io.LimitReader(roomResp.Body, 1<<20)).Decode(&room) == nil && room.Code == 0 && room.Data.UID == user.Data.MID
}

type giftItem struct {
	ID         string `json:"id"`
	UID        int64  `json:"uid"`
	Uname      string `json:"uname"`
	GiftName   string `json:"gift_name"`
	GiftNum    int    `json:"gift_num"`
	NormalGold int    `json:"normal_gold"`
	Time       string `json:"time"`
}
