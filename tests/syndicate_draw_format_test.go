// 赏金奖励显示格式化与路径截断回归测试：
// 覆盖 |COUNT| 模板替换、内嵌数量去重（结尾 X N / 前缀数字）、干净条目数量追加与 x1 保留。
package tests

import (
	"testing"

	"nyxbot-go/internal/draw"
)

// TestFormatSyndicateReward 奖励显示文本规则（数据形态取自真实库 478 条奖励普查结果）。
func TestFormatSyndicateReward(t *testing.T) {
	cases := []struct {
		item  string
		count int
		want  string
	}{
		{"永冻晶矿 X |COUNT|", 100, "永冻晶矿 X 100"}, // 重导后的原始模板：替换后不追加
		{"伊莱体 X 15", 15, "伊莱体 X 15"},          // 内嵌结尾数量：去重
		{"赤毒 X 300", 300, "赤毒 X 300"},
		{"50 内融核心", 50, "50 内融核心"}, // 内嵌前缀数量：去重
		{"300内融核心", 300, "300内融核心"},
		{"1,500 现金匣", 1, "1,500 现金匣 x1"}, // 前缀 1,500 ≠ 数量 1，保留 x1
		{"2,500 现金匣", 1, "2,500 现金匣 x1"},
		{"控制模块", 2, "控制模块 x2"},       // 干净条目：追加数量
		{"夜灵晶体 蓝图", 1, "夜灵晶体 蓝图 x1"}, // 干净条目保留 x1
		{"前纪 E6 遗物", 1, "前纪 E6 遗物 x1"},
	}
	for _, c := range cases {
		if got := draw.FormatSyndicateReward(c.item, c.count); got != c.want {
			t.Errorf("FormatSyndicateReward(%q, %d) = %q, want %q", c.item, c.count, got, c.want)
		}
	}
}

// TestPathTail 路径文本渲染前取尾段；含 "/" 的译名与含空格的正常译文不受影响。
func TestPathTail(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/Lotus/Types/Gameplay/Eidolon/Jobs/Narmer/AssassinateBountyAss", "AssassinateBountyAss"},
		{"/Lotus/StoreItems/Types/Items/MiscItems/Forma", "Forma"},
		{"完成日历任务", "完成日历任务"},
		{"使用近战武器击杀200名科腐者敌人。", "使用近战武器击杀200名科腐者敌人。"},
		{"红/白/蓝", "红/白/蓝"}, // 含 "/" 的译名（真实库存在，如调色板）不能按路径截断
		{"A/B", "A/B"},     // 不以 "/" 开头的文本不是资源路径
		{"", ""},
	}
	for _, c := range cases {
		if got := draw.PathTail(c.in); got != c.want {
			t.Errorf("PathTail(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
