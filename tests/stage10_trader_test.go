// 阶段10 虚空商人/每日特惠/突击 三张图 + 数据查询黑盒测试。
// 覆盖：VoidTraders/DailyDeals/Sorties 的 Raw JSON 解析、节点/Boss/商品名翻译，
// 以及 draw 包三张图的非空绘制（对齐 Java WorldStateUtils / DefaultDraw*Image）。
package tests

import (
	"testing"

	"nyxbot-go/internal/database"
	"nyxbot-go/internal/draw"
	"nyxbot-go/internal/enum/drawplugin"
	modelwarframe "nyxbot-go/internal/model/warframe"
	"nyxbot-go/internal/warframe"
)

const traderSortieRaw = `{
  "VoidTraders": [{
    "_id":"t1","Activation":"2026-08-06T10:00:00Z","Expiry":"2026-08-08T10:00:00Z",
    "Character":"Baro'Ki Teel","Node":"/Node/SaturnRelay",
    "Manifest":[
      {"ItemType":"/Lotus/StoreItems/Weapons/Prisma/Grakata","PrimePrice":300,"RegularPrice":65000,"Limit":1},
      {"ItemType":"/Lotus/StoreItems/Mods/PrimySpore","PrimePrice":150,"RegularPrice":35000,"Limit":3}
    ]}],
  "DailyDeals": [{
    "Activation":"2026-08-07T10:00:00Z","Expiry":"2026-08-08T10:00:00Z",
    "StoreItem":"/Lotus/StoreItems/Weapons/ChromaPrime","OriginalPrice":200,"SalePrice":100,
    "Discount":50,"AmountTotal":20,"AmountSold":8}],
  "Sorties": [{
    "_id":"s1","Activation":"2026-08-07T10:00:00Z","Expiry":"2026-08-08T10:00:00Z",
    "Boss":"SORTIE_BOSS_HEK","Variants":[
      {"missionType":"MT_MOBILE_DEFENSE","modifierType":"SORTIE_MODIFIER_LOW_ENERGY","node":"/Node/RailDefend","tileset":""},
      {"missionType":"MT_EXTERMINATION","modifierType":"","node":"/Node/RailCleanup","tileset":""}]}]
}`

// TestGetVoidTraders 验证虚空商人解析：节点翻译、商品名翻译、价格字段透传。
func TestGetVoidTraders(t *testing.T) {
	setupStage10DB(t)
	if err := database.DB.Create(&modelwarframe.Nodes{
		UniqueName: "/Node/SaturnRelay",
		Name:       "土星 中继站",
		SystemName: "土星",
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.DB.Create(&modelwarframe.StateTranslation{
		UniqueName: "Weapons/Prisma/Grakata",
		Name:       "棱晶 葛拉克斯",
		Type:       0,
	}).Error; err != nil {
		t.Fatal(err)
	}
	warframe.DefaultWorldState().SetRaw([]byte(traderSortieRaw))

	traders, err := warframe.GetVoidTraders()
	if err != nil {
		t.Fatal(err)
	}
	if len(traders) != 1 {
		t.Fatalf("虚空商人数量 = %d, 期望 1", len(traders))
	}
	vt := traders[0]
	// 节点：建库键 /Node/SaturnRelay → 命中翻译为 "名称(系统名)"
	if vt.Node != "土星 中继站(土星)" {
		t.Fatalf("节点翻译失败, 实际 %q", vt.Node)
	}
	if len(vt.Manifest) != 2 {
		t.Fatalf("商品清单数量 = %d, 期望 2", len(vt.Manifest))
	}
	first := vt.Manifest[0]
	if first.Item != "棱晶 葛拉克斯" {
		t.Fatalf("商品名未翻译, 实际 %q", first.Item)
	}
	if first.PrimePrice == nil || *first.PrimePrice != 300 {
		t.Fatalf("杜卡币价格应为 300")
	}
	if first.RegularPrice == nil || *first.RegularPrice != 65000 {
		t.Fatalf("星币价格应为 65000")
	}
}

// TestGetDailyDeals 验证每日特惠解析：物品名翻译、折扣/库存/价格透传。
func TestGetDailyDeals(t *testing.T) {
	setupStage10DB(t)
	if err := database.DB.Create(&modelwarframe.StateTranslation{
		UniqueName: "StoreItems/Weapons/ChromaPrime",
		Name:       "Chroma Prime",
		Type:       0,
	}).Error; err != nil {
		t.Fatal(err)
	}
	raw := `{"DailyDeals":[{"Activation":"2026-08-07T10:00:00Z","Expiry":"2026-08-08T10:00:00Z",
	  "StoreItem":"/Lotus/StoreItems/Weapons/ChromaPrime","OriginalPrice":200,"SalePrice":100,
	  "Discount":0.5,"AmountTotal":20,"AmountSold":8}]}`
	warframe.DefaultWorldState().SetRaw([]byte(raw))

	deals, err := warframe.GetDailyDeals()
	if err != nil {
		t.Fatal(err)
	}
	if len(deals) != 1 {
		t.Fatalf("每日特惠数量 = %d, 期望 1", len(deals))
	}
	d := deals[0]
	if d.Item != "Chroma Prime" {
		t.Fatalf("物品名未翻译, 实际 %q", d.Item)
	}
	if d.Count == nil || *d.Count != 0.5 {
		t.Fatalf("折扣应为 0.5, 实际 %v", d.Count)
	}
	if d.OriginalPrice == nil || *d.OriginalPrice != 200 {
		t.Fatal("原价应为 200")
	}
	if d.SalePrice == nil || *d.SalePrice != 100 {
		t.Fatal("现价应为 100")
	}
	if d.Total == nil || *d.Total != 20 || d.Sold == nil || *d.Sold != 8 {
		t.Fatal("总/售库存字段错误")
	}
}

// TestGetSorties 验证突击解析：variants 节点翻译、Boss 中文映射、mission/modifier 枚举透传。
func TestGetSorties(t *testing.T) {
	setupStage10DB(t)
	if err := database.DB.Create(&modelwarframe.Nodes{
		UniqueName: "/Node/V1Defend",
		Name:       "金星 防御点",
		SystemName: "金星",
	}).Error; err != nil {
		t.Fatal(err)
	}
	raw := `{"Sorties":[{"_id":"s1","Activation":"2026-08-07T10:00:00Z","Expiry":"2026-08-08T10:00:00Z",
	  "Boss":"SORTIE_BOSS_HEK","Variants":[
	    {"missionType":"MT_MOBILE_DEFENSE","modifierType":"SORTIE_MODIFIER_LOW_ENERGY","node":"/Node/V1Defend"},
	    {"missionType":"MT_EXTERMINATION","modifierType":"","node":"/Node/None"} ]}]}`
	warframe.DefaultWorldState().SetRaw([]byte(raw))

	sorties, err := warframe.GetSorties()
	if err != nil {
		t.Fatal(err)
	}
	if len(sorties) != 1 {
		t.Fatalf("突击数量 = %d, 期望 1", len(sorties))
	}
	s := sorties[0]
	if s.Boss != "Councilor Vay Hek" {
		t.Fatalf("Boss 应映射为 Weapon ClubHek, 实际 %q", s.Boss)
	}
	if len(s.Variants) != 2 {
		t.Fatalf("阶段数量 = %d, 期望 2", len(s.Variants))
	}
	if s.Variants[0].ModifierType != drawplugin.ModLowEnergy {
		t.Fatalf("modifier 透传失败, 实际 %q", s.Variants[0].ModifierType)
	}
	if s.Variants[0].Node == "/Node/V1Defend" {
		t.Fatalf("阶段节点未翻译, 实际 %q", s.Variants[0].Node)
	}
	if s.Variants[0].MissionType != drawplugin.MTMobileDefense {
		t.Fatalf("任务类型透传失败")
	}
}

// TestDrawTraderImagesNonEmpty 验证三张图对构造 DTO 均能生成非空 PNG。
func TestDrawTraderImagesNonEmpty(t *testing.T) {
	setupStage10DB(t)
	if p := draw.DrawVoidTrader([]*draw.VoidTrader{
		{
			Node:     "中继站",
			Manifest: []*draw.VoidTraderItem{{Item: "武器", PrimePrice: intPtr(300), RegularPrice: intPtr(65000)}},
		},
	}); len(p) == 0 {
		t.Fatal("DrawVoidTrader 输出为空")
	}

	if p := draw.DrawDailyDeals(&draw.DailyDeals{
		Item: "物品", OriginalPrice: intPtr(200), SalePrice: intPtr(100),
		Count: floatPtr(0.5), Total: intPtr(20), Sold: intPtr(8),
	}); len(p) == 0 {
		t.Fatal("DrawDailyDeals 输出为空")
	}

	if p := draw.DrawSorties(&draw.Sortie{
		Boss: "Boss",
		Variants: []*draw.SortieVariant{
			{MissionType: drawplugin.MTExtermination, ModifierType: drawplugin.ModLowEnergy, Node: "节点A"},
			{MissionType: drawplugin.MTDefense, Node: "节点B"},
		},
	}); len(p) == 0 {
		t.Fatal("DrawSorties 输出为空")
	}
}

func floatPtr(v float64) *float64 { return &v }
