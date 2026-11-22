package danmaku

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type giftTransport func(*http.Request) (*http.Response, error)

func (f giftTransport) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestGiftPollerRequiresRoomOwnerAndUsesSession(t *testing.T) {
	oldClient := httpClient
	defer func() { httpClient = oldClient; SetCookie("") }()
	SetCookie("fake-session")
	ownerID := "42"
	giftCalls := 0
	httpClient = &http.Client{Transport: giftTransport(func(req *http.Request) (*http.Response, error) {
		var body string
		switch req.URL.Path {
		case "/x/web-interface/nav":
			if req.Header.Get("Cookie") != "SESSDATA=fake-session" {
				t.Error("session missing from nav request")
			}
			body = `{"code":0,"data":{"mid":42,"isLogin":true}}`
		case "/room/v1/Room/room_init":
			body = `{"code":0,"data":{"uid":` + ownerID + `}}`
		default:
			giftCalls++
			if req.Header.Get("Cookie") != "SESSDATA=fake-session" {
				t.Error("session missing from gift request")
			}
			body = `{"code":0,"data":{"list":[{"id":"one","uid":3,"uname":"观众","gift_name":"小花花","gift_num":1,"time":"now"}]}}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	g := NewGiftPoller(123)
	g.fetch()
	select {
	case msg := <-g.Messages():
		if msg.GiftName != "小花花" {
			t.Fatal("gift missing")
		}
	case <-time.After(time.Second):
		t.Fatal("gift not delivered")
	}
	ownerID = "43"
	g.fetch()
	if giftCalls != 1 {
		t.Fatal("non-owner gift endpoint was called")
	}
}
