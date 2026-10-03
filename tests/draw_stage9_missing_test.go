// internal/draw 绘图模块补全测试（阶段 9 遗留）：
// 覆盖七个此前缺失的绘图层导出函数（警报/双衍/钢铁奖励/集团/电波季节/执刑官/1999日历），
// 图片输出到项目根目录 temp/ 下
package tests

import (
	"image/color"
	"testing"

	"nyxbot-go/internal/draw"
	"nyxbot-go/internal/enum/drawplugin"
)

// missionColor 构造 color.RGBA（测试样本用）。
func missionColor(rgb uint32) color.RGBA {
	return color.RGBA{uint8(rgb >> 16), uint8(rgb >> 8), uint8(rgb), 0xff}
}

// TestDrawAlertsImage 警报：两列卡片网格（地点/剩余时间/任务类型/派系/奖励）。
func TestDrawAlertsImage(t *testing.T) {
	alerts := []*draw.Alert{
		{
			TimeLeft: "23m",
			MissionInfo: &draw.AlertMissionInfo{
				Location:    "Ceres, Gabii",
				MissionType: drawplugin.MTSurvival,
				Faction:     drawplugin.FactionGrineer,
				MissionReward: &draw.AlertReward{
					Credits: intPtr(1000),
					Items:   []string{"Orokin 催化剂 蓝图"},
				},
			},
		},
		{
			TimeLeft: "55m",
			MissionInfo: &draw.AlertMissionInfo{
				Location:    "Earth, Montes",
				MissionType: drawplugin.MTExtermination,
				Faction:     drawplugin.FactionCorpus,
				MissionReward: &draw.AlertReward{
					Credits: intPtr(2000),
					Items:   []string{"Forma 蓝图", "氮感震爆 蓝图"},
				},
			},
		},
		{
			TimeLeft: "1h 12m",
			MissionInfo: &draw.AlertMissionInfo{
				Location:    "Mars, Ara",
				MissionType: drawplugin.MTDefense,
				Faction:     drawplugin.FactionInfest,
				MissionReward: &draw.AlertReward{
					Credits: intPtr(300),
				},
			},
		},
	}
	w, h := assertPNG(t, "stage9_alert_alerts.png", draw.DrawAlerts(alerts))
	if w < 1000 || h < 500 {
		t.Fatalf("alerts image too small: %dx%d", w, h)
	}
}

// TestDrawDuviriCycleImage 双衍王境：情绪展示卡 + 普通/钢铁双列选择卡。
func TestDrawDuviriCycleImage(t *testing.T) {
	cyc := &draw.DuvalierCycle{
		State:    "愤怒",
		TimeLeft: "3h 20m",
		Choices: []draw.DuviriChoice{
			{Category: draw.DuviriNormal, Choices: []string{"宝剑", "枪械", "战甲", "近战", "骑乘", "生存", "防御"}},
			{Category: draw.DuviriHard, Choices: []string{"宝剑", "枪械", "战甲", "近战"}},
		},
	}
	w, h := assertPNG(t, "draw_duviri_cycle.png", draw.DrawDuviriCycle(cyc))
	if w < 800 || h < 400 {
		t.Fatalf("duviri image too small: %dx%d", w, h)
	}
}

// TestDrawSteelPathImage 钢铁奖励：三行文本信息。
func TestDrawSteelPathImage(t *testing.T) {
	sp := &draw.SteelPathOffering{
		CurrentReward: "利布缇斯 蓝图",
		NextReward:    "腐败 吸毒枪 蓝图",
		Remaining:     "6h",
	}
	w, h := assertPNG(t, "draw_steel_path.png", draw.DrawSteelPath(sp))
	if w < 1000 || h < 400 {
		t.Fatalf("steel path image too small: %dx%d", w, h)
	}
}

// TestDrawSyndicateImage 集团：节点视图 + 赏金任务三列卡片视图。
func TestDrawSyndicateImage(t *testing.T) {
	// 节点视图
	nodes := &draw.SyndicateMission{
		Tag:   &draw.SyndicateTag{Name: "英择谛"},
		Nodes: []string{"英择谛营地", "突尔跌平原", "地窟营地"},
	}
	w, h := assertPNG(t, "draw_syndicate_nodes.png", draw.DrawSyndicateImage(nodes))
	if w < 1000 || h < 200 {
		t.Fatalf("syndicate nodes image too small: %dx%d", w, h)
	}

	// 赏金任务三列视图
	jobs := &draw.SyndicateMission{
		Tag: &draw.SyndicateTag{Name: "英择谛"},
		Jobs: []*draw.SyndicateJob{
			{Type: "存档守护", MinLevel: 55, MaxLevel: 70, MasteryReq: 0,
				Desc: "在夜索尔哨站抵御破洞的来袭", Rewards: []*draw.SyndicateReward{
					{Rarity: drawplugin.RarityCommon, Item: "污染陀螺仪", ItemCount: 250},
					{Rarity: drawplugin.RarityRare, Item: "英择谛联合徽章", ItemCount: 400},
				}, XpAmounts: []int{1000}},
			{Type: "拦截", MinLevel: 60, MaxLevel: 75, MasteryReq: 2, Endless: true,
				Desc: "在近岸挡下英择谛的连续波次", Rewards: []*draw.SyndicateReward{
					{Rarity: drawplugin.RarityUncommon, Item: "纳什提亚 蓝图", ItemCount: 1},
				}, XpAmounts: []int{1200}},
			{Type: "资源回收", MinLevel: 60, MaxLevel: 75, MasteryReq: 2, IsVault: true,
				Desc: "回收被污染的开采设施", Rewards: []*draw.SyndicateReward{
					{Rarity: drawplugin.RarityLegendary, Item: "高等精英掉落", ItemCount: 1},
				}, XpAmounts: []int{1500}},
			{Type: "挖掘", MinLevel: 50, MaxLevel: 65,
				Desc: "在下方深处挖掘珍贵资源", Rewards: []*draw.SyndicateReward{
					{Rarity: drawplugin.RarityCommon, Item: "能量金属", ItemCount: 300},
				}, XpAmounts: []int{800}},
		},
	}
	w2, h2 := assertPNG(t, "draw_syndicate_jobs.png", draw.DrawSyndicateImage(jobs))
	if w2 < 1400 || h2 < 400 {
		t.Fatalf("syndicate jobs image too small: %dx%d", w2, h2)
	}
}

// TestDrawSeasonInfoImage 电波赛季：两列挑战卡片。
func TestDrawSeasonInfoImage(t *testing.T) {
	info := &draw.SeasonInfo{
		Season: 12,
		Phase:  3,
		ActiveChallenges: []*draw.ActiveChallenge{
			{Daily: true, Name: "完成 3 次移动防御任务", Description: "清除任意派系的移动防御任务 3 次", Standing: "3000"},
			{Daily: true, Name: "击杀 30 名敌人", Description: "用任意武器击杀 30 名敌人", Standing: "3000"},
			{Weekly: true, Name: "完成 5 次生存任务", Description: "完成任意节点上的生存任务 5 次（要求生存满 5 分钟）", Standing: "7000"},
			{Elite: true, Name: "完成夜灵狩猎", Description: "在夜灵狩猎中击杀夜灵丘陵幼灵", Standing: "45000"},
		},
	}
	w, h := assertPNG(t, "draw_Season_info.png", draw.DrawSeasonInfo(info))
	if w < 1500 || h < 600 {
		t.Fatalf("season info image too small: %dx%d", w, h)
	}
}

// TestDrawLiteSoriteImage 执刑官猎杀：Boss + 任务列表。
func TestDrawLiteSoriteImage(t *testing.T) {
	ls := &draw.LiteSorite{
		Boss:   "护盾强化",
		Expiry: "2026-08-08 00:00:00",
		Missions: []*draw.LiteSoriteMission{
			{TypeName: "暗杀", Node: "赛特斯·东寺", TypeColor: missionColor(0xff6b6b)},
			{TypeName: "生存", Node: "荒原·凯尔斯", TypeColor: missionColor(0xcda241)},
			{TypeName: "救援", Node: "枢纽·永光", TypeColor: missionColor(0x1dd1a1)},
		},
	}
	w, h := assertPNG(t, "draw_lite_sorite.png", draw.DrawLiteSorite(ls))
	if w < 800 || h < 500 {
		t.Fatalf("lite sorite image too small: %dx%d", w, h)
	}
}

// TestDrawKnownCalendarSeasonsImage 1999 日历：双列月卡片。
func TestDrawKnownCalendarSeasonsImage(t *testing.T) {
	list := []*draw.KnownCalendarSeasons{
		{
			Season:        "冬季",
			YearIteration: 2025,
			Version:       "1.0",
			MonthDays: map[int][]*draw.CalendarDay{
				12: {
					{Month: 12, Day: 8, Events: []*draw.CalendarEvent{
						{Type: draw.CETChallenge, Challenge: "今夜突涌"},
						{Type: draw.CETReward, Reward: "已解锁奖励"},
					}},
					{Month: 12, Day: 9, Events: []*draw.CalendarEvent{
						{Type: draw.CETUpgrade, Upgrade: "感染加成"},
					}},
				},
			},
		},
	}
	w, h := assertPNG(t, "draw_calendar_seasons.png", draw.DrawKnownCalendarSeasons(list))
	if w < 1200 || h < 500 {
		t.Fatalf("calendar image too small: %dx%d", w, h)
	}
}
