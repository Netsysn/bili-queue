package danmaku

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestHistoryReplyIsSeparateFromText(t *testing.T) {
	old := httpClient
	defer func() { httpClient = old }()
	httpClient = &http.Client{Transport: giftTransport(func(req *http.Request) (*http.Response, error) {
		body := `{"code":0,"data":{"room":[{"uid":12,"nickname":"观众","text":"排队","timeline":"2026-01-02 15:04:05","reply":{"reply_type_enum":1,"reply_uname":"小明"}}]}}`
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	h := NewHistoryFetcher(1)
	h.streamStart = time.Date(2026, 1, 2, 14, 0, 0, 0, time.Local)
	h.fetch()
	select {
	case msg := <-h.Messages():
		if msg.ReplyTo != "小明" || msg.Content != "排队" {
			t.Fatalf("unexpected reply: %+v", msg)
		}
	default:
		t.Fatal("history reply not delivered")
	}
}

func TestHistoryEmotesPreservePlainText(t *testing.T) {
	old := httpClient
	defer func() { httpClient = old }()
	httpClient = &http.Client{Transport: giftTransport(func(req *http.Request) (*http.Response, error) {
		body := `{"code":0,"data":{"room":[{"uid":12,"nickname":"观众","text":"排队[笑]","timeline":"2026-01-02 15:04:05","emots":{"[笑]":{"url":"https://i0.hdslb.com/a.png","width":20,"height":20}}}]}}`
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	h := NewHistoryFetcher(1)
	h.streamStart = time.Date(2026, 1, 2, 14, 0, 0, 0, time.Local)
	h.fetch()
	select {
	case msg := <-h.Messages():
		if msg.Content != "排队[笑]" || msg.Emotes["[笑]"].URL != "https://i0.hdslb.com/a.png" {
			t.Fatalf("unexpected history emote: %+v", msg)
		}
	default:
		t.Fatal("history message not delivered")
	}
}
