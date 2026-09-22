// 阶段10 世界状态裂隙/九重天指令黑盒测试。
// 通过默认缓存注入构造的 Raw JSON，验证 GetActiveMissions / GetVoidStorms 的
// 过滤、排序、翻译与字段透传（对齐 Java WorldStateUtils.getFissure）。
package tests

import (
	"strings"
	"testing"

	"nyxbot-go/internal/database"
	"nyxbot-go/internal/enum/drawplugin"
	modelwarframe "nyxbot-go/internal/model/warframe"
	"nyxbot-go/internal/warframe"
)

const fissureRaw = `{
  "activeMissions": [
    {"_id":"f1","Activation":"2026-08-07T10:00:00Z","Expiry":"2026-08-07T10:35:00Z",
     "MissionType":"MT_SURVIVAL","Modifier":"VoidT3","Node":"/Node/Abaddon",
     "Faction":"FC_CORPUS","Region":1,"Seed":11,"Hard":false,"voidStorms":false},
    {"_id":"f2","Activation":"2026-08-07T10:00:00Z","Expiry":"2026-08-07T10:35:00Z",
     "MissionType":"MT_EXTERMINATION","Modifier":"VoidT1","Node":"/Node/Abaddon",
     "Faction":"FC_GRINEER","Region":3,"Seed":22,"Hard":false,"voidStorms":false},
    {"_id":"f3","Activation":"2026-08-07T10:00:00Z","Expiry":"2026-08-07T10:35:00Z",
     "MissionType":"MT_CAPTURE","Modifier":"VoidT5","Node":"/Node/Milestone",
     "Faction":"FC_GRINEER","Region":1,"Seed":33,"Hard":true,"voidStorms":false},
    {"_id":"f4","Activation":"2026-08-07T10:00:00Z","Expiry":"2026-08-07T10:35:00Z",
     "MissionType":"MT_DEFENSE","Modifier":"VoidT2","Node":"/Node/Nightmare",
     "Faction":"FC_CORPUS","Region":5,"Seed":44,"Hard":false,"voidStorms":false}
  ],
  "voidStorms": [
    {"_id":"v1","Activation":"2026-08-07T10:00:00Z","Expiry":"2026-08-07T10:35:00Z",
     "Node":"/Node/Nightmare","Tier":"VoidT4"}
  ]
}`

// TestGetActiveMissions 验证普通裂隙：过滤钢铁任务、按遗物等级升序、时间解析。
func TestGetActiveMissions(t *testing.T) {
	setupStage10DB(t)
	warframe.DefaultWorldState().SetRaw([]byte(fissureRaw))

	missions, err := warframe.GetActiveMissions(false)
	if err != nil {
		t.Fatal(err)
	}
	if len(missions) != 3 {
		t.Fatalf("普通裂隙数量 = %d, 期望 3（应排除钢铁 f3）", len(missions))
	}
	// 排序：按遗物等级 VoidT1(fl2) → VoidT2(f4) → VoidT3(f1)。
	wantOrder := []drawplugin.VoidTier{
		drawplugin.VoidT1, drawplugin.VoidT2, drawplugin.VoidT3,
	}
	for i := range missions {
		if missions[i].Modifier != wantOrder[i] {
			t.Fatalf("第 %d 项 Modifier = %v, 期望 %v", i, missions[i].Modifier, wantOrder[i])
		}
		if missions[i].Expiry.IsZero() {
			t.Fatalf("第 %d 项 Expiry 解析失败", i)
		}
	}
}

// TestGetSteelPath 验证钢铁裂隙只保留 hard=true 项。
func TestGetSteelPath(t *testing.T) {
	setupStage10DB(t)
	warframe.DefaultWorldState().SetRaw([]byte(fissureRaw))

	missions, err := warframe.GetActiveMissions(true)
	if err != nil {
		t.Fatal(err)
	}
	if len(missions) != 1 || missions[0].ID != "f3" {
		t.Fatalf("钢铁裂隙应仅含 f3, 实际 %d 项", len(missions))
	}
	if !missions[0].Hard {
		t.Fatal("钢铁裂隙项 Hard 应为 true")
	}
}

// TestGetVoidStorms 验证九重天：节点翻译、任务类型/派系取自 nodes 表、
// 未命中节点的派系保留 worldstate 自带值。
func TestGetVoidStorms(t *testing.T) {
	setupStage10DB(t)
	if err := database.DB.Create(&modelwarframe.Nodes{
		UniqueName:   "/Node/Nightmare",
		Name:         "赛德娜 梦魇",
		SystemName:   "赛德娜",
		FactionIndex: 3,
		MissionIndex: 2,
	}).Error; err != nil {
		t.Fatal(err)
	}
	warframe.DefaultWorldState().SetRaw([]byte(fissureRaw))

	storms, err := warframe.GetVoidStorms()
	if err != nil {
		t.Fatal(err)
	}
	if len(storms) != 1 {
		t.Fatalf("九重天数量 = %d, 期望 1", len(storms))
	}
	m := storms[0]
	if !m.VoidStorms {
		t.Fatal("九重天 VoidStorms 应为 true")
	}
	if m.MissionType != drawplugin.MTSurvival {
		t.Fatalf("任务类型应来自 nodes 表 MissionIndex 2 → MT_SURVIVAL, 实际 %v", m.MissionType)
	}
	if m.Faction != drawplugin.FactionOrokin {
		t.Fatalf("派系应来自 nodes 表 FactionIndex 3 → FC_OROKIN, 实际 %v", m.Faction)
	}
	if !strings.Contains(m.Node, "梦魇") {
		t.Fatalf("节点未翻译为中文, 实际 %q", m.Node)
	}
}

// TestGetActiveMissionsNodeMiss 验证节点未命中时保留自建 Faction。
func TestGetActiveMissionsNodeMiss(t *testing.T) {
	setupStage10DB(t)
	raw := `{"activeMissions":[{"_id":"m","Activation":"2026-08-07T10:00:00Z",
	  "Expiry":"2026-08-07T10:35:00Z","MissionType":"MT_CAPTURE","Modifier":"VoidT1",
	  "Node":"/Node/NoneSuch","Faction":"FC_CORPUS","Hard":false,"voidStorms":false}]}`
	warframe.DefaultWorldState().SetRaw([]byte(raw))

	missions, err := warframe.GetActiveMissions(false)
	if err != nil {
		t.Fatal(err)
	}
	if len(missions) != 1 {
		t.Fatalf("数量 = %d, 期望 1", len(missions))
	}
	if missions[0].Node != "/Node/NoneSuch" {
		t.Fatalf("未命中的节点应保留原文, 实际 %q", missions[0].Node)
	}
	if missions[0].Faction != drawplugin.FactionCorpus {
		t.Fatalf("未命中节点派系应保留 worldstate 自带 FC_CORPUS, 实际 %v", missions[0].Faction)
	}
}

// TestGetInvasions 验证入侵：过滤已完成项、节点翻译、奖励物品名翻译。
func TestGetInvasions(t *testing.T) {
	setupStage10DB(t)
	if err := database.DB.Create(&modelwarframe.Nodes{
		UniqueName: "/Node/Abaddon",
		Name:       "海王星 阿巴顿",
		SystemName: "海王星",
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.DB.Create(&modelwarframe.StateTranslation{
		UniqueName: "StoreItems/Weapons/ChromaPrime",
		Name:       "Chroma Prime",
		Type:       0,
	}).Error; err != nil {
		t.Fatal(err)
	}
	raw := `{"invasions":[
	  {"_id":"i1","Activation":"2026-08-07T10:00:00Z","Expiry":"2026-08-07T11:00:00Z",
	   "Faction":"FC_GRINEER","DefenderFaction":"FC_CORPUS","Node":"/Node/Abaddon",
	   "Count":-1200,"Goal":2500,"LocTag":"Vast Spiral","Completed":false,"ChainID":"c1",
	   "AttackerReward":[{"credits":0,"xp":0,"items":[],"countedItems":[
	     {"ItemType":"/Lotus/StoreItems/Weapons/ChromaPrime","ItemCount":1}]}],
	   "DefenderReward":{"credits":20000,"xp":0,"items":["Foo"],"countedItems":[]}},
	  {"_id":"i2","Activation":"2026-08-07T10:00:00Z","Expiry":"2026-08-07T11:00:00Z",
	   "Faction":"FC_CORPUS","DefenderFaction":"FC_GRINEER","Node":"/Node/None",
	   "Count":800,"Goal":1000,"LocTag":"","Completed":true,"ChainID":"c2",
	   "AttackerReward":null,"DefenderReward":{"credits":0,"xp":0,"items":[],"countedItems":[]}}]}`
	warframe.DefaultWorldState().SetRaw([]byte(raw))

	invasions, err := warframe.GetInvasions()
	if err != nil {
		t.Fatal(err)
	}
	if len(invasions) != 1 {
		t.Fatalf("入侵数量 = %d, 期望 1（应排除已完成 i2）", len(invasions))
	}
	m := invasions[0]
	if m.ID != "i1" {
		t.Fatalf("应为 i1, 实际 %q", m.ID)
	}
	if !strings.Contains(m.Node, "阿巴顿") {
		t.Fatalf("节点未翻译为中文, 实际 %q", m.Node)
	}
	if m.Goal == nil || *m.Goal != 2500 {
		t.Fatalf("Goal 应为 2500")
	}
	if len(m.AttackerReward) != 1 || len(m.AttackerReward[0].CountedItems) != 1 {
		t.Fatalf("进攻方奖励应包含 1 个带数量物品")
	}
	item := m.AttackerReward[0].CountedItems[0]
	if item.Name != "Chroma Prime" {
		t.Fatalf("奖励物品名应翻译为 Chroma Prime, 实际 %q", item.Name)
	}
	if item.Count == nil || *item.Count != 1 {
		t.Fatalf("物品数量应为 1")
	}
	if m.DefenderReward == nil || m.DefenderReward.Credits != 20000 {
		t.Fatalf("防守方奖励现金应为 20000")
	}
}

// TestMissionIndexMappings 验证 nodes 表任务类型序号 → 枚举的全部尾段映射（对齐 Java getMissionType）。
func TestMissionIndexMappings(t *testing.T) {
	setupStage10DB(t)
	mapping := map[int]drawplugin.MissionType{
		0:   drawplugin.MTAssassination,
		2:   drawplugin.MTSurvival,
		60:  drawplugin.MTSkirmish,
		61:  drawplugin.MTVolatile,
		62:  drawplugin.MTOrpheus,
		90:  drawplugin.MTAscension,
		100: drawplugin.MTRelay,
	}
	for index, want := range mapping {
		raw := `{"voidStorms":[{"_id":"s","Activation":"2026-08-07T10:00:00Z","Expiry":"2026-08-07T10:35:00Z","Node":"/N","Tier":"VoidT1"}]}`
		warframe.DefaultWorldState().SetRaw([]byte(raw))
		if err := database.DB.Delete(&modelwarframe.Nodes{}, "unique_name = ?", "/N").Error; err != nil {
			t.Fatal(err)
		}
		if err := database.DB.Create(&modelwarframe.Nodes{
			UniqueName:   "/N",
			Name:         "节点",
			SystemName:   "星系",
			FactionIndex: 0,
			MissionIndex: index,
		}).Error; err != nil {
			t.Fatal(err)
		}
		storms, err := warframe.GetVoidStorms()
		if err != nil {
			t.Fatal(err)
		}
		if len(storms) != 1 || storms[0].MissionType != want {
			t.Fatalf("missionIndex=%d 应映射 %v, 实际 %v", index, want, storms[0].MissionType)
		}
	}
}
