package danmaku

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

// GiftPoller 通过 HTTP API 轮询收到的礼物列表（需要主播 Cookie）。
type GiftPoller struct {
	roomID   int64
	lastID   string
	seen     map[string]bool
	mu       sync.Mutex
	msgCh    chan DanmakuMsg
	stopCh   chan struct{}
}

func NewGiftPoller(roomID int64) *GiftPoller {
	return &GiftPoller{
		roomID: roomID,
		seen:   make(map[string]bool),
		msgCh:  make(chan DanmakuMsg, 64),
		stopCh: make(chan struct{}),
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
	params := url.Values{}
	params.Set("limit", "50")
	params.Set("coin_type", "0") // 0=all
	params.Set("begin_time", strconv.FormatInt(time.Now().Add(-24*time.Hour).Unix(), 10))
	if g.lastID != "" {
		params.Set("last_id", g.lastID)
	}

	api := fmt.Sprintf("https://api.live.bilibili.com/xlive/revenue/v1/giftStream/getReceivedGiftStreamNextList?%s",
		params.Encode())
	req, _ := http.NewRequest("GET", api, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	req.Header.Set("Referer", "https://link.bilibili.com/p/center/index")

	resp, err := httpClient.Do(req)
	if err != nil {
		return
	}
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
		}:
		default:
		}
	}

	if len(r.Data.List) > 0 {
		g.lastID = r.Data.List[len(r.Data.List)-1].ID
	}
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
