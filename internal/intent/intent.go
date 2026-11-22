// Package intent 识别弹幕中的排队请求并提取类型与服务器。
package intent

import (
	"regexp"
	"strings"
	"unicode"
)

// 明确的取消、否定、状态查询和讨论不应触发入队。
var rejectPatterns = []*regexp.Regexp{
	regexp.MustCompile(`取消.{0,4}(排队|上车)|退出.{0,4}(排队|队列)|不想.{0,4}(排|上车|帮|带)`),
	regexp.MustCompile(`不.{0,3}(排队|排了|排个|排一|上车|帮|带)|别.{0,3}(排队|上车|帮|带)`),
	regexp.MustCompile(`(怎么|咋|如何|哪里|在哪|去哪).{0,5}(排队|排|上车)`),
	regexp.MustCompile(`(多少|几个|多久|几位|人数|第几|还有没有).{0,8}(排队|队列|排|上车)|(排队|队列).{0,5}(多少|几个人|多久|人数|规则|系统)`),
	regexp.MustCompile(`(已经|正在|刚刚|刚才).{0,4}(排队|排过|上车)|(排队中|排过了|排好了)`),
	regexp.MustCompile(`(听说|有人说|听人说|插队)`),
	regexp.MustCompile(`(?i)傻逼|垃圾|废物|ntmd|cnm|草泥马`),
}

// “我能排队吗”是请求；“有人排队吗”则通常是在询问。
var personalRequest = regexp.MustCompile(`(我|俺|本人).{0,8}(排队|排个|排一|排下|帮|带|上车|求带)`)
var questionWord = regexp.MustCompile(`[?？吗]|(是不是|能不能|可不可以|有没有|什么|啥)`)

var fuzzyMap = map[string]*regexp.Regexp{
	"排队": regexp.MustCompile(`排(?:个队|一排|一下|下|了|个号|个位置)`),
	"帮帮": regexp.MustCompile(`帮(?:帮我|我一下|我一把|一下|个忙)`),
	"带带": regexp.MustCompile(`带(?:带我|我一下|我一把|一下|我玩|个我)`),
	"上车": regexp.MustCompile(`上(?:车|船)`),
}

// normalize 将全角英数和空格归一化，使 B服、ｂ服等写法一致。
func normalize(s string) string {
	return strings.ToLower(strings.Map(func(r rune) rune {
		switch {
		case r >= '！' && r <= '～':
			return r - 0xFEE0
		case unicode.IsSpace(r):
			return ' '
		case r == '\u200b' || r == '\ufeff':
			return -1
		default:
			return r
		}
	}, strings.TrimSpace(s)))
}

// Match 返回命中的配置值。服务器可选；空关键词不会匹配任何内容。
func Match(text string, helpTypes, servers []string) (string, string, bool) {
	text = normalize(text)
	if text == "" {
		return "", "", false
	}
	for _, pattern := range rejectPatterns {
		if pattern.MatchString(text) {
			return "", "", false
		}
	}
	if questionWord.MatchString(text) && !personalRequest.MatchString(text) {
		return "", "", false
	}
	helpType := matchOne(text, helpTypes, true)
	if helpType == "" {
		return "", "", false
	}
	return helpType, matchOne(text, servers, false), true
}

// 优先选择最长的精确关键词；仅帮类型使用预定义的口语变体。
func matchOne(text string, keywords []string, fuzzy bool) string {
	best, bestLen := "", 0
	for _, keyword := range keywords {
		normalized := normalize(keyword)
		if normalized == "" {
			continue
		}
		if strings.Contains(text, normalized) && len([]rune(normalized)) > bestLen {
			best, bestLen = keyword, len([]rune(normalized))
		}
	}
	if best != "" || !fuzzy {
		return best
	}
	for _, keyword := range keywords {
		if re := fuzzyMap[normalize(keyword)]; re != nil && re.MatchString(text) {
			return keyword
		}
	}
	return ""
}
