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
