package danmaku

import (
	"net/http"
	"testing"
)

func TestCookieOnlySentToBilibili(t *testing.T) {
	defer SetCookie("")
	SetCookie("SESSDATA=test-session; other=not-sent")
	req, _ := http.NewRequest("GET", "https://api.bilibili.com/x/web-interface/nav", nil)
	if !addBiliCookie(req) || req.Header.Get("Cookie") != "SESSDATA=test-session" {
		t.Fatal("session cookie missing or included extra data")
	}
	external, _ := http.NewRequest("GET", "https://bilibili.com.example.org/", nil)
	if addBiliCookie(external) || external.Header.Get("Cookie") != "" {
		t.Fatal("cookie sent to third party")
	}
	insecure, _ := http.NewRequest("GET", "http://api.bilibili.com/", nil)
	if addBiliCookie(insecure) {
		t.Fatal("cookie sent over HTTP")
	}
}
