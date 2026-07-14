package main

import (
	"context"
	"log"
	"strings"
	"time"

	"bili-queue-overlay/internal/danmaku"
	"bili-queue-overlay/internal/intent"
	"bili-queue-overlay/internal/queue"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type QueueItem struct {
	UID          int64  `json:"uid"`
	Username     string `json:"username"`
	Avatar       string `json:"avatar"`
	Status       int    `json:"status"`
	IsFirst      bool   `json:"is_first"`
	HelpType     string `json:"help_type"`
	Server       string `json:"server"`
	Message      string `json:"message"`
	JoinedAt     string `json:"joined_at"`
	ElapsedSec   int    `json:"elapsed_sec"` // 已等待秒数，前端倒计时用
	MedalName    string `json:"medal_name"`
	MedalLevel   int    `json:"medal_level"`
	UserLevel    int    `json:"user_level"`
}

type LogItem struct {
	UID         int64  `json:"uid"`
	Username    string `json:"username"`
	Avatar      string `json:"avatar"`
	Content     string `json:"content"`
	Time        string `json:"time"`
	IsQueue     bool   `json:"is_queue"`
	FromCurrent bool   `json:"from_current"`
	IsFirstNew  bool   `json:"is_first_new"`
	MedalName   string `json:"medal_name"`
	MedalLevel  int    `json:"medal_level"`
	UserLevel   int    `json:"user_level"`
	ReplyTo     string `json:"reply_to"`
	IsGift      bool   `json:"is_gift"`
	HasMedal    bool   `json:"has_medal"` // 有当前直播间粉丝勋章=可插队
	GiftName    string `json:"gift_name"`
	GiftNum     int    `json:"gift_num"`
}

type QueueUpdated struct {
	Queue          []QueueItem `json:"queue"`
	Logs           []LogItem   `json:"logs"`
	IsLive         bool        `json:"is_live"`
	LiveTime       string      `json:"live_time"`
	TimeoutMinutes int         `json:"timeout_minutes"`
}

type AppService struct {
	app     *application.App
	manager *queue.Manager
	roomID  int64
	msgCh   chan danmaku.DanmakuMsg
	stopCh  chan struct{}
	hist    *danmaku.HistoryFetcher
}

func (s *AppService) ServiceStartup(ctx context.Context, opts application.ServiceOptions) error {
	s.app = appGlobal
	s.msgCh = make(chan danmaku.DanmakuMsg, 512)
	s.stopCh = make(chan struct{})

	cfg := getConfig()
	s.manager = queue.New(func(entries []queue.Entry, logs []queue.DanmakuLog) {
		s.emit(entries, logs)
	}, cfg.TimeoutMinutes)
	s.manager.StartTimeoutChecker()

	go s.consumeLoop()
	go s.wsSource()
	go s.httpSource()
	return nil
}

func (s *AppService) ServiceShutdown() {
	close(s.stopCh)
	if s.hist != nil {
		s.hist.Stop()
	}
	if s.manager != nil {
		s.manager.Stop()
	}
}

func (s *AppService) wsSource() {
	for {
		select {
		case <-s.stopCh:
			return
		default:
		}
		client := danmaku.New(s.roomID)
		done := make(chan struct{})
		go func() {
			defer close(done)
			for msg := range client.Messages() {
				select {
				case s.msgCh <- msg:
				default:
				}
			}
		}()
		if err := client.Connect(); err != nil {
			log.Printf("WS: %v", err)
		}
		<-done
		time.Sleep(3 * time.Second)
	}
}

func (s *AppService) httpSource() {
	s.hist = danmaku.NewHistoryFetcher(s.roomID)
	go func() {
		for msg := range s.hist.Messages() {
			select {
			case s.msgCh <- msg:
			default:
			}
		}
	}()
	s.hist.Poll(1500 * time.Millisecond)
}

func (s *AppService) consumeLoop() {
	for {
		select {
		case msg, ok := <-s.msgCh:
			if !ok {
				return
			}
			s.processMsg(msg)
		case <-s.stopCh:
			return
		}
	}
}

func (s *AppService) processMsg(msg danmaku.DanmakuMsg) {
	// 过滤 B站隐私预览消息（UID=0 是真实消息的预发布版本，后面会跟完整版）
	if msg.UID == 0 {
		return
	}

	cfg := getConfig()
	ht, sv, matched := "", "", false
	if msg.FromCurrent && !cfg.PayMode {
		ht, sv, matched = intent.Match(msg.Content, cfg.HelpTypes, cfg.Servers)
	}
	if matched && sv == "" {
		sv = "未指定" // 有帮类型没服务器，标记待问
	}
	avatar := msg.Avatar
	if avatar == "" {
		avatar = danmaku.GetAvatar(msg.UID)
	}

	s.manager.AddLog(queue.DanmakuLog{
		UID: msg.UID, Username: msg.Username, Avatar: avatar,
		Content: msg.Content, Time: time.Now(),
		IsQueue: matched, FromCurrent: msg.FromCurrent,
		HelpType: ht, Server: sv,
		MedalName: msg.MedalName, MedalLevel: msg.MedalLevel,
		UserLevel: msg.UserLevel,
		IsGift: msg.IsGift, GiftName: msg.GiftName, GiftNum: msg.GiftNum,
	})
	if matched && msg.FromCurrent {
		s.manager.Enqueue(msg.UID, msg.Username, avatar, ht, sv, msg.Content,
			msg.MedalName, msg.MedalLevel, msg.UserLevel)
	}
	// 礼物插队：送了配置的礼物直接入队
	if msg.IsGift && msg.FromCurrent {
		for _, gk := range cfg.GiftQueue {
			if strings.EqualFold(msg.GiftName, gk) {
				s.manager.Enqueue(msg.UID, msg.Username, avatar, "礼物插队", "", msg.Content,
					msg.MedalName, msg.MedalLevel, msg.UserLevel)
				break
			}
		}
	}
}

func (s *AppService) GetQueue() QueueUpdated { return s.buildUpdate() }
func (s *AppService) Refresh() {}
func (s *AppService) Complete()               { s.manager.Complete() }
func (s *AppService) Start() string {
	if err := s.manager.Start(); err != nil {
		return err.Error()
	}
	return ""
}
func (s *AppService) Skip()                   { s.manager.Skip() }
func (s *AppService) Remove(i int)            { s.manager.Remove(i) }
func (s *AppService) Restore(i int)           { s.manager.Restore(i) }

func (s *AppService) Quit() {
	if s.app != nil {
		s.app.Quit()
	}
}

func (s *AppService) GetConfig() Config { return getConfig() }

func (s *AppService) SetFocusMode(on bool) {
	if on {
		removeShadow()
	}
}

func (s *AppService) SaveConfig(c Config) {
	updateConfig(func(cfg *Config) {
		cfg.Theme = c.Theme
		cfg.RoomID = c.RoomID
		cfg.TimeoutMinutes = c.TimeoutMinutes
		cfg.WindowOpacity = c.WindowOpacity
		cfg.PayMode = c.PayMode
		cfg.FocusMode = c.FocusMode
		if len(c.HelpTypes) > 0 { cfg.HelpTypes = c.HelpTypes }
		if len(c.Servers) > 0   { cfg.Servers = c.Servers }
		if len(c.GiftQueue) > 0 { cfg.GiftQueue = c.GiftQueue }
	})
}

func (s *AppService) buildUpdate() QueueUpdated {
	entries := s.manager.List()
	logs := s.manager.Logs()

	qitems := make([]QueueItem, 0, len(entries))
	// 检查是否有进行中——如果有，后面的都不标"等待"
	hasInProgress := false
	for _, e := range entries {
		if e.Status == queue.StatusInProgress {
			hasInProgress = true
			break
		}
	}
	firstFound := false
	for _, e := range entries {
		if e.Status == queue.StatusDone {
			continue
		}
		isFirst := false
		if !firstFound && e.Status == queue.StatusActive && !hasInProgress {
			isFirst = true
			firstFound = true
		}
		qitems = append(qitems, QueueItem{
			UID: e.UID, Username: e.Username, Avatar: e.Avatar, Status: int(e.Status),
			IsFirst: isFirst, HelpType: e.HelpType, Server: e.Server,
			Message: e.Message, JoinedAt: e.JoinedAt.Format("15:04:05"),
			ElapsedSec: int(time.Since(e.JoinedAt).Seconds()),
			MedalName: e.MedalName, MedalLevel: e.MedalLevel, UserLevel: e.UserLevel,
		})
	}

	litems := make([]LogItem, len(logs))
	foundCurrent := false
	for i, l := range logs {
		isFirstNew := false
		if l.FromCurrent && !foundCurrent {
			isFirstNew = true
			foundCurrent = true
		}
		litems[i] = LogItem{
			UID: l.UID, Username: l.Username, Avatar: l.Avatar,
			Content: l.Content, Time: l.Time.Format("15:04:05"),
			IsQueue: l.IsQueue, FromCurrent: l.FromCurrent, IsFirstNew: isFirstNew,
			MedalName: l.MedalName, MedalLevel: l.MedalLevel, UserLevel: l.UserLevel,
			IsGift: l.IsGift, GiftName: l.GiftName, GiftNum: l.GiftNum,
		}
	}
	isLive := danmaku.IsRoomLive()
	cfg := getConfig()
	return QueueUpdated{
		Queue: qitems, Logs: litems,
		IsLive: isLive, LiveTime: "",
		TimeoutMinutes: cfg.TimeoutMinutes,
	}
}

func (s *AppService) emit(_ []queue.Entry, _ []queue.DanmakuLog) {
	if s.app != nil {
		s.app.Event.Emit("queue:updated", s.buildUpdate())
	}
}
