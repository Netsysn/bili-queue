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
				if err == nil {
					log.Printf("[WS] Gift: %s %s x%d", g.Uname, g.GiftName, g.Num)
					select {
					case c.msgCh <- DanmakuMsg{
						UID: 0, Username: g.Uname,
						Content: fmt.Sprintf("送出 %s x%d", g.GiftName, g.Num),
						FromCurrent: true, IsGift: true,
						GiftName: g.GiftName, GiftNum: g.Num,
					}:
					default:
					}
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
				// 库不识别的消息类型，尝试从 raw JSON 提取礼物信息
				if gm, ok := tp.Msg.(*live.MsgGeneral); ok {
					data := gm.Raw()
					var gift struct {
						Cmd  string `json:"cmd"`
						Data struct {
							Uname    string `json:"uname"`
							GiftName string `json:"giftName"`
							Num      int    `json:"num"`
							Action   string `json:"action"`
						} `json:"data"`
					}
					if json.Unmarshal(data, &gift) == nil && gift.Cmd != "" {
						if strings.Contains(gift.Cmd, "GIFT") || gift.Data.GiftName != "" {
							SetLive(true)
							name := gift.Data.Uname
							gname := gift.Data.GiftName
							n := gift.Data.Num
							if n == 0 { n = 1 }
							log.Printf("[WS] Gift(raw): %s %s x%d (cmd=%s)", name, gname, n, gift.Cmd)
							select {
							case c.msgCh <- DanmakuMsg{
								Username: name, Content: fmt.Sprintf("送出 %s x%d", gname, n),
								FromCurrent: true, IsGift: true,
								GiftName: gname, GiftNum: n,
							}:
							default:
							}
						}
					}
				}
				if msgCount <= 20 {
					log.Printf("[WS] OTHER: %T", m)
				}
			}
		case <-c.stopCh:
			SetLive(false)
			return nil
		}
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
