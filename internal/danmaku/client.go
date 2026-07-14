package danmaku

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	live "github.com/iyear/biligo-live"
)

type DanmakuMsg struct {
	UID         int64
	Username    string
	Avatar      string
	Content     string
	FromCurrent bool
	MedalName   string
	MedalLevel  int
	UserLevel   int
	Vip         int
	IsGift      bool
	GiftName    string
	GiftNum     int
}

type Client struct {
	roomID int64
	msgCh  chan DanmakuMsg
	stopCh chan struct{}
}

func New(roomID int64) *Client {
	return &Client{
		roomID: roomID,
		msgCh:  make(chan DanmakuMsg, 512),
		stopCh: make(chan struct{}),
	}
}

func (c *Client) Messages() <-chan DanmakuMsg { return c.msgCh }

func (c *Client) Connect() error {
	realID, err := ResolveRoomID(c.roomID)
	if err != nil {
		return fmt.Errorf("resolve room: %w", err)
	}
	c.checkLive(realID)
	token := getToken(realID)

	l := live.NewLive(false, 30*time.Second, 0, nil)
	if err := l.Conn(websocket.DefaultDialer, live.WsDefaultHost); err != nil {
		return fmt.Errorf("conn: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		if err := l.Enter(ctx, realID, token, 0); err != nil {
			log.Printf("danmaku enter: %v", err)
		}
	}()

	lastCheck := time.Now()
	msgCount := 0
	for {
		select {
		case tp := <-l.Rev:
			if tp.Error != nil {
				continue
			}
			msgCount++
			if msgCount <= 30 {
				log.Printf("[WS] msg#%d type=%T", msgCount, tp.Msg)
			}
			switch m := tp.Msg.(type) {
			case *live.MsgHeartbeatReply:
				if time.Since(lastCheck) > 60*time.Second {
					c.checkLive(realID)
					lastCheck = time.Now()
				}
			case *live.MsgLive:
				SetLive(true)
			case *live.MsgPreparing:
				SetLive(false)
			case *live.MsgSendGift:
				SetLive(true)
				g, err := m.Parse()
				if err != nil {
					log.Printf("[WS] SendGift parse ERR: %v", err)
					continue
				}
				log.Printf("[WS] SendGift: %s %s x%d", g.Uname, g.GiftName, g.Num)
				select {
				case c.msgCh <- DanmakuMsg{
					Username: g.Uname, Content: fmt.Sprintf("送出 %s x%d", g.GiftName, g.Num),
					FromCurrent: true, IsGift: true, GiftName: g.GiftName, GiftNum: g.Num,
				}:
				default:
				}
			case *live.MsgDanmaku:
				SetLive(true)
				dm, err := m.Parse()
				if err != nil {
					continue
				}
				uname := dm.Uname
				if dm.MID == 0 || strings.Contains(uname, "***") {
					uname = "匿名用户"
				}
				select {
				case c.msgCh <- DanmakuMsg{
					UID: dm.MID, Username: uname, Content: dm.Content,
					FromCurrent: true,
					MedalName: dm.MedalName, MedalLevel: dm.MedalLevel,
					UserLevel: dm.UserLevel, Vip: dm.Vip,
				}:
				default:
				}
			default:
				if gm, ok := tp.Msg.(*live.MsgGeneral); ok {
					c.parseRawGift(gm.Raw(), msgCount)
				}
			}
		case <-c.stopCh:
			SetLive(false)
			return nil
		}
	}
}

func (c *Client) parseRawGift(raw []byte, n int) {
	var outer struct {
		Cmd  string          `json:"cmd"`
		Data json.RawMessage `json:"data"`
	}
	if json.Unmarshal(raw, &outer) != nil {
		return
	}
	log.Printf("[WS] msg#%d MsgGeneral cmd=%s", n, outer.Cmd)

	if !strings.Contains(outer.Cmd, "GIFT") {
		return
	}

	var gd struct {
		Uname    string `json:"uname"`
		GiftName string `json:"giftName"`
		Num      int    `json:"num"`
		UID      int64  `json:"uid"`
	}
	if json.Unmarshal(outer.Data, &gd) != nil {
		var str string
		if json.Unmarshal(outer.Data, &str) == nil {
			json.Unmarshal([]byte(str), &gd)
		}
	}
	if gd.GiftName == "" {
		log.Printf("[WS] msg#%d gift parse: giftName empty, raw=%s", n, string(raw))
		return
	}
	SetLive(true)
	if gd.Num == 0 {
		gd.Num = 1
	}
	log.Printf("[WS] Gift(raw): uid=%d %s %s x%d", gd.UID, gd.Uname, gd.GiftName, gd.Num)
	select {
	case c.msgCh <- DanmakuMsg{
		UID: gd.UID, Username: gd.Uname,
		Content: fmt.Sprintf("送出 %s x%d", gd.GiftName, gd.Num),
		FromCurrent: true, IsGift: true, GiftName: gd.GiftName, GiftNum: gd.Num,
	}:
	default:
	}
}

func (c *Client) checkLive(realID int64) {
	api := fmt.Sprintf("https://api.live.bilibili.com/room/v1/Room/room_init?id=%d", realID)
	req, _ := http.NewRequest("GET", api, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	resp, err := httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var r struct {
		Data struct {
			LiveStatus int `json:"live_status"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &r) == nil {
		SetLive(r.Data.LiveStatus == 1)
	}
}

func (c *Client) Stop() {
	select {
	case <-c.stopCh:
	default:
		close(c.stopCh)
	}
}
