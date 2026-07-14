package danmaku

import (
	"fmt"
	"log"
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
	ch, err := connectWS(c.roomID)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	log.Printf("ws: connected to room %d", c.roomID)

	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return fmt.Errorf("ws closed")
			}
			select {
			case c.msgCh <- msg:
			default:
			}
		case <-c.stopCh:
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

// checkLive stub for compatibility
func (c *Client) checkLive(realID int64) {}
