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
	ReplyTo     string // @回复的用户名
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
	// 连接后立即查一次 room_init 确定开播状态
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
	for {
		select {
		case tp := <-l.Rev:
			if tp.Error != nil {
				continue
			}
			switch m := tp.Msg.(type) {
			case *live.MsgHeartbeatReply:
				// 每 60s 重新确认开播状态
				if time.Since(lastCheck) > 60*time.Second {
					c.checkLive(realID)
					lastCheck = time.Now()
				}
			case *live.MsgLive:
				SetLive(true)
			case *live.MsgPreparing:
				SetLive(false)
			case *live.MsgDanmaku:
				log.Printf("[WS] MsgDanmaku -> SetLive(true)")
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
