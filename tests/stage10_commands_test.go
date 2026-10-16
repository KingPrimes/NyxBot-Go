// 阶段10 Warframe Bot 指令转换层测试：
// 覆盖 仲裁/遗物 DTO 转换、节点/物品翻译（临时 sqlite）、绘图链路非空
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
	modelwarframe "nyxbot-go/internal/model/warframe"
	"nyxbot-go/internal/warframe"
)

// setupStage10DB 初始化临时 sqlite，迁移翻译与遗物表并置入 global DB。
func setupStage10DB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "stage10.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&modelwarframe.Nodes{},
		&modelwarframe.ArbitrationTier{},
		&modelwarframe.StateTranslation{},
		&modelwarframe.Relics{},
		&modelwarframe.RelicRewards{},
		&modelwarframe.MissionSubscribe{},
		&modelwarframe.MissionSubscribeUser{},
		&modelwarframe.MissionSubscribeUserCheckType{},
	); err != nil {
		t.Fatal(err)
	}
	database.DB = db
	t.Cleanup(func() { closeGlobalDB(t) })
}

// TestArbitrationToDraw 验证仲裁领域数据 → 绘图 DTO：Unix 秒换算为时间，
// 节点名（含星系）/ 任务类型 / 评级由 nodes 与 arbitration_tier 表在转换时补齐。
func TestArbitrationToDraw(t *testing.T) {
	setupStage10DB(t)
	if err := database.DB.Create(&modelwarframe.Nodes{
		UniqueName: "SolNode64", Name: "Umbriel", SystemName: "天王星", MissionIndex: 13, FactionIndex: 7,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.DB.Create(&modelwarframe.ArbitrationTier{Key: "SolNode64", Tier: "S"}).Error; err != nil {
		t.Fatal(err)
	}

	start := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	dto := warframe.ArbitrationToDraw(warframe.Arbitration{
		Activation: start.Unix(),
		NodeKey:    "SolNode64",
		Expiry:     start.Add(time.Hour).Unix(),
		Faction:    drawplugin.FactionMurmur,
	})
	if dto == nil {
		t.Fatal("ArbitrationToDraw returned nil")
	}
	if dto.Node != "Umbriel(天王星)" {
		t.Fatalf("节点名应为「名称(星系)」格式, got %q", dto.Node)
	}
	if dto.MissionName != "拦截" {
		t.Fatalf("任务类型应由 nodes.mission_index 补齐, got %q", dto.MissionName)
	}
	if dto.Faction != drawplugin.FactionMurmur {
		t.Fatalf("派系应直传枚举, got %q", dto.Faction)
	}
	if dto.Tier != "S" {
		t.Fatalf("评级应由 arbitration_tier 表补齐, got %q", dto.Tier)
	}
	if !dto.Activation.Equal(start) || !dto.Expiry.Equal(start.Add(time.Hour)) {
		t.Fatalf("Unix 秒应换算为对应时间: act=%v exp=%v", dto.Activation, dto.Expiry)
	}
}

// TestArbitrationToDrawUnknownNode 节点未收录时评级回退 F、节点名回退原文，且不 panic。
func TestArbitrationToDrawUnknownNode(t *testing.T) {
	setupStage10DB(t)
	useUntranslatedFile(t)

	dto := warframe.ArbitrationToDraw(warframe.Arbitration{
		Activation: 1762261200,
		NodeKey:    "SolNode999",
		Expiry:     1762264800,
	})
	if dto.Node != "SolNode999" {
		t.Fatalf("未收录节点应回退原文, got %q", dto.Node)
	}
	if dto.Tier != "F" {
		t.Fatalf("未收录节点评级应为 F, got %q", dto.Tier)
	}
}

// TestRelicsToDrawRarityMapping 验证遗物奖励稀有度序数 → 枚举映射与数量指针。
func TestRelicsToDrawRarityMapping(t *testing.T) {
	relic := &modelwarframe.Relics{
		Name: "愚龙 后纪",
		RelicRewards: []modelwarframe.RelicRewards{
			{RewardName: "愚龙 后纪 蓝图", Rarity: 0, ItemCount: 2},
			{RewardName: "愚龙 后纪 机体", Rarity: 1, ItemCount: 4},
			{RewardName: "愚龙 后纪 系统", Rarity: 2, ItemCount: 3},
			{RewardName: "愚龙 后纪 头部", Rarity: 3, ItemCount: 1},
		},
	}
	dto := warframe.RelicsToDraw(relic)
	if dto == nil {
		t.Fatal("RelicsToDraw returned nil")
	}
	if dto.Name != "愚龙 后纪" || len(dto.Rewards) != 4 {
		t.Fatalf("relic mapping mismatch: %+v", dto)
	}
	want := []drawplugin.Rarity{
		drawplugin.RarityCommon,
		drawplugin.RarityUncommon,
		drawplugin.RarityRare,
		drawplugin.RarityLegendary,
	}
	for i, reward := range dto.Rewards {
		if reward.Rarity != want[i] {
			t.Errorf("reward[%d] rarity = %s, want %s", i, reward.Rarity, want[i])
		}
		if reward.ItemCount == nil || *reward.ItemCount != relic.RelicRewards[i].ItemCount {
			t.Errorf("reward[%d] itemCount mismatch", i)
		}
	}
	// 空奖励应生成空切片而非 nil 崩溃
	empty := warframe.RelicsToDraw(&modelwarframe.Relics{Name: "虚空 前纪"})
	if empty == nil || empty.Rewards == nil {
		t.Fatalf("empty relic should have non-nil rewards slice")
	}
}

// TestTranslateNodeAndStateName 验证节点/物品中文翻译（nodes + state_translation 表）。
func TestTranslateNodeAndStateName(t *testing.T) {
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

	node := warframe.TranslateNode("/Node/Abaddon")
	if node != "海王星 阿巴顿(海王星)" {
		t.Fatalf("TranslateNode = %q, want 海王星 阿巴顿(海王星)", node)
	}
	if miss := warframe.TranslateNode("/Node/Unknown"); miss != "/Node/Unknown" {
		t.Fatalf("TranslateNode miss should fallback, got %q", miss)
	}
	// 长 uniqueName 只取最后三段再匹配 state_translation（对齐 Java getLastThreeSegments）
	item := warframe.TranslateStateName("/Lotus/StoreItems/Weapons/ChromaPrime")
	if item != "Chroma Prime" {
		t.Fatalf("TranslateStateName = %q, want Chroma Prime", item)
	}
	if miss := warframe.TranslateStateName("/Lotus/Whatever/Missing"); miss != "/Lotus/Whatever/Missing" {
		t.Fatalf("TranslateStateName miss should fallback, got %q", miss)
	}
}

// TestDrawChainNonEmpty 验证转换后的 DTO 可直接生成非空 PNG（端到端链路）。
func TestDrawChainNonEmpty(t *testing.T) {
	base := time.Now().Add(-30 * time.Minute)
	arbDTO := warframe.ArbitrationToDraw(warframe.Arbitration{
		Activation: base.Unix(),
		Expiry:     base.Add(30 * time.Minute).Unix(),
		NodeKey:    "SolNode64",
		Faction:    drawplugin.FactionGrineer,
	})
	if image := draw.DrawArbitration(arbDTO); len(image) == 0 {
		t.Fatal("DrawArbitration returned empty image")
	}
	relicDTO := warframe.RelicsToDraw(&modelwarframe.Relics{
		Name: "愚龙 后纪",
		RelicRewards: []modelwarframe.RelicRewards{
			{RewardName: "愚龙 后纪 蓝图", Rarity: 0, ItemCount: 2},
		},
	})
	if image := draw.DrawRelics([]*draw.Relics{relicDTO}); len(image) == 0 {
		t.Fatal("DrawRelics returned empty image")
	}
}
