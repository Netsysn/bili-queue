package danmaku

import (
	"net/http"
	"net/http/cookiejar"
	"time"
)

var biliCookie string

func initHTTPClient() {
	jar, _ := cookiejar.New(nil)
	httpClient = &http.Client{
		Timeout: 5 * time.Second,
		Jar:     jar,
	}
}

func SetCookie(c string) {
	biliCookie = c
}
