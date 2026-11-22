package intent

import "testing"

func TestMatch(t *testing.T) {
	helpTypes := []string{"排队", "帮帮", "带带", "求带", "上车"}
	servers := []string{"B服", "官服"}

	tests := []struct {
		text         string
		wantHelpType string
		wantServer   string
		wantMatch    bool
	}{
		// 精确匹配
		{"排队，B服", "排队", "B服", true},
		{"排队", "排队", "", true},
		{"帮帮，官服", "帮帮", "官服", true},
		// 模糊匹配
		{"好的，那我排一排", "排队", "", true},
		{"排个队，B服", "排队", "B服", true},
		{"排一下", "排队", "", true},
		{"帮我一下，官服", "帮帮", "官服", true},
		{"带带我，B服", "带带", "B服", true},
		// 兜底
		{"排队", "排队", "", true},
		{"我排队，b服[妙]", "排队", "B服", true},
		// 不匹配（否定/疑问）
		{"怎么排队啊", "", "", false},
		{"有人排队吗", "", "", false},
		{"这游戏排队系统真烂", "", "", false},
		// 不匹配（无关）
		{"今天天气不错", "", "", false},
	}

	for _, tt := range tests {
		ht, sv, matched := Match(tt.text, helpTypes, servers)
		if matched != tt.wantMatch {
			t.Errorf("Match(%q) matched=%v, want %v", tt.text, matched, tt.wantMatch)
		}
		if ht != tt.wantHelpType {
			t.Errorf("Match(%q) helpType=%q, want %q", tt.text, ht, tt.wantHelpType)
		}
		if sv != tt.wantServer {
			t.Errorf("Match(%q) server=%q, want %q", tt.text, sv, tt.wantServer)
		}
	}
}

func TestMatchRegressions(t *testing.T) {
	help := []string{"", "排队", "帮帮", "带带", "求带", "上车"}
	servers := []string{"", "B服", "官服", "官服一区"}
	tests := []struct {
		text, help, server string
		matched            bool
	}{
		{"天气真好", "", "", false},
		{"取消排队", "", "", false},
		{"我不想排队了", "", "", false},
		{"我已经排队了", "", "", false},
		{"怎么排队", "", "", false},
		{"排队有多少人", "", "", false},
		{"有人排队吗", "", "", false},
		{"我可以排队吗？", "排队", "", true},
		{"我能排个队吗", "排队", "", true},
		{"麻烦帮帮我", "帮帮", "", true},
		{"带我一下，ｂ服", "带带", "B服", true},
		{"排下 官服一区", "排队", "官服一区", true},
		{"排队 官服", "排队", "官服", true},
		{"排除故障", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			h, s, ok := Match(tt.text, help, servers)
			if h != tt.help || s != tt.server || ok != tt.matched {
				t.Errorf("got (%q,%q,%v), want (%q,%q,%v)", h, s, ok, tt.help, tt.server, tt.matched)
			}
		})
	}
}

func TestMatchEmptyKeywordsAndLiteralPriority(t *testing.T) {
	h, _, ok := Match("排队", []string{"", "排队"}, nil)
	if !ok || h != "排队" {
		t.Fatalf("empty keyword matched: %q, %v", h, ok)
	}
	h, _, ok = Match("求带", []string{"带带", "求带"}, nil)
	if !ok || h != "求带" {
		t.Fatalf("literal lost to fuzzy match: %q, %v", h, ok)
	}
}
