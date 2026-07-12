package danmaku

import (
	"context"
	"fmt"
	"log"
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

	for {
		select {
		case tp := <-l.Rev:
			if tp.Error != nil {
				continue
			}
			switch m := tp.Msg.(type) {
			case *live.MsgHeartbeatReply:
				SetLive(true)
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
			}
		case <-c.stopCh:
			SetLive(false)
			return nil
		}
	}
}

func (c *Client) Stop() {
	select {
	case <-c.stopCh:
	default:
		close(c.stopCh)
	}
}
