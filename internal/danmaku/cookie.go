package danmaku

import (
	"net/http"
	"time"
)

var biliCookie string

func SetCookie(c string) {
	biliCookie = c
	if c != "" {
		httpClient = &http.Client{
			Timeout: 5 * time.Second,
			Transport: &cookieTransport{http.DefaultTransport},
		}
	} else {
		httpClient = &http.Client{Timeout: 5 * time.Second}
	}
}

type cookieTransport struct {
	base http.RoundTripper
}

func (t *cookieTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if biliCookie != "" {
		req.Header.Set("Cookie", biliCookie)
	}
	return t.base.RoundTrip(req)
}
