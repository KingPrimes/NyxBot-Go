// 阶段10 Warframe Bot 指令转换层测试：
// 覆盖 仲裁/遗物 DTO 转换、节点/物品翻译（临时 sqlite）、绘图链路非空
package tests

import (
	"encoding/json"
	"path/filepath"
	"strconv"
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

// TestArbitrationToDraw 验证仲裁领域数据 → 绘图 DTO 的字段透传与时间解析。
func TestArbitrationToDraw(t *testing.T) {
	start := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	end := start.Add(1 * time.Hour)
	arb := warframe.Arbitration{
		ID:         "arb-1",
		Activation: start.Format(time.RFC3339),
		Expiry:     end.Format(time.RFC3339),
		Node:       "谷神星 葛兰塔峡谷",
		Planet:     "谷神星",
		Enemy:      "Grineer",
		Type:       "拦截",
	}
	dto := warframe.ArbitrationToDraw(arb)
	if dto == nil {
		t.Fatal("ArbitrationToDraw returned nil")
	}
	if dto.ID != "arb-1" || dto.Node != "谷神星 葛兰塔峡谷" || dto.Planet != "谷神星" {
		t.Fatalf("field passthrough mismatch: %+v", dto)
	}
	if dto.Enemy != "Grineer" || dto.Type != "拦截" {
		t.Fatalf("enemy/type mismatch: %+v", dto)
	}
	if !dto.Activation.Equal(start) || !dto.Expiry.Equal(end) {
		t.Fatalf("time parsing mismatch: act=%v exp=%v", dto.Activation, dto.Expiry)
	}
	// 非法时间应解析为零值而非 panic
	bad := warframe.ArbitrationToDraw(warframe.Arbitration{Activation: "not-a-time", Expiry: ""})
	if !bad.Activation.IsZero() {
		t.Fatalf("invalid activation should be zero, got %v", bad.Activation)
	}
}

// TestArbitrationJSONFields 验证仲裁 API 原始 JSON 字段映射（任务类型为 missionType，ID 即开始时间戳）。
func TestArbitrationJSONFields(t *testing.T) {
	const raw = `[{"id":"1762261200","activation":"2025-11-04T13:00:00.000Z","startString":"-24m -54s",` +
		`"expiry":"2025-11-04T14:00:00.000Z","node":"Gabii (谷神星)","missionType":"生存",` +
		`"enemy":"Infestation","eta":"36m 6s","bounds":{"resourceBonus":0.35}}]`
	var list []warframe.Arbitration
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("expect 1 entry, got %d", len(list))
	}
	arb := list[0]
	if arb.Type != "生存" {
		t.Fatalf("missionType 未映射到 Type: %q", arb.Type)
	}
	if arb.Node != "Gabii (谷神星)" || arb.Enemy != "Infestation" {
		t.Fatalf("node/enemy 映射异常: %+v", arb)
	}
	// 用户约定：ID 就是任务开始的时间戳（Unix 秒），应与 activation 一致
	activation, err := time.Parse(time.RFC3339, arb.Activation)
	if err != nil {
		t.Fatal(err)
	}
	if strconv.FormatInt(activation.Unix(), 10) != arb.ID {
		t.Fatalf("id %q 与 activation %v 不一致", arb.ID, activation)
	}
}

// TestArbitrationIsWorth 验证「值得参与」判断（对齐 Java Arbitration.isWorth 三重匹配）。
func TestArbitrationIsWorth(t *testing.T) {
	cases := []struct {
		name string
		arb  warframe.Arbitration
		want bool
	}{
		{"拦截+谷神星+Grineer", warframe.Arbitration{Type: "拦截", Node: "Stöfler (谷神星)", Enemy: "Grineer"}, true},
		{"防御+水星+Infestation", warframe.Arbitration{Type: "防御", Node: "Sinai (水星)", Enemy: "Infestation"}, true},
		{"生存不算", warframe.Arbitration{Type: "生存", Node: "Gabii (谷神星)", Enemy: "Infestation"}, false},
		{"非谷神星/水星不算", warframe.Arbitration{Type: "防御", Node: "Coba (地球)", Enemy: "Infestation"}, false},
		{"Corpus 不算", warframe.Arbitration{Type: "防御", Node: "Sinai (水星)", Enemy: "Corpus"}, false},
		{"任务类型为空不算", warframe.Arbitration{Node: "Stöfler (谷神星)", Enemy: "Grineer"}, false},
	}
	for _, tc := range cases {
		if got := tc.arb.IsWorth(); got != tc.want {
			t.Errorf("%s: IsWorth()=%v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestArbitrationActivationFallbackToID 验证 activation 非法时回退 ID（Unix 秒时间戳）。
func TestArbitrationActivationFallbackToID(t *testing.T) {
	base := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	arb := warframe.Arbitration{
		ID:         strconv.FormatInt(base.Unix(), 10),
		Activation: "not-a-time",
		Expiry:     base.Add(time.Hour).Format(time.RFC3339),
	}
	dto := warframe.ArbitrationToDraw(arb)
	if !dto.Activation.Equal(base) {
		t.Fatalf("activation 应回退为 ID 时间戳 %v, got %v", base, dto.Activation)
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
		ID:         "a",
		Activation: base.Format(time.RFC3339),
		Expiry:     base.Add(30 * time.Minute).Format(time.RFC3339),
		Node:       "谷神星 葛兰塔峡谷",
		Planet:     "谷神星",
		Enemy:      "Grineer",
		Type:       "拦截",
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
