// Package intent 弹幕排队意图识别 + 帮类型/服务器提取。
package intent

import (
	"regexp"
	"strings"
)

var denyPatterns = []*regexp.Regexp{
	regexp.MustCompile(`[怎咋哪]`),
	regexp.MustCompile(`\?|？`),
	regexp.MustCompile(`吗`),
	regexp.MustCompile(`[这那][^排]{3,6}排`), // "这么牛逼的排队"，中间不能含"排"，不杀"那我排一排"
	regexp.MustCompile(`排队.{0,3}(真|太|烂)`), // "排队系统真烂"
	regexp.MustCompile(`如何`),
	regexp.MustCompile(`什么`),
	regexp.MustCompile(`多少|几个|多久`),
	regexp.MustCompile(`啥`),
	regexp.MustCompile(`有什么|有啥`),
	regexp.MustCompile(`有人说|听人说|听说`),
	regexp.MustCompile(`(?i)sb|傻逼|垃圾|废物|ntmd|cnm|草泥马`),
	regexp.MustCompile(`按理|其实|应该|话说`),
	regexp.MustCompile(`[说讲]话`),
	regexp.MustCompile(`可以.{0,3}[吗?？]`),
	regexp.MustCompile(`插队`),
	regexp.MustCompile(`不.{0,2}排`),
	regexp.MustCompile(`排队中`),
	regexp.MustCompile(`已经`),
	regexp.MustCompile(`人数`),
	regexp.MustCompile(`是不是`),
}

// Match 匹配弹幕中的帮类型和服务器。
// 返回 (helpType, server, matched)。
// 匹配到帮类型就算排队意图；服务器可选，没匹配到时 server 为空。
func Match(text string, helpTypes, servers []string) (string, string, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", "", false
	}
	if isNegative(text) {
		return "", "", false
	}

	ht := matchOne(text, helpTypes)
	if ht == "" {
		return "", "", false
	}

	sv := matchOne(text, servers)
	return ht, sv, true
}

// isNegative 检查是否为否定/疑问/闲聊（复用之前的 deny 逻辑）。
func isNegative(text string) bool {
	for _, pat := range denyPatterns {
		if pat.MatchString(text) {
			return true
		}
	}
	return false
}

// matchOne 在文本中查找第一个匹配的关键词，返回原始配置值。
// 帮类型模糊匹配变体
var fuzzyMap = map[string]*regexp.Regexp{
	"排队": regexp.MustCompile(`排[个一队].|排一排|排一下|排了`),
	"帮帮": regexp.MustCompile(`帮[帮我].|帮一下|帮个忙`),
	"带带": regexp.MustCompile(`带[带我].|带一下|带个`),
	"上车": regexp.MustCompile(`上[车船]`),
}

func matchOne(text string, keywords []string) string {
	lower := strings.ToLower(text)
	for _, kw := range keywords {
		// 精确子串
		if strings.Contains(lower, strings.ToLower(kw)) {
			return kw
		}
		// 模糊变体
		if re, ok := fuzzyMap[kw]; ok && re.MatchString(text) {
			return kw
		}
	}
	return ""
}
