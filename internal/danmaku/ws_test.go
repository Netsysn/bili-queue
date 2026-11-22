package danmaku

import (
	"encoding/binary"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestWSLoopSkipsTruncatedPacket(t *testing.T) {
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		bad := makePacket(5, nil)
		binary.BigEndian.PutUint32(bad[:4], 100)
		_ = conn.WriteMessage(websocket.BinaryMessage, bad)
		good := makePacket(5, []byte(`{"cmd":"LIVE_OPEN_PLATFORM_DM","data":{"uid":123,"uname":"viewer","msg":"排队"}}`))
		_ = conn.WriteMessage(websocket.BinaryMessage, good)
	}))
	defer server.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	messages := make(chan DanmakuMsg, 1)
	stop := make(chan struct{})
	defer close(stop)
	go wsLoop(conn, messages, stop)
	select {
	case msg, ok := <-messages:
		if !ok || msg.UID != 123 || msg.Content != "排队" {
			t.Fatalf("unexpected message: %+v, open=%v", msg, ok)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for message after truncated packet")
	}
}

func TestMakePacket(t *testing.T) {
	packet := makePacket(7, []byte("auth"))
	if binary.BigEndian.Uint32(packet[:4]) != uint32(len(packet)) || binary.BigEndian.Uint16(packet[4:6]) != 16 || binary.BigEndian.Uint32(packet[8:12]) != 7 || string(packet[16:]) != "auth" {
		t.Fatalf("invalid packet: %v", packet)
	}
}

func TestParseGiftEventIDTypes(t *testing.T) {
	for _, raw := range []string{
		`{"uname":"viewer","uid":1,"giftName":"花","num":1,"tid":"123"}`,
		`{"uname":"viewer","uid":1,"giftName":"花","num":1,"tid":123}`,
	} {
		msg := parseGift([]byte(raw))
		if msg.GiftName != "花" || msg.EventID != "ws:123" {
			t.Fatalf("gift lost or invalid: %+v", msg)
		}
	}
}

func TestLiveEventsSetRoomState(t *testing.T) {
	liveStatus.mu.Lock()
	liveStatus.live, liveStatus.known = false, false
	liveStatus.mu.Unlock()
	defer func() { liveStatus.mu.Lock(); liveStatus.live, liveStatus.known = false, false; liveStatus.mu.Unlock() }()
	parseMessages([]byte(`{"cmd":"LIVE"}`), make(chan DanmakuMsg, 1))
	if live, known := LiveStatus(); !live || !known {
		t.Fatal("LIVE event not applied")
	}
	parseMessages([]byte(`{"cmd":"PREPARING"}`), make(chan DanmakuMsg, 1))
	if live, known := LiveStatus(); live || !known {
		t.Fatal("PREPARING event not applied")
	}
}

func TestProcessFrameReadsAllPackets(t *testing.T) {
	messages := make(chan DanmakuMsg, 2)
	a := makePacket(5, []byte(`{"cmd":"LIVE_OPEN_PLATFORM_DM","data":{"uid":1,"msg":"排队"}}`))
	b := makePacket(5, []byte(`{"cmd":"SEND_GIFT","data":{"uid":2,"uname":"观众","giftName":"小花花","num":1}}`))
	if !processFrame(append(a, b...), messages) {
		t.Fatal("valid frame rejected")
	}
	first, second := <-messages, <-messages
	if first.Content != "排队" || !second.IsGift || second.GiftName != "小花花" {
		t.Fatalf("unexpected messages: %+v %+v", first, second)
	}
}

func TestParseDMReply(t *testing.T) {
	tests := []struct {
		name     string
		metadata any
		want     string
	}{
		{"extra json", map[string]any{"extra": `{"show_reply":true,"reply_uname":"小明"}`}, "小明"},
		{"metadata fallback", map[string]any{"reply_uname": "小红"}, "小红"},
		{"malformed extra", map[string]any{"extra": "{"}, ""},
		{"missing", nil, ""},
		{"untrusted name retained as text", map[string]any{"extra": `{"reply_uname":"<img onerror=alert(1)>"}`}, "<img onerror=alert(1)>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := make([]any, 16)
			meta[15] = tt.metadata
			msg := parseDM([]any{meta, "帮帮我", []any{float64(123), "观众"}})
			if msg.ReplyTo != tt.want || msg.Content != "帮帮我" {
				t.Fatalf("unexpected parsed message: %+v", msg)
			}
		})
	}
}

func TestWSLoopClosesSilentConnection(t *testing.T) {
	upgrader := websocket.Upgrader{}
	done := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		<-done
	}))
	defer server.Close()
	defer close(done)
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	messages := make(chan DanmakuMsg, 1)
	stop := make(chan struct{})
	defer close(stop)
	go wsLoopWithTimeout(conn, messages, stop, time.Hour, 100*time.Millisecond)
	select {
	case _, ok := <-messages:
		if ok {
			t.Fatal("unexpected message")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("silent connection was not closed")
	}
}

func TestWSLoopReceivesLaterMessages(t *testing.T) {
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.WriteMessage(websocket.BinaryMessage, makePacket(5, []byte(`{"cmd":"LIVE_OPEN_PLATFORM_DM","data":{"uid":1,"msg":"第一条"}}`)))
		time.Sleep(50 * time.Millisecond)
		_ = conn.WriteMessage(websocket.BinaryMessage, makePacket(5, []byte(`{"cmd":"LIVE_OPEN_PLATFORM_DM","data":{"uid":2,"msg":"后续弹幕"}}`)))
	}))
	defer server.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	messages := make(chan DanmakuMsg, 2)
	stop := make(chan struct{})
	defer close(stop)
	go wsLoopWithTimeout(conn, messages, stop, time.Second, 2*time.Second)
	for _, want := range []string{"第一条", "后续弹幕"} {
		select {
		case msg, ok := <-messages:
			if !ok || msg.Content != want {
				t.Fatalf("wanted %q, got %+v, open=%v", want, msg, ok)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("later message did not arrive")
		}
	}
}
