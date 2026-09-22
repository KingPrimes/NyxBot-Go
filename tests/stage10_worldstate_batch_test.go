// 阶段10 世界状态批次 A-D 黑盒测试：
// 覆盖警报 / 执刑官猎杀 / 钢铁奖励 / 平原周期 / 三赏金 / 双衍轮换 / 电波 / 1999 日历
// 的 WorldState 解析、翻译与绘图链路，以及订阅的退群级联清理。
package tests

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"nyxbot-go/internal/database"
	"nyxbot-go/internal/draw"
	"nyxbot-go/internal/enum/drawplugin"
	modelbot "nyxbot-go/internal/model/bot"
	modelwarframe "nyxbot-go/internal/model/warframe"
	"nyxbot-go/internal/warframe"
)

// setupWorldStateBatchDB 初始化临时 sqlite，迁移本批次用到的表并置入全局 DB。
func setupWorldStateBatchDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "ws-batch.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&modelwarframe.Nodes{},
		&modelwarframe.StateTranslation{},
		&modelwarframe.RewardPool{},
		&modelwarframe.Reward{},
		&modelwarframe.NightWave{},
		&modelwarframe.Weapons{},
		&modelwarframe.MissionSubscribe{},
		&modelwarframe.MissionSubscribeUser{},
		&modelwarframe.MissionSubscribeUserCheckType{},
		&modelbot.BotAdmin{}, &modelbot.GroupWhite{}, &modelbot.ProveWhite{},
		&modelbot.GroupBlack{}, &modelbot.ProveBlack{},
	); err != nil {
		t.Fatal(err)
	}
	database.DB = db
	t.Cleanup(func() { closeGlobalDB(t) })
}

// TestGetAlerts 验证警报解析：节点翻译、派系由节点表推导、奖励星币与物品名翻译。
func TestGetAlerts(t *testing.T) {
	setupWorldStateBatchDB(t)
	if err := database.DB.Create(&modelwarframe.Nodes{
		UniqueName: "/Lotus/Types/Levels/Alert1", Name: "土星 中继站", SystemName: "土星", FactionIndex: 0,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.DB.Create(&modelwarframe.StateTranslation{
		UniqueName: "/Lotus/StoreItems/Weapons/Prisma/Grakata", Name: "棱晶 葛拉克斯", Type: 0,
	}).Error; err != nil {
		t.Fatal(err)
	}

	raw := `{"Alerts":[
	  {"MissionInfo":{"location":"/Lotus/Types/Levels/Alert1","missionType":"MT_EXTERMINATION",
	    "faction":"FC_GRINEER",
	    "missionReward":{"credits":12000,"items":["/Lotus/StoreItems/Weapons/Prisma/Grakata","/Lotus/Unknown/Thing"]}}},
	  {"MissionInfo":null},
	  {}
	]}`
	warframe.DefaultWorldState().SetRaw([]byte(raw))

	alerts, err := warframe.GetAlerts()
	if err != nil {
		t.Fatal(err)
	}
	// 缺失 MissionInfo 的条目应被过滤
	if len(alerts) != 1 {
		t.Fatalf("警报数 = %d, 期望 1", len(alerts))
	}
	info := alerts[0].MissionInfo
	if info == nil {
		t.Fatal("MissionInfo 不应为空")
	}
	if info.Location != "土星 中继站(土星)" {
		t.Fatalf("节点未翻译: %q", info.Location)
	}
	if info.MissionType != drawplugin.MTExtermination {
		t.Fatalf("任务类型透传失败: %q", info.MissionType)
	}
	if info.Faction != drawplugin.FactionGrineer {
		t.Fatalf("派系应由节点表 factionIndex 推导为 Grineer, 实际 %q", info.Faction)
	}
	if info.MissionReward == nil {
		t.Fatal("奖励不应为空")
	}
	if info.MissionReward.Credits == nil || *info.MissionReward.Credits != 12000 {
		t.Fatalf("星币透传失败: %v", info.MissionReward.Credits)
	}
	want := []string{"棱晶 葛拉克斯", "/Lotus/Unknown/Thing"}
	if len(info.MissionReward.Items) != len(want) {
		t.Fatalf("奖励物品数 = %d, 期望 %d", len(info.MissionReward.Items), len(want))
	}
	for i, item := range info.MissionReward.Items {
		if item != want[i] {
			t.Fatalf("奖励物品[%d] = %q, 期望 %q", i, item, want[i])
		}
	}
}

// TestGetAlertsEmptyWhenWorldStateMissing 验证 WorldState 未就绪时返回错误而非空图。
func TestGetAlertsEmptyWhenWorldStateMissing(t *testing.T) {
	setupWorldStateBatchDB(t)
	warframe.DefaultWorldState().SetRaw(nil)
	if _, err := warframe.GetAlerts(); err == nil {
		t.Fatal("WorldState 未就绪时应返回错误")
	}
}

// TestGetLiteSorite 验证执刑官猎杀：Boss 中文映射、节点翻译、任务类型名与配色。
func TestGetLiteSorite(t *testing.T) {
	setupWorldStateBatchDB(t)
	if err := database.DB.Create(&modelwarframe.Nodes{
		UniqueName: "/Lotus/Types/Levels/Kuva1", Name: "赤毒要塞", SystemName: "赤毒要塞",
	}).Error; err != nil {
		t.Fatal(err)
	}
	raw := `{"LiteSorties":[{"_id":"ls1","Boss":"SORTIE_BOSS_HEK","Expiry":"2030-01-01T00:00:00Z",
	  "Missions":[{"missionType":"MT_EXTERMINATION","node":"/Lotus/Types/Levels/Kuva1"},
	              {"missionType":"MT_SURVIVAL","node":"/Lotus/Types/Levels/Unknown"}]}]}`
	warframe.DefaultWorldState().SetRaw([]byte(raw))

	list, err := warframe.GetLiteSorite()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("执刑官猎杀数 = %d, 期望 1", len(list))
	}
	ls := list[0]
	if ls.Boss != "Councilor Vay Hek" {
		t.Fatalf("Boss 映射失败: %q", ls.Boss)
	}
	if ls.Expiry == "" {
		t.Fatal("剩余时间不应为空")
	}
	if len(ls.Missions) != 2 {
		t.Fatalf("任务数 = %d, 期望 2", len(ls.Missions))
	}
	if ls.Missions[0].TypeName != "歼灭" {
		t.Fatalf("任务类型中文 = %q, 期望 歼灭", ls.Missions[0].TypeName)
	}
	if ls.Missions[0].Node != "赤毒要塞(赤毒要塞)" {
		t.Fatalf("节点未翻译: %q", ls.Missions[0].Node)
	}
	if ls.Missions[1].Node != "/Lotus/Types/Levels/Unknown" {
		t.Fatalf("未命中节点应回退原文: %q", ls.Missions[1].Node)
	}
	if ls.Missions[0].TypeColor != draw.MissionTypeColor(drawplugin.MTExtermination) {
		t.Fatal("任务类型配色应与 draw 包一致")
	}
}

// TestGetSteelPathRotation 验证钢铁奖励轮换：当前/下一个奖励非空且互不相同。
// 注意：stage10_worldstate_test.go 中已有的 TestGetSteelPath 实际测的是「钢铁裂隙」，故此处另起名。
func TestGetSteelPathRotation(t *testing.T) {
	sp := warframe.GetSteelPath()
	if sp == nil {
		t.Fatal("钢铁奖励不应为空")
	}
	if sp.CurrentReward == "" || sp.NextReward == "" {
		t.Fatalf("奖励名不应为空: %+v", sp)
	}
	if sp.CurrentReward == sp.NextReward {
		t.Fatalf("当前与下一个奖励不应相同: %q", sp.CurrentReward)
	}
	if sp.Remaining == "" {
		t.Fatal("剩余时间不应为空")
	}
	// 幂等：同一时刻连续调用结果一致
	if again := warframe.GetSteelPath(); again.CurrentReward != sp.CurrentReward || again.NextReward != sp.NextReward {
		t.Fatalf("同一时刻结果应一致: %+v vs %+v", sp, again)
	}
}

// TestGetAllCycle 验证平原周期：五项非空、状态取值合法、赏金结束时间来自 SyndicateMissions。
func TestGetAllCycle(t *testing.T) {
	setupWorldStateBatchDB(t)
	future := time.Now().Add(6 * time.Hour).Format(time.RFC3339)
	raw := `{"SyndicateMissions":[
	  {"Tag":"CetusSyndicate","Expiry":"` + future + `"},
	  {"Tag":"ZarimanSyndicate","Expiry":"` + future + `"}]}`
	warframe.DefaultWorldState().SetRaw([]byte(raw))

	all, err := warframe.GetAllCycle()
	if err != nil {
		t.Fatal(err)
	}
	if all == nil {
		t.Fatal("AllCycle 不应为空")
	}
	if all.EarthCycle == nil || all.CetusCycle == nil || all.CambionCycle == nil ||
		all.VallisCycle == nil || all.ZarimanCycle == nil {
		t.Fatalf("五项周期均不应为空: %+v", all)
	}
	if all.CetusCycle.State != "白昼" && all.CetusCycle.State != "夜晚" {
		t.Fatalf("夜灵平原状态非法: %q", all.CetusCycle.State)
	}
	if all.EarthCycle.State != "白昼" && all.EarthCycle.State != "夜晚" {
		t.Fatalf("地球状态非法: %q", all.EarthCycle.State)
	}
	if all.VallisCycle.State != "温暖" && all.VallisCycle.State != "寒冷" {
		t.Fatalf("奥布山谷状态非法: %q", all.VallisCycle.State)
	}
	if all.ZarimanCycle.State != "Grineer" && all.ZarimanCycle.State != "Corpus" {
		t.Fatalf("扎里曼状态非法: %q", all.ZarimanCycle.State)
	}
	if all.CambionCycle.Active != "FASS" && all.CambionCycle.Active != "VOME" {
		t.Fatalf("魔胎之境状态非法: %q", all.CambionCycle.Active)
	}
	// 赏金结束时间为未来 6 小时，剩余时间不应为空
	if all.CetusCycle.TimeLeft == "" {
		t.Fatal("夜灵平原剩余时间不应为空")
	}
}

// TestGetAllCycleWithoutWorldState 验证 WorldState 缺失时仍能产出本地可算的周期。
func TestGetAllCycleWithoutWorldState(t *testing.T) {
	setupWorldStateBatchDB(t)
	warframe.DefaultWorldState().SetRaw(nil)
	all, err := warframe.GetAllCycle()
	if err != nil {
		t.Fatal(err)
	}
	if all == nil || all.EarthCycle == nil || all.CetusCycle == nil {
		t.Fatalf("无 WorldState 时仍应返回周期: %+v", all)
	}
}

// TestGetSyndicate 验证三赏金：按 Tag 命中、任务类型翻译、奖励池关联、节点视图数据。
func TestGetSyndicate(t *testing.T) {
	setupWorldStateBatchDB(t)
	// 赏金任务按最后三段查 state_translation（对齐 Java StringUtils.getLastThreeSegments）
	if err := database.DB.Create(&modelwarframe.StateTranslation{
		UniqueName: "Levels/Jobs/CetusBounty1", Name: "赏金任务", Description: "清剿感染体", Type: 0,
	}).Error; err != nil {
		t.Fatal(err)
	}
	pool := modelwarframe.RewardPool{UniqueName: "/Lotus/Rewards/Pool1"}
	if err := database.DB.Create(&pool).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.DB.Create(&modelwarframe.Reward{
		ID: "r1", PoolID: pool.UniqueName, Item: "现金", Rarity: 1, ItemCount: 5000,
	}).Error; err != nil {
		t.Fatal(err)
	}

	raw := `{"SyndicateMissions":[
	  {"Tag":"CetusSyndicate","Nodes":["/Node/Cetus1","/Node/Cetus2"],"Jobs":[
	     {"jobType":"/Lotus/Types/Levels/Jobs/CetusBounty1","rewards":"/Lotus/Rewards/Pool1",
	      "masteryReq":3,"minEnemyLevel":5,"maxEnemyLevel":15,"xpAmounts":[1000],
	      "endless":false,"isVault":false,"locationTag":"/Lotus/Types/Levels/Cetus"}]},
	  {"Tag":"EntratiSyndicate","Jobs":[]},
	  {"Tag":"SolarisSyndicate","Nodes":["/Node/Venus1"],"Jobs":[]}]}`
	warframe.DefaultWorldState().SetRaw([]byte(raw))

	mission, err := warframe.GetSyndicate(warframe.SyndicateOstrons)
	if err != nil {
		t.Fatal(err)
	}
	if mission == nil || mission.Tag == nil || mission.Tag.Name != "Ostron" {
		t.Fatalf("集团标签错误: 期望 Ostron, 实际 %+v (tag=%+v)", mission, mission.Tag)
	}
	if len(mission.Nodes) != 2 {
		t.Fatalf("节点数 = %d, 期望 2", len(mission.Nodes))
	}
	if len(mission.Jobs) != 1 {
		t.Fatalf("赏金数 = %d, 期望 1", len(mission.Jobs))
	}
	job := mission.Jobs[0]
	if job.Type != "赏金任务" {
		t.Fatalf("任务类型未翻译: %q", job.Type)
	}
	if job.Desc != "清剿感染体" {
		t.Fatalf("任务描述未翻译: %q", job.Desc)
	}
	if job.MinLevel != 5 || job.MaxLevel != 15 || job.MasteryReq != 3 {
		t.Fatalf("等级/段位透传失败: %+v", job)
	}
	if len(job.Rewards) != 1 || job.Rewards[0].Item != "现金" || job.Rewards[0].ItemCount != 5000 {
		t.Fatalf("奖励池关联失败: %+v", job.Rewards)
	}
}

// TestGetSyndicateNoJobs 验证该集团无赏金时返回空结果（对齐 Java 的 jobs 过滤）。
func TestGetSyndicateNoJobs(t *testing.T) {
	setupWorldStateBatchDB(t)
	raw := `{"SyndicateMissions":[{"Tag":"EntratiSyndicate","Jobs":[]}]}`
	warframe.DefaultWorldState().SetRaw([]byte(raw))

	mission, err := warframe.GetSyndicate(warframe.SyndicateEntrati)
	if err != nil {
		t.Fatal(err)
	}
	if mission == nil || len(mission.Jobs) != 0 {
		t.Fatalf("无赏金时应返回空结果: %+v", mission)
	}
}

// TestGetDuviriCycle 验证双衍轮换：情绪本地推算、普通选项原样、钢铁选项按武器表译中文。
func TestGetDuviriCycle(t *testing.T) {
	setupWorldStateBatchDB(t)
	if err := database.DB.Create(&modelwarframe.Weapons{
		UniqueName: "/Lotus/Weapons/Sirocco", Name: "西若科", EnglishName: "Sirocco",
	}).Error; err != nil {
		t.Fatal(err)
	}
	raw := `{"EndlessXpSchedule":[{"CategoryChoices":[
	  {"Category":"EXC_NORMAL","Choices":["普通奖励A"]},
	  {"Category":"EXC_HARD","Choices":["Sirocco","UnknownWeapon"]}]}]}`
	warframe.DefaultWorldState().SetRaw([]byte(raw))

	dto, err := warframe.GetDuviriCycle()
	if err != nil {
		t.Fatal(err)
	}
	if dto.State == "" || dto.TimeLeft == "" {
		t.Fatalf("情绪与剩余时间不应为空: %+v", dto)
	}
	if len(dto.Choices) != 2 {
		t.Fatalf("选择分类数 = %d, 期望 2", len(dto.Choices))
	}
	if dto.Choices[0].Category != draw.DuviriNormal || dto.Choices[0].Choices[0] != "普通奖励A" {
		t.Fatalf("普通分类不应翻译: %+v", dto.Choices[0])
	}
	if dto.Choices[1].Category != draw.DuviriHard {
		t.Fatalf("钢铁分类映射失败: %+v", dto.Choices[1])
	}
	if dto.Choices[1].Choices[0] != "西若科" {
		t.Fatalf("钢铁选项未按武器表译中文: %q", dto.Choices[1].Choices[0])
	}
	if dto.Choices[1].Choices[1] != "UnknownWeapon" {
		t.Fatalf("未命中武器应回退原文: %q", dto.Choices[1].Choices[1])
	}
}

// TestGetSeasonInfo 验证电波：挑战按 night_wave 表补全名称/描述/声望，|COUNT| 占位替换。
func TestGetSeasonInfo(t *testing.T) {
	setupWorldStateBatchDB(t)
	if err := database.DB.Create(&modelwarframe.NightWave{
		UniqueName: "/Lotus/Types/Challenges/NW1", Name: "击杀敌人",
		Description: "击杀 |COUNT| 名敌人", Standing: 4500, Required: 30,
	}).Error; err != nil {
		t.Fatal(err)
	}
	raw := `{"SeasonInfo":{"Season":12,"Phase":1,"ActiveChallenges":[
	  {"Challenge":"/Lotus/Types/Challenges/NW1","Daily":true,"Weekly":false,"Elite":true},
	  {"Challenge":"/Lotus/Types/Challenges/Missing","Daily":false,"Weekly":true,"Elite":false}]}}`
	warframe.DefaultWorldState().SetRaw([]byte(raw))

	info, err := warframe.GetSeasonInfo()
	if err != nil {
		t.Fatal(err)
	}
	if info.Season != 12 || info.Phase != 1 {
		t.Fatalf("赛季/阶段透传失败: %+v", info)
	}
	if len(info.ActiveChallenges) != 2 {
		t.Fatalf("挑战数 = %d, 期望 2", len(info.ActiveChallenges))
	}
	first := info.ActiveChallenges[0]
	if first.Name != "击杀敌人" {
		t.Fatalf("挑战名未翻译: %q", first.Name)
	}
	if first.Description != "击杀 30 名敌人" {
		t.Fatalf("|COUNT| 占位未替换: %q", first.Description)
	}
	if first.Standing != "4500" {
		t.Fatalf("声望 = %q, 期望 4500", first.Standing)
	}
	if !first.Daily || first.Weekly || !first.Elite {
		t.Fatalf("标记位错误: %+v", first)
	}
	// 未命中 night_wave 的挑战只保留标记位
	second := info.ActiveChallenges[1]
	if second.Name != "" || !second.Weekly {
		t.Fatalf("未命中挑战应仅保留标记: %+v", second)
	}
}

// TestGetKnownCalendarSeasons 验证 1999 日历：一年中第几天换算月/日、按月分组、事件翻译。
func TestGetKnownCalendarSeasons(t *testing.T) {
	setupWorldStateBatchDB(t)
	if err := database.DB.Create(&modelwarframe.StateTranslation{
		UniqueName: "/Lotus/Challenges/Cal1", Name: "完成日历任务", Type: 0,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.DB.Create(&modelwarframe.StateTranslation{
		UniqueName: "/Lotus/Rewards/Cal1", Name: "<TAG>稀有奖励", Type: 0,
	}).Error; err != nil {
		t.Fatal(err)
	}
	// day=32 → 2 月 1 日；day=1 → 1 月 1 日
	raw := `{"KnownCalendarSeasons":[{"Season":"CST_WINTER","YearIteration":2025,"Version":7,"Days":[
	  {"day":32,"events":[{"type":"CET_CHALLENGE","challenge":"/Lotus/Challenges/Cal1"}]},
	  {"day":1,"events":[{"type":"CET_REWARD","reward":"/Lotus/Rewards/Cal1"}]}]}]}`
	warframe.DefaultWorldState().SetRaw([]byte(raw))

	list, err := warframe.GetKnownCalendarSeasons()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("季节数 = %d, 期望 1", len(list))
	}
	season := list[0]
	if season.Season != "冬季" {
		t.Fatalf("季节枚举未译中文: %q", season.Season)
	}
	if season.YearIteration != 2025 || season.Version != "7" {
		t.Fatalf("年份/版本透传失败: %+v", season)
	}
	jan := season.MonthDays[1]
	feb := season.MonthDays[2]
	if len(jan) != 1 || jan[0].Day != 1 {
		t.Fatalf("1 月分组错误: %+v", jan)
	}
	if len(feb) != 1 || feb[0].Day != 1 {
		t.Fatalf("2 月分组错误（day=32 应为 2 月 1 日）: %+v", feb)
	}
	if got := feb[0].Events[0]; got.Type != draw.CETChallenge || got.Challenge != "完成日历任务" {
		t.Fatalf("挑战事件翻译失败: %+v", got)
	}
	if got := jan[0].Events[0]; got.Type != draw.CETReward || got.Reward != "稀有奖励" {
		t.Fatalf("奖励事件应去除 <...> 标记: %+v", got)
	}
}

// TestWorldStateBatchDrawChain 验证批次 A-D 的转换结果都能产出非空图片。
func TestWorldStateBatchDrawChain(t *testing.T) {
	setupWorldStateBatchDB(t)

	if image := draw.DrawAlerts([]*draw.Alert{{
		MissionInfo: &draw.AlertMissionInfo{
			Location: "土星 中继站(土星)", MissionType: drawplugin.MTExtermination,
			Faction:       drawplugin.FactionGrineer,
			MissionReward: &draw.AlertReward{Credits: intPtr(12000), Items: []string{"棱晶 葛拉克斯"}},
		},
	}}); len(image) == 0 {
		t.Fatal("DrawAlerts 输出为空")
	}

	if image := draw.DrawLiteSorite(&draw.LiteSorite{
		Boss: "Councilor Vay Hek", Expiry: "2h 30m 0s",
		Missions: []*draw.LiteSoriteMission{
			{TypeName: "歼灭", TypeColor: draw.MissionTypeColor(drawplugin.MTExtermination), Node: "赤毒要塞"},
		},
	}); len(image) == 0 {
		t.Fatal("DrawLiteSorite 输出为空")
	}

	if image := draw.DrawSteelPath(warframe.GetSteelPath()); len(image) == 0 {
		t.Fatal("DrawSteelPath 输出为空")
	}

	all, err := warframe.GetAllCycle()
	if err != nil {
		t.Fatal(err)
	}
	if image := draw.DrawAllCycle(all); len(image) == 0 {
		t.Fatal("DrawAllCycle 输出为空")
	}

	if image := draw.DrawSyndicateImage(&draw.SyndicateMission{
		Tag:   &draw.SyndicateTag{Name: "Ostron"},
		Nodes: []string{"希图斯赏金1", "希图斯赏金2"},
	}); len(image) == 0 {
		t.Fatal("DrawSyndicateImage（节点视图）输出为空")
	}
	if image := draw.DrawSyndicateImage(&draw.SyndicateMission{
		Tag: &draw.SyndicateTag{Name: "Ostron"},
		Jobs: []*draw.SyndicateJob{{
			Type: "赏金任务", MinLevel: 5, MaxLevel: 15, XpAmounts: []int{1000},
			Rewards: []*draw.SyndicateReward{{Rarity: drawplugin.RarityUncommon, Item: "现金", ItemCount: 5000}},
		}},
	}); len(image) == 0 {
		t.Fatal("DrawSyndicateImage（赏金视图）输出为空")
	}

	if image := draw.DrawDuviriCycle(&draw.DuvalierCycle{
		State: "悲伤", TimeLeft: "1h 0m 0s",
		Choices: []draw.DuviriChoice{
			{Category: draw.DuviriNormal, Choices: []string{"普通A"}},
			{Category: draw.DuviriHard, Choices: []string{"钢铁A"}},
		},
	}); len(image) == 0 {
		t.Fatal("DrawDuviriCycle 输出为空")
	}

	if image := draw.DrawSeasonInfo(&draw.SeasonInfo{
		Season: 12, Phase: 1,
		ActiveChallenges: []*draw.ActiveChallenge{
			{Name: "击杀敌人", Description: "击杀 30 名敌人", Standing: "4500", Daily: true, Elite: true},
		},
	}); len(image) == 0 {
		t.Fatal("DrawSeasonInfo 输出为空")
	}

	if image := draw.DrawKnownCalendarSeasons([]*draw.KnownCalendarSeasons{{
		Season: "冬季", YearIteration: 2025, Version: "7",
		MonthDays: map[int][]*draw.CalendarDay{
			1: {{Month: 1, Day: 1, Events: []*draw.CalendarEvent{{Type: draw.CETChallenge, Challenge: "完成日历任务"}}}},
		},
	}}); len(image) == 0 {
		t.Fatal("DrawKnownCalendarSeasons 输出为空")
	}
}
