// 世界状态「真实载荷形态」回归测试。
//
// 背景：官方 https://api.warframe.com/cdn/worldState.php 返回 MongoDB 扩展 JSON
// （_id 为 {"$oid":...}、时间为 {"$date":{"$numberLong":"毫秒"}}），而早期 Go 结构体把
// 两者都声明成了 string，导致任何一次世界状态查询都会整体解析失败：
//
//	parse world state envelope failed: json: cannot unmarshal object into Go struct field
//	wsSortie.Sorties._id of type string
//
// 本文件用与线上载荷逐字段一致的片段（摘自真实快照）锁定该契约，避免再次回归。
package tests

import (
	"fmt"
	"testing"
	"time"

	"nyxbot-go/internal/draw"
	"nyxbot-go/internal/enum/drawplugin"
	"nyxbot-go/internal/warframe"
)

// 固定毫秒时间戳（对应 2026-08-07 前后的真实快照值）。
const (
	activeMissionActivationMS = int64(1786010822836)
	activeMissionExpiryMS     = int64(1786016957903)
	sortieExpiryMS            = int64(1786032000000)
	voidTraderExpiryMS        = int64(1786280400000)
	dailyDealExpiryMS         = int64(1786100400000)
	liteSoriteExpiryMS        = int64(1786320000000)
	syndicateExpiryMS         = int64(1786098900000)
)

// realPayloadRaw 返回真实形态的世界状态片段；liteSoriteExpiry 由调用方给出，
// 便于构造「剩余时间」这类依赖当前时刻的断言。
func realPayloadRaw(liteSoriteExpiry int64) string {
	return fmt.Sprintf(`{
  "ActiveMissions": [
    {"_id":{"$oid":"6a745cc691391b96d7084ea1"},"Region":2,"Seed":67733,
     "Activation":{"$date":{"$numberLong":"%d"}},
     "Expiry":{"$date":{"$numberLong":"%d"}},
     "Node":"SolNode61","MissionType":"MT_SABOTAGE","Modifier":"VoidT1"},
    {"_id":{"$oid":"6a745d0390814b3150ea34e1"},"Region":11,"Seed":16293,
     "Activation":{"$date":{"$numberLong":"%d"}},
     "Expiry":{"$date":{"$numberLong":"%d"}},
     "Node":"SolNode153","MissionType":"MT_RESCUE","Modifier":"VoidT4","Hard":true}
  ],
  "VoidStorms": [
    {"_id":{"$oid":"6a7456720a7996c9469a40da"},"Node":"CrewBattleNode522",
     "Activation":{"$date":{"$numberLong":"1786011603351"}},
     "Expiry":{"$date":{"$numberLong":"1786017003351"}},"ActiveMissionTier":"VoidT1"}
  ],
  "Sorties": [
    {"_id":{"$oid":"6a735a7e9b3962c8397ea095"},
     "Activation":{"$date":{"$numberLong":"1785945600000"}},
     "Expiry":{"$date":{"$numberLong":"%d"}},
     "Reward":"/Lotus/Types/Game/MissionDecks/SortieRewards","Seed":66513,
     "Boss":"SORTIE_BOSS_HEK","ExtraDrops":[],"Twitter":true,
     "Variants":[
       {"missionType":"MT_MOBILE_DEFENSE","modifierType":"SORTIE_MODIFIER_LOW_ENERGY","node":"SolNode24","tileset":"GrineerForestTileset"},
       {"missionType":"MT_INTEL","modifierType":"SORTIE_MODIFIER_ARMOR","node":"SolNode67","tileset":"GrineerAsteroidTileset"}
     ]}
  ],
  "LiteSorties": [
    {"_id":{"$oid":"6a6fd67ee89feb5ea7ff5594"},
     "Activation":{"$date":{"$numberLong":"1785715200000"}},
     "Expiry":{"$date":{"$numberLong":"%d"}},
     "Reward":"/Lotus/Types/Game/MissionDecks/ArchonSortieRewards","Seed":15262,
     "Boss":"SORTIE_BOSS_NIRA",
     "Missions":[{"missionType":"MT_INTEL","node":"SolNode73"}]}
  ],
  "Invasions": [
    {"_id":{"$oid":"6a7370c3c2efe3f061676b93"},"Faction":"FC_GRINEER","DefenderFaction":"FC_CORPUS",
     "Node":"SolNode102","Count":1199,"Goal":34000,"LocTag":"/Lotus/Language/Menu/GrineerInvasionGeneric",
     "Completed":false,"ChainID":{"$oid":"6a7103aa90bf205b80ebfb85"},
     "AttackerReward":{"countedItems":[{"ItemType":"/Lotus/Types/Items/Research/ChemComponent","ItemCount":3}]},
     "AttackerMissionInfo":{"seed":552280,"faction":"FC_CORPUS"},
     "DefenderReward":{"countedItems":[{"ItemType":"/Lotus/Types/Items/Research/EnergyComponent","ItemCount":3}]},
     "DefenderMissionInfo":{"seed":972449,"faction":"FC_GRINEER"},
     "Activation":{"$date":{"$numberLong":"1785951369998"}}}
  ],
  "VoidTraders": [
    {"_id":{"$oid":"5d1e07a0a38e4a4fdd7cefca"},
     "Activation":{"$date":{"$numberLong":"1786107600000"}},
     "Expiry":{"$date":{"$numberLong":"%d"}},
     "Character":"Baro'Ki Teel","Node":"SaturnHUB"}
  ],
  "DailyDeals": [
    {"StoreItem":"/Lotus/StoreItems/Powersuits/Infestation/Infestation",
     "Activation":{"$date":{"$numberLong":"1786006800000"}},
     "Expiry":{"$date":{"$numberLong":"%d"}},
     "Discount":40,"OriginalPrice":225,"SalePrice":135,"AmountTotal":100,"AmountSold":34}
  ],
  "SyndicateMissions": [
    {"_id":{"$oid":"6a7370c3c2efe3f061676b94"},"Tag":"CetusSyndicate",
     "Expiry":{"$date":{"$numberLong":"%d"}},"Nodes":["SolNode1"],
     "Jobs":[{"jobType":"/Lotus/Types/Levels/Jobs/CetusBounty1","rewards":"/Lotus/Rewards/Pool1",
              "masteryReq":3,"minEnemyLevel":5,"maxEnemyLevel":15,"xpAmounts":[1000],
              "endless":false,"isVault":false,"locationTag":"/Lotus/Types/Levels/Cetus"}]}
  ],
  "Alerts": [
    {"_id":{"$oid":"6a737a200000000000000000"},
     "Activation":{"$date":{"$numberLong":"1785952800000"}},
     "Expiry":{"$date":{"$numberLong":"1786557600000"}},
     "MissionInfo":{"location":"SolNode72","missionType":"MT_SABOTAGE","faction":"FC_GRINEER",
       "difficulty":1,"missionReward":{"credits":50000,
         "countedItems":[{"ItemType":"/Lotus/Types/Items/MiscItems/WaterFightBucks","ItemCount":350}]},
       "minEnemyLevel":1,"maxEnemyLevel":2},
     "Tag":"WaterFight"}
  ],
  "SeasonInfo": {
    "Activation":{"$date":{"$numberLong":"1775662200000"}},
    "Expiry":{"$date":{"$numberLong":"1786546800000"}},
    "AffiliationTag":"RadioLegionIntermission15Syndicate","Season":17,"Phase":0,"Params":"",
    "ActiveChallenges":[
      {"_id":{"$oid":"001800180000000000000247"},"Daily":true,"Challenge":"/Lotus/Types/Challenges/NW2",
       "Activation":{"$date":{"$numberLong":"1785801600000"}},"Expiry":{"$date":{"$numberLong":"1786060800000"}}},
      {"_id":{"$oid":"001800180000000000000239"},"Challenge":"/Lotus/Types/Challenges/NW1",
       "Activation":{"$date":{"$numberLong":"1785715200000"}},"Expiry":{"$date":{"$numberLong":"1786320000000"}}},
      {"_id":{"$oid":"001800180000000000000245"},"Challenge":"/Lotus/Types/Challenges/NW3",
       "Activation":{"$date":{"$numberLong":"1785715200000"}},"Expiry":{"$date":{"$numberLong":"1786320000000"}}}
    ]
  },
  "EndlessXpSchedule": [
    {"Activation":{"$date":{"$numberLong":"1785715200000"}},
     "Expiry":{"$date":{"$numberLong":"1786320000000"}},
     "CategoryChoices":[
       {"Category":"EXC_NORMAL","Choices":["Hydroid","Mirage","Limbo"]},
       {"Category":"EXC_HARD","Choices":["AckAndBrunt","Soma","Vasto"]}
     ]}
  ],
  "KnownCalendarSeasons": [
    {"Season":"CST_FALL","YearIteration":1,"Version":1,
     "UpgradeAvaliabilityRequirements":[],
     "Days":[
       {"day":95,"events":[{"type":"CET_CHALLENGE","challenge":"/Lotus/Types/Challenges/Calendar1999/CalendarKillEximusEasy"}]},
       {"day":99,"events":[{"type":"CET_UPGRADE","upgrade":"/Lotus/Upgrades/Calendar/Armor"}]}
     ]}
  ]
}`, activeMissionActivationMS, activeMissionExpiryMS, activeMissionActivationMS, activeMissionExpiryMS,
		sortieExpiryMS, liteSoriteExpiry, voidTraderExpiryMS, dailyDealExpiryMS, syndicateExpiryMS)
}

// TestWorldStateExtendedJSONSorties 验证突击：_id 对象、$date 时间与 Boss 翻译。
func TestWorldStateExtendedJSONSorties(t *testing.T) {
	setupStage10DB(t)
	warframe.DefaultWorldState().SetRaw([]byte(realPayloadRaw(liteSoriteExpiryMS)))

	sorties, err := warframe.GetSorties()
	if err != nil {
		t.Fatalf("突击解析失败（真实载荷不应报错）: %v", err)
	}
	if len(sorties) != 1 {
		t.Fatalf("突击数量 = %d, 期望 1", len(sorties))
	}
	if sorties[0].Boss != "Councilor Vay Hek" {
		t.Fatalf("Boss 应翻译为 Councilor Vay Hek, 实际 %q", sorties[0].Boss)
	}
	if want := time.UnixMilli(sortieExpiryMS); !sorties[0].Expiry.Equal(want) {
		t.Fatalf("Expiry = %v, 期望 %v", sorties[0].Expiry, want)
	}
	if len(sorties[0].Variants) != 2 {
		t.Fatalf("Variants 数量 = %d, 期望 2", len(sorties[0].Variants))
	}
	if sorties[0].Variants[0].ModifierType != drawplugin.ModifierType("SORTIE_MODIFIER_LOW_ENERGY") {
		t.Fatalf("ModifierType 透传失败, 实际 %v", sorties[0].Variants[0].ModifierType)
	}
}

// TestWorldStateExtendedJSONActiveMissions 验证裂隙：时间毫秒换算与钢铁模式过滤。
func TestWorldStateExtendedJSONActiveMissions(t *testing.T) {
	setupStage10DB(t)
	warframe.DefaultWorldState().SetRaw([]byte(realPayloadRaw(liteSoriteExpiryMS)))

	normal, err := warframe.GetActiveMissions(false)
	if err != nil {
		t.Fatalf("裂隙解析失败: %v", err)
	}
	if len(normal) != 1 {
		t.Fatalf("普通裂隙数量 = %d, 期望 1", len(normal))
	}
	if normal[0].ID != "6a745cc691391b96d7084ea1" {
		t.Fatalf("$oid 未解析为字符串 ID, 实际 %q", normal[0].ID)
	}
	if want := time.UnixMilli(activeMissionExpiryMS); !normal[0].Expiry.Equal(want) {
		t.Fatalf("Expiry = %v, 期望 %v", normal[0].Expiry, want)
	}
	if normal[0].Modifier != drawplugin.VoidT1 {
		t.Fatalf("Modifier = %v, 期望 VoidT1", normal[0].Modifier)
	}

	steel, err := warframe.GetActiveMissions(true)
	if err != nil {
		t.Fatalf("钢铁裂隙解析失败: %v", err)
	}
	if len(steel) != 1 || !steel[0].Hard || steel[0].Modifier != drawplugin.VoidT4 {
		t.Fatalf("钢铁裂隙过滤失败: %+v", steel)
	}
}

// TestWorldStateExtendedJSONVoidStorms 验证九重天遗物等级取自 ActiveMissionTier。
func TestWorldStateExtendedJSONVoidStorms(t *testing.T) {
	setupStage10DB(t)
	warframe.DefaultWorldState().SetRaw([]byte(realPayloadRaw(liteSoriteExpiryMS)))

	storms, err := warframe.GetVoidStorms()
	if err != nil {
		t.Fatalf("九重天解析失败: %v", err)
	}
	if len(storms) != 1 {
		t.Fatalf("九重天数量 = %d, 期望 1", len(storms))
	}
	if storms[0].Modifier != drawplugin.VoidT1 {
		t.Fatalf("遗物等级应来自 ActiveMissionTier, 实际 %v", storms[0].Modifier)
	}
	if storms[0].ID != "6a7456720a7996c9469a40da" {
		t.Fatalf("$oid 未解析, 实际 %q", storms[0].ID)
	}
}

// TestWorldStateExtendedJSONInvasions 验证入侵：ChainID 对象、AttackerReward 单对象形态。
func TestWorldStateExtendedJSONInvasions(t *testing.T) {
	setupStage10DB(t)
	warframe.DefaultWorldState().SetRaw([]byte(realPayloadRaw(liteSoriteExpiryMS)))

	invasions, err := warframe.GetInvasions()
	if err != nil {
		t.Fatalf("入侵解析失败: %v", err)
	}
	if len(invasions) != 1 {
		t.Fatalf("入侵数量 = %d, 期望 1", len(invasions))
	}
	m := invasions[0]
	if m.ChainID != "6a7103aa90bf205b80ebfb85" {
		t.Fatalf("ChainID 未从 $oid 解析, 实际 %q", m.ChainID)
	}
	if !m.Activation.Equal(time.UnixMilli(1785951369998)) {
		t.Fatalf("Activation = %v, 期望 %v", m.Activation, time.UnixMilli(1785951369998))
	}
	// 载荷中 AttackerReward 是单个对象，Java 侧靠 ACCEPT_SINGLE_VALUE_AS_ARRAY 读成单元素列表
	if len(m.AttackerReward) != 1 {
		t.Fatalf("进攻方奖励应折叠为 1 项, 实际 %d", len(m.AttackerReward))
	}
	if len(m.AttackerReward[0].CountedItems) != 1 {
		t.Fatalf("进攻方奖励应含 1 个带数量物品")
	}
	if m.DefenderReward == nil || len(m.DefenderReward.CountedItems) != 1 {
		t.Fatalf("防守方奖励解析失败: %+v", m.DefenderReward)
	}
}

// TestWorldStateExtendedJSONTraderAndDeals 验证奸商与每日特惠的时间字段。
func TestWorldStateExtendedJSONTraderAndDeals(t *testing.T) {
	setupStage10DB(t)
	warframe.DefaultWorldState().SetRaw([]byte(realPayloadRaw(liteSoriteExpiryMS)))

	traders, err := warframe.GetVoidTraders()
	if err != nil {
		t.Fatalf("奸商解析失败: %v", err)
	}
	if len(traders) != 1 {
		t.Fatalf("奸商数量 = %d, 期望 1", len(traders))
	}
	if want := time.UnixMilli(voidTraderExpiryMS); !traders[0].Expiry.Equal(want) {
		t.Fatalf("奸商 Expiry = %v, 期望 %v", traders[0].Expiry, want)
	}

	deals, err := warframe.GetDailyDeals()
	if err != nil {
		t.Fatalf("每日特惠解析失败: %v", err)
	}
	if len(deals) != 1 {
		t.Fatalf("每日特惠数量 = %d, 期望 1", len(deals))
	}
	if want := time.UnixMilli(dailyDealExpiryMS); !deals[0].Expiry.Equal(want) {
		t.Fatalf("特惠 Expiry = %v, 期望 %v", deals[0].Expiry, want)
	}
	if deals[0].Count == nil || *deals[0].Count != 40 {
		t.Fatalf("特惠折扣应为 40")
	}
}

// TestWorldStateExtendedJSONLiteSorite 验证执刑官猎杀：Expiry 用于计算剩余时间。
func TestWorldStateExtendedJSONLiteSorite(t *testing.T) {
	setupStage10DB(t)
	future := time.Now().Add(2 * time.Hour).UnixMilli()
	warframe.DefaultWorldState().SetRaw([]byte(realPayloadRaw(future)))

	list, err := warframe.GetLiteSorite()
	if err != nil {
		t.Fatalf("执刑官猎杀解析失败: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("执刑官猎杀数量 = %d, 期望 1", len(list))
	}
	if list[0].Boss != "Nira" {
		t.Fatalf("Boss 应翻译为 Nira, 实际 %q", list[0].Boss)
	}
	if list[0].Expiry == "" {
		t.Fatal("Expiry 剩余时间不应为空（$date 未解析）")
	}
	if len(list[0].Missions) != 1 || list[0].Missions[0].TypeName == "" {
		t.Fatalf("猎杀任务解析失败: %+v", list[0].Missions)
	}
}

// TestWorldStateExtendedJSONSyndicateBountyExpiry 验证平原周期取到真实赏金结束时间。
// 赏金结束时间解析失败时 syndicateBountyExpiry 会回退为「当前时间」，因此给一个未来的
// 赏金结束时间戳：解析成功时夜灵平原的当前状态结束时间会明显晚于 now，否则只剩回退值。
func TestWorldStateExtendedJSONSyndicateBountyExpiry(t *testing.T) {
	setupStage10DB(t)
	future := time.Now().Add(90 * time.Minute).UnixMilli()
	raw := fmt.Sprintf(`{"SyndicateMissions":[{"_id":{"$oid":"6a7370c3c2efe3f061676b94"},
	  "Tag":"CetusSyndicate","Expiry":{"$date":{"$numberLong":"%d"}},
	  "Nodes":["SolNode1"],"Jobs":[]}]}`, future)
	warframe.DefaultWorldState().SetRaw([]byte(raw))

	all, err := warframe.GetAllCycle()
	if err != nil {
		t.Fatalf("平原周期解析失败: %v", err)
	}
	if all.CetusCycle == nil {
		t.Fatal("夜灵平原周期缺失")
	}
	// 赏金结束时间 now+90m 属于白天（剩余 >3000s），Cetus 当前状态 ≈ now+40m。
	if !all.CetusCycle.Expiry.After(time.Now().Add(20 * time.Minute)) {
		t.Fatalf("夜灵平原 Expiry = %v 未基于赏金结束时间推算（疑似回退到当前时间）",
			all.CetusCycle.Expiry)
	}
}

// TestCollectExpiryTimestampsFromExtendedJSON 验证轮询间隔用的过期时间采集
// 能识别真实键名与 $date 形态（此前采集永远为空 → 轮询恒为最大间隔 10 分钟）。
func TestCollectExpiryTimestampsFromExtendedJSON(t *testing.T) {
	setupStage10DB(t)
	// 最近的未来过期时间取「5 分钟后」：延迟应为 5m - 30s 缓冲 ≈ 270s，
	// 明显区别于「采不到过期时间」时的最大间隔 600s。
	future := time.Now().Add(5 * time.Minute).UnixMilli()
	warframe.DefaultWorldState().SetRaw([]byte(realPayloadRaw(future)))

	delay, _ := warframe.DefaultWorldState().NextDelaySeconds(0)
	if delay < 200 || delay > 300 {
		t.Fatalf("轮询延迟 = %ds, 期望约 270s（采集失败会退化为 600s）", delay)
	}
}

// TestDrawSortiesFromExtendedJSON 验证真实载荷可直接走到绘图链路。
func TestDrawSortiesFromExtendedJSON(t *testing.T) {
	setupStage10DB(t)
	warframe.DefaultWorldState().SetRaw([]byte(realPayloadRaw(liteSoriteExpiryMS)))

	sorties, err := warframe.GetSorties()
	if err != nil {
		t.Fatal(err)
	}
	if len(sorties) == 0 {
		t.Fatal("突击数据为空")
	}
	img := draw.DrawSorties(sorties[0])
	if len(img) == 0 {
		t.Fatal("突击绘图结果为空")
	}
}
