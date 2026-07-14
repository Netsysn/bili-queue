package danmaku

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	wbiKey     string
	wbiKeyMu   sync.Mutex
	wbiKeyTime time.Time
)

func getWbiKey() string {
	wbiKeyMu.Lock()
	defer wbiKeyMu.Unlock()
	if wbiKey != "" && time.Since(wbiKeyTime) < time.Hour {
		return wbiKey
	}
	// 从 nav API 获取 mix_key
	req, _ := http.NewRequest("GET", "https://api.bilibili.com/x/web-interface/nav", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	resp, err := httpClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var r struct {
		Data struct {
			WbiImg struct {
				ImgURL string `json:"img_url"`
				SubURL string `json:"sub_url"`
			} `json:"wbi_img"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &r) != nil {
		return ""
	}
	imgKey := extractKey(r.Data.WbiImg.ImgURL)
	subKey := extractKey(r.Data.WbiImg.SubURL)
	wbiKey = imgKey + subKey
	wbiKeyTime = time.Now()
	return wbiKey
}

func extractKey(url string) string {
	// url like "https://i0.hdslb.com/bfs/wbi/653657f524a547ac981ded72ea172057.png"
	idx := strings.LastIndex(url, "/")
	if idx < 0 {
		return ""
	}
	fn := url[idx+1:]
	dot := strings.Index(fn, ".")
	if dot < 0 {
		return ""
	}
	return fn[:dot]
}

func wbiSign(params map[string]string) (wts, wRid string) {
	key := getWbiKey()
	if key == "" {
		return "", ""
	}
	ts := time.Now().Unix()
	params["wts"] = strconv.FormatInt(ts, 10)
	var keys []string
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	for i, k := range keys {
		if i > 0 {
			sb.WriteByte('&')
		}
		sb.WriteString(k)
		sb.WriteByte('=')
		sb.WriteString(params[k])
	}
	sb.WriteString(key)
	hash := md5.Sum([]byte(sb.String()))
	return params["wts"], fmt.Sprintf("%x", hash)
}
