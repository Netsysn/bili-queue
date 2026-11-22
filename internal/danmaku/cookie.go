package danmaku

import (
	"net/http"
	"net/http/cookiejar"
	"strings"
	"sync"
	"time"
)

var cookieState struct {
	sync.RWMutex
	sessdata string
}

func initHTTPClient() {
	jar, _ := cookiejar.New(nil)
	httpClient = &http.Client{Timeout: 5 * time.Second, Jar: jar}
}

// 仅保存 SESSDATA，避免把用户输入的其他 Cookie 发送到意外的服务。
func SetCookie(c string) {
	value := strings.TrimSpace(c)
	if strings.Contains(value, "=") {
		value = ""
		for _, part := range strings.Split(c, ";") {
			name, v, ok := strings.Cut(strings.TrimSpace(part), "=")
			if ok && strings.EqualFold(strings.TrimSpace(name), "SESSDATA") {
				value = strings.TrimSpace(v)
				break
			}
		}
	}
	if strings.ContainsAny(value, "\r\n;") {
		value = ""
	}
	cookieState.Lock()
	cookieState.sessdata = value
	cookieState.Unlock()
}

func addBiliCookie(req *http.Request) bool {
	if req.URL.Scheme != "https" || !(req.URL.Hostname() == "bilibili.com" || strings.HasSuffix(req.URL.Hostname(), ".bilibili.com")) {
		return false
	}
	cookieState.RLock()
	value := cookieState.sessdata
	cookieState.RUnlock()
	if value == "" {
		return false
	}
	req.AddCookie(&http.Cookie{Name: "SESSDATA", Value: value})
	return true
}
