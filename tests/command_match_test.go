package tests

import (
	"regexp"
	"testing"

	"nyxbot-go/internal/enum/nyxbot"
)

// TestCommandPatternsRE2Compatible 验证全部指令正则可被 Go RE2 编译，
// 且 CodesOrder 与 CodesInfo 两个数据源保持同步。
// ZeroBot RegexRule 在注册时 regexp.MustCompile（非法模式直接 panic），
// 本测试把这一风险提前到 CI 阶段暴露。
func TestCommandPatternsRE2Compatible(t *testing.T) {
	if len(nyxbot.CodesOrder) != len(nyxbot.CodesInfo) {
		t.Fatalf("CodesOrder(%d) 与 CodesInfo(%d) 数量不一致，请同步新增指令", len(nyxbot.CodesOrder), len(nyxbot.CodesInfo))
	}
	seen := make(map[nyxbot.Codes]bool, len(nyxbot.CodesOrder))
	for _, code := range nyxbot.CodesOrder {
		seen[code] = true
		info, ok := nyxbot.CodesInfo[code]
		if !ok {
			t.Errorf("CodesInfo 缺少 %s 的条目", code)
			continue
		}
		if _, err := regexp.Compile(info.Comm); err != nil {
			t.Errorf("%s 的指令正则无法被 Go RE2 编译: %v", code, err)
		}
	}
	for code := range nyxbot.CodesInfo {
		if !seen[code] {
			t.Errorf("CodesOrder 缺少 %s（/log/codes 选项将丢失该指令）", code)
		}
	}
}

// TestCommandMatchSemantics 按 ZeroBot RegexRule 的匹配语义（FindStringSubmatch，
// 等同于 Java shiro 的 Matcher.find）抽查指令正则的行为：
// 带 $ 锚定的是精确指令（不多匹配、大小写按枚举别名），无 $ 的是前缀指令（可携带参数）。
func TestCommandMatchSemantics(t *testing.T) {
	cases := []struct {
		code    nyxbot.Codes
		match   []string // 应匹配的消息
		noMatch []string // 不应匹配的消息
	}{
		{nyxbot.CmdHelp,
			[]string{"帮助", "指令", "命令", "菜单", "help", "HELP"},
			[]string{"帮助一下", "请帮助", "Help"}},
		{nyxbot.CmdWfAlerts,
			[]string{"警报"},
			[]string{"警报列表", "有警报吗"}},
		{nyxbot.CmdWfArbitration,
			[]string{"仲裁"},
			[]string{"仲裁表"}},
		{nyxbot.CmdWfArbitrationEx,
			[]string{"仲裁表"},
			[]string{"仲裁"}},
		{nyxbot.CmdUpdateWfResMarketItems,
			[]string{"更新WM物品"},
			[]string{"更新WM物品列表"}},
		{nyxbot.CmdWfSubscribe,
			[]string{"订阅", "订阅 警报"},
			[]string{"请订阅", "取消订阅"}},
		{nyxbot.CmdWfUnsubscribe,
			[]string{"取消订阅", "取消订阅 警报"},
			[]string{"订阅"}},
		{nyxbot.CmdWfSisters,
			[]string{"信条", "信条 月", "XT", "xt", "/XT", "XT123"},
			[]string{"紫卡信条", "AT"}},
		{nyxbot.CmdWfRelics,
			[]string{"核桃", "查核桃", "核桃 虚空"},
			[]string{"吃核桃"}},
	}

	for _, tc := range cases {
		re, err := regexp.Compile(nyxbot.CodesInfo[tc.code].Comm)
		if err != nil {
			t.Fatalf("%s 正则编译失败: %v", tc.code, err)
		}
		for _, msg := range tc.match {
			if re.FindStringSubmatch(msg) == nil {
				t.Errorf("%s 应匹配 %q 但未命中（正则 %s）", tc.code, msg, nyxbot.CodesInfo[tc.code].Comm)
			}
		}
		for _, msg := range tc.noMatch {
			if re.FindStringSubmatch(msg) != nil {
				t.Errorf("%s 不应匹配 %q 但命中了（正则 %s）", tc.code, msg, nyxbot.CodesInfo[tc.code].Comm)
			}
		}
	}
}
