package danmaku

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

type HistoryFetcher struct {
	roomID      int64
	streamStart time.Time
	lastSeen    map[string]bool
	mu          sync.Mutex
	msgCh       chan DanmakuMsg
	stopCh      chan struct{}
	IsLive      bool
	LiveTime    string
}

func NewHistoryFetcher(roomID int64) *HistoryFetcher {
	return &HistoryFetcher{
		roomID:   roomID,
		lastSeen: make(map[string]bool),
		msgCh:    make(chan DanmakuMsg, 256),
		stopCh:   make(chan struct{}),
	}
}

func (h *HistoryFetcher) Messages() <-chan DanmakuMsg { return h.msgCh }
func (h *HistoryFetcher) LiveStatus() (bool, string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.IsLive, h.LiveTime
}
func (h *HistoryFetcher) Stop() {
	select {
	case <-h.stopCh:
	default:
		close(h.stopCh)
	}
}

func (h *HistoryFetcher) Poll(interval time.Duration) {
	h.refreshLiveTime()
	h.fetch()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	ticks := 0
	for {
		select {
		case <-ticker.C:
			ticks++
			if ticks%10 == 0 {
				h.refreshLiveTime()
			}
			h.fetch()
		case <-h.stopCh:
			return
		}
	}
}

func (h *HistoryFetcher) FetchOnce() { h.fetch() }

func (h *HistoryFetcher) refreshLiveTime() {
	realID, err := ResolveRoomID(h.roomID)
	if err != nil {
		return
	}
	api := fmt.Sprintf("https://api.live.bilibili.com/room/v1/Room/room_init?id=%d", realID)
	req, _ := http.NewRequest("GET", api, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	req.Header.Set("Referer", "https://live.bilibili.com/")
	resp, err := httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var r struct {
		Data struct {
			LiveTime   int64 `json:"live_time"`
			LiveStatus int   `json:"live_status"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &r) != nil {
		return
	}
	h.mu.Lock()
	h.IsLive = r.Data.LiveStatus == 1
	h.mu.Unlock()
	if r.Data.LiveTime > 0 {
		h.streamStart = time.Unix(r.Data.LiveTime, 0)
		h.LiveTime = h.streamStart.Format("15:04:05")
	}
}

type dmEntry struct {
	Text        string `json:"text"`
	UID         int64  `json:"uid"`
	Nickname    string `json:"nickname"`
	Timeline    string `json:"timeline"`
	Vip         int    `json:"vip"`
	Medal       []any  `json:"medal"`
	UserLevel   []any  `json:"user_level"`
	WealthLevel int    `json:"wealth_level"`
	Reply       struct {
		ReplyType  int    `json:"reply_type_enum"`
		ReplyUname string `json:"reply_uname"`
	} `json:"reply"`
	Emoticon struct {
		Text   string `json:"text"`
		URL    string `json:"url"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
	} `json:"emoticon"`
	Emots map[string]struct {
		URL    string `json:"url"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
	} `json:"emots"`
	User struct {
		Base struct {
			Face string `json:"face"`
		} `json:"base"`
		Medal struct {
			Name  string `json:"name"`
			Level int    `json:"level"`
		} `json:"medal"`
	} `json:"user"`
}

func (h *HistoryFetcher) fetch() {
	api := fmt.Sprintf("https://api.live.bilibili.com/xlive/web-room/v1/dM/gethistory?roomid=%d&room_type=0", h.roomID)
	req, _ := http.NewRequest("GET", api, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	req.Header.Set("Referer", "https://live.bilibili.com/")
	resp, err := httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var r struct {
		Code int `json:"code"`
		Data struct {
			Room []dmEntry `json:"room"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &r); err != nil || r.Code != 0 {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	for _, dm := range r.Data.Room {
		t, err := time.ParseInLocation("2006-01-02 15:04:05", dm.Timeline, time.Local)
		if err != nil {
			continue
		}
		if !h.streamStart.IsZero() && t.Before(h.streamStart) {
			continue
		}
		key := fmt.Sprintf("%d:%s", dm.UID, dm.Timeline)
		if h.lastSeen[key] {
			continue
		}
		h.lastSeen[key] = true

		avatar := strings.Replace(dm.User.Base.Face, "http://", "https://", 1)

		medalName := dm.User.Medal.Name
		medalLevel := dm.User.Medal.Level
		if medalName == "" && len(dm.Medal) >= 2 {
			if s, ok := dm.Medal[1].(string); ok {
				medalName = s
			}
			if v, ok := dm.Medal[0].(float64); ok {
				medalLevel = int(v)
			}
		}
		userLevel := dm.WealthLevel
		if userLevel == 0 && len(dm.UserLevel) > 0 {
			if v, ok := dm.UserLevel[0].(float64); ok {
				userLevel = int(v)
			}
		}

		content := dm.Text
		if dm.Reply.ReplyType == 1 && dm.Reply.ReplyUname != "" {
			content = "<b>@" + dm.Reply.ReplyUname + "</b> " + content
		}

		if dm.Emots != nil {
			for name, em := range dm.Emots {
				if em.URL != "" {
					content = replaceEmote(content, name, em.URL, em.Width, em.Height)
				}
			}
		}
		if dm.Emoticon.URL != "" && dm.Emoticon.Text != "" {
			content = replaceEmote(content, dm.Emoticon.Text, dm.Emoticon.URL,
				dm.Emoticon.Width, dm.Emoticon.Height)
		}
		content = cleanBrackets(content)

		username := dm.Nickname
		if dm.UID == 0 || strings.Contains(username, "***") {
			username = "匿名用户"
		}

		select {
		case h.msgCh <- DanmakuMsg{
			UID: dm.UID, Username: username, Avatar: avatar,
			Content: content, FromCurrent: !h.streamStart.IsZero() && !t.Before(h.streamStart),
			MedalName: medalName, MedalLevel: medalLevel,
			UserLevel: userLevel, Vip: dm.Vip,
		}:
		default:
		}
	}
	if len(h.lastSeen) > 5000 {
		h.lastSeen = make(map[string]bool)
	}
}

func replaceEmote(text, name, url string, w, h int) string {
	if w == 0 {
		w = 20
	}
	if h == 0 {
		h = 20
	}
	url = strings.Replace(url, "http://", "https://", 1)
	img := fmt.Sprintf(`<img src="%s" referrerpolicy="no-referrer" style="width:%dpx;height:%dpx;vertical-align:middle">`,
		url, w, h)
	return strings.ReplaceAll(text, name, img)
}

var bracketRe = regexp.MustCompile(`\[[^\]]+\]`)

func cleanBrackets(text string) string {
	return bracketRe.ReplaceAllStringFunc(text, func(s string) string {
		if strings.Contains(s, "<img") {
			return s
		}
		return s
	})
}
