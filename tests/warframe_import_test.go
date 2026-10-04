// Warframe 数据导入修复回归测试：
// 覆盖 state_translation 名称字段解析/类型正则、遗物奖励共享主键、奖励名翻译、
// state_translation 后缀匹配（对齐 Java findByUniqueName）与遗物四级回退查询。
package tests

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"nyxbot-go/internal/database"
	"nyxbot-go/internal/logging"
	modelwarframe "nyxbot-go/internal/model/warframe"
	"nyxbot-go/internal/warframe"
)

// setupImportFixDB 初始化临时 sqlite 并迁移导入修复相关表，置入全局 DB。
func setupImportFixDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "import-fix.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&modelwarframe.Alias{},
		&modelwarframe.StateTranslation{},
		&modelwarframe.Relics{},
		&modelwarframe.RelicRewards{},
		&modelwarframe.Warframes{},
		&modelwarframe.WarframesAbility{},
	); err != nil {
		t.Fatal(err)
	}
	database.DB = db
	t.Cleanup(func() { closeGlobalDB(t) })
}

// TestParseStateTranslationExportUsesNameField 验证导出解析读取 name 字段（不是 englishName）、
// 跳过空名条目、并按 StateTypeEnum.KEY 正则整串匹配分类（对齐 Java parseFromExport）。
func TestParseStateTranslationExportUsesNameField(t *testing.T) {
	raw := []byte(`{"ExportWeapons":[
		{"uniqueName":"/Lotus/Weapons/Tenno/ChromaPrime","name":"Chroma Prime","description":""},
		{"uniqueName":"/Lotus/Weapons/Tenno/LegacyFallback","englishName":"Legacy Fallback","name":"","description":"d"},
		{"uniqueName":"/Lotus/Weapons/Tenno/NoName","name":"","description":""},
		{"uniqueName":"/Lotus/Types/Game/Projections/T3VoidProjectionASilver","name":"中纪 A 遗物","description":[]},
		{"uniqueName":"","name":"空 uniqueName","description":""}
	]}`)

	records := warframe.ParseStateTranslationExport(raw, "ExportWeapons", 8) // 8 = WEAPONS（来源默认类型）
	if len(records) != 3 {
		t.Fatalf("expect 3 records (空名与空 uniqueName 被过滤), got %d: %+v", len(records), records)
	}
	// 中文导出文件用 name；englishName 仅在 name 缺失时兜底
	if records[0].Name != "Chroma Prime" || records[0].Type != 8 {
		t.Fatalf("name/type mismatch: %+v", records[0])
	}
	if records[1].Name != "Legacy Fallback" || records[1].Type != 8 {
		t.Fatalf("englishName fallback mismatch: %+v", records[1])
	}
	// 遗物路径按 KEY 正则命中 RELIC_SILVER（序数 12），覆盖来源默认类型
	if records[2].Name != "中纪 A 遗物" || records[2].Type != 12 {
		t.Fatalf("relic type regex mismatch: %+v", records[2])
	}

	// 无匹配 key 时返回空
	if got := warframe.ParseStateTranslationExport(raw, "ExportWarframes", 7); len(got) != 0 {
		t.Fatalf("missing key should yield no records, got %d", len(got))
	}
}

// TestParseRelicExportKeepsSharedRewards 验证同一奖励物品被多个遗物共用时，每个遗物各自保留奖励行，
// 且奖励行主键全局唯一（旧实现用奖励物品名当主键会互相覆盖，导致遗物奖励列表为空）。
func TestParseRelicExportKeepsSharedRewards(t *testing.T) {
	raw := []byte(`{"ExportRelicArcane":[
		{"uniqueName":"/R/RequiemI","name":"安魂 I 遗物","codexSecret":false,"description":"d","relicRewards":[
			{"rewardName":"/Lotus/StoreItems/Types/Items/MiscItems/Kuva","rarity":"COMMON","tier":0,"itemCount":1200},
			{"rewardName":"/Lotus/Upgrades/Mods/Immortal/ImmortalOneMod","rarity":"UNCOMMON","tier":0,"itemCount":1},
			{"rewardName":"/Lotus/StoreItems/Types/Recipes/Components/FormaBlueprint","rarity":"UNCOMMON","tier":0,"itemCount":2},
			{"rewardName":"/Lotus/StoreItems/Types/Recipes/Components/FormaBlueprint","rarity":"COMMON","tier":0,"itemCount":2}
		]},
		{"uniqueName":"/R/RequiemIII","name":"安魂 III 遗物","codexSecret":false,"description":"d","relicRewards":[
			{"rewardName":"/Lotus/StoreItems/Types/Items/MiscItems/Kuva","rarity":"COMMON","tier":0,"itemCount":1200},
			{"rewardName":"/Lotus/Upgrades/Mods/Immortal/ImmortalFiveMod","rarity":"UNCOMMON","tier":0,"itemCount":1}
		]},
		{"uniqueName":"/R/Dup","name":"安魂 I 遗物","codexSecret":false,"description":"d","relicRewards":[]},
		{"uniqueName":"/R/Secret","name":"机密遗物","codexSecret":true,"description":"d","relicRewards":[]}
	]}`)

	relics, err := warframe.ParseRelicExport(raw)
	if err != nil {
		t.Fatal(err)
	}
	// 按 name 去重（保留首个）+ 过滤 codexSecret：RequiemI / RequiemIII
	if len(relics) != 2 {
		t.Fatalf("expect 2 relics after dedup/secret filter, got %d", len(relics))
	}
	if relics[0].Name != "安魂 I 遗物" || relics[1].Name != "安魂 III 遗物" {
		t.Fatalf("relic order/name mismatch: %+v", relics)
	}
	// 同名不同稀有度的槽位必须都保留（真实数据："前纪 D1 遗物" 的 Forma 蓝图在 UNCOMMON/COMMON 各一条）
	if len(relics[0].RelicRewards) != 4 || len(relics[1].RelicRewards) != 2 {
		t.Fatalf("shared rewards must be kept per relic: %d / %d",
			len(relics[0].RelicRewards), len(relics[1].RelicRewards))
	}
	if relics[0].RelicRewards[0].RelicsID != "/R/RequiemI" || relics[1].RelicRewards[0].RelicsID != "/R/RequiemIII" {
		t.Fatalf("relics_id mismatch: %+v", []modelwarframe.RelicRewards{
			relics[0].RelicRewards[0], relics[1].RelicRewards[0]})
	}
	// 主键必须全局唯一（旧实现两条都是奖励物品名 → 冲突）
	ids := map[string]bool{}
	for _, relic := range relics {
		for _, reward := range relic.RelicRewards {
			if ids[reward.ID] {
				t.Fatalf("duplicate reward id %q", reward.ID)
			}
			ids[reward.ID] = true
		}
	}
	wantID := "/R/RequiemI|/Lotus/StoreItems/Types/Items/MiscItems/Kuva|COMMON|0|1200"
	if !ids[wantID] {
		t.Fatalf("unexpected deterministic id set: %v", ids)
	}
}

// TestTranslateRelicRewardNames 验证遗物奖励名按 state_translation 翻译，未命中保留原始路径且主键不变。
func TestTranslateRelicRewardNames(t *testing.T) {
	setupImportFixDB(t)
	// 对齐真实数据：state_translation 存完整 uniqueName，奖励名只给末三段
	database.DB.Create(&modelwarframe.StateTranslation{
		UniqueName: "/Lotus/Upgrades/Mods/Immortal/ImmortalOneMod", Name: "Lohk", Type: 6,
	})
	database.DB.Create(&modelwarframe.StateTranslation{
		UniqueName: "/Lotus/StoreItems/Types/Items/MiscItems/Kuva", Name: "赤毒", Type: 3,
	})

	relics, err := warframe.ParseRelicExport([]byte(`{"ExportRelicArcane":[
		{"uniqueName":"/R/RequiemI","name":"安魂 I 遗物","codexSecret":false,"relicRewards":[
			{"rewardName":"/Lotus/Upgrades/Mods/Immortal/ImmortalOneMod","rarity":"UNCOMMON","tier":0,"itemCount":1},
			{"rewardName":"/Lotus/StoreItems/Types/Items/MiscItems/Kuva","rarity":"COMMON","tier":0,"itemCount":1200},
			{"rewardName":"/Lotus/StoreItems/Types/Items/MiscItems/Unknown","rarity":"COMMON","tier":0,"itemCount":1}
		]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	untranslated := warframe.TranslateRelicRewardNames(relics)
	if len(untranslated) != 1 || untranslated[0] != "/Lotus/StoreItems/Types/Items/MiscItems/Unknown" {
		t.Fatalf("untranslated mismatch: %v", untranslated)
	}
	rewards := relics[0].RelicRewards
	if rewards[0].RewardName != "Lohk" || rewards[1].RewardName != "赤毒" {
		t.Fatalf("translation mismatch: %+v", rewards)
	}
	if rewards[2].RewardName != "/Lotus/StoreItems/Types/Items/MiscItems/Unknown" {
		t.Fatalf("untranslated should keep raw name: %+v", rewards[2])
	}
	// 翻译只改名称，不改主键
	if rewards[0].ID != "/R/RequiemI|/Lotus/Upgrades/Mods/Immortal/ImmortalOneMod|UNCOMMON|0|1" {
		t.Fatalf("id must stay raw-based: %q", rewards[0].ID)
	}
	// itemCount > 1 时 JSON 输出数量前缀（对齐 Java RelicRewards.getRewardName）
	raw, err := json.Marshal(rewards[1])
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["rewardName"] != "1200X赤毒" || decoded["rarity"] != "COMMON" {
		t.Fatalf("reward json mismatch: %s", string(raw))
	}
}

// TestTranslateStateNameSuffixLookup 验证 state_translation 查询按 Java 的 RIGHT(uniqueName, LEN) 后缀语义，
// 而不是等值匹配（表内存完整路径，调用方传末三段）。
func TestTranslateStateNameSuffixLookup(t *testing.T) {
	setupImportFixDB(t)
	database.DB.Create(&modelwarframe.StateTranslation{
		UniqueName: "/Lotus/StoreItems/Weapons/ChromaPrime", Name: "Chroma Prime", Type: 8,
	})
	// 兼容 Java 侧曾用短 key 入库的数据
	database.DB.Create(&modelwarframe.StateTranslation{
		UniqueName: "StoreItems/Weapons/FrostPrime", Name: "Frost Prime", Type: 8,
	})
	// 空名行不应被采用
	database.DB.Create(&modelwarframe.StateTranslation{
		UniqueName: "/Lotus/StoreItems/Weapons/EmptyName", Name: "", Type: 8,
	})

	if got := warframe.TranslateStateName("/Lotus/StoreItems/Weapons/ChromaPrime"); got != "Chroma Prime" {
		t.Fatalf("suffix lookup failed, got %q", got)
	}
	if got := warframe.TranslateStateName("/Lotus/StoreItems/Weapons/FrostPrime"); got != "Frost Prime" {
		t.Fatalf("short-key lookup failed, got %q", got)
	}
	if got := warframe.TranslateStateNameDirect("/Lotus/StoreItems/Weapons/ChromaPrime"); got != "Chroma Prime" {
		t.Fatalf("direct lookup failed, got %q", got)
	}
	if got := warframe.TranslateStateNameSuffix("Weapons/EmptyName"); got != "" {
		t.Fatalf("empty name row must not be used, got %q", got)
	}
	if got := warframe.TranslateStateName("/Lotus/StoreItems/Weapons/Missing"); got != "/Lotus/StoreItems/Weapons/Missing" {
		t.Fatalf("miss should fall back to raw, got %q", got)
	}
}

// TestStateTypeTableOrdinalAlignment 验证 StateType 单一事实来源表：切片下标即 ORDINAL，
// 前 9 项 KEY 为空（对齐 Java，仅作来源默认类型），枚举名/label 与 Java 声明一致。
func TestStateTypeTableOrdinalAlignment(t *testing.T) {
	if len(modelwarframe.StateTypes) != 40 {
		t.Fatalf("StateTypes should have 40 entries (对齐 StateTypeEnum), got %d", len(modelwarframe.StateTypes))
	}
	want := map[int]string{
		0: "ALL", 3: "RESOURCES", 7: "WARFRAMES", 8: "WEAPONS",
		9: "RELIC_BRONZE", 12: "RELIC_SILVER", 14: "SKINS", 37: "PACKAGES", 39: "BLUEPRINT",
	}
	for ordinal, name := range want {
		if got := modelwarframe.StateTypes[ordinal].Name; got != name {
			t.Errorf("StateTypes[%d].Name = %q, want %q", ordinal, got, name)
		}
		if got := modelwarframe.StateTypeOrdinalToName(ordinal); got != name {
			t.Errorf("StateTypeOrdinalToName(%d) = %q, want %q", ordinal, got, name)
		}
		if got, ok := modelwarframe.StateTypeOrdinal(name); !ok || got != ordinal {
			t.Errorf("StateTypeOrdinal(%q) = (%d,%v), want (%d,true)", name, got, ok, ordinal)
		}
	}
	// Java 中 ALL/GEAR/KEYS/RESOURCES/SENTINELS/OTHER/MODS/WARFRAMES/WEAPONS 的 KEY 为空串
	for index := 0; index <= 8; index++ {
		if modelwarframe.StateTypes[index].Key != "" {
			t.Errorf("StateTypes[%d] (%s) KEY 应为空串，实际 %q",
				index, modelwarframe.StateTypes[index].Name, modelwarframe.StateTypes[index].Key)
		}
	}
	// label 对齐 Java NAME（前端下拉文案）
	if label := modelwarframe.StateTypes[7].Label; label != "战甲" {
		t.Errorf("WARFRAMES label = %q, want 战甲", label)
	}
	// 未知枚举名回退 RESOURCES(3)，越界序数回退 ALL
	if got := modelwarframe.StateTypeNameToOrdinal("NOT_A_TYPE"); got != 3 {
		t.Errorf("unknown type should fall back to RESOURCES(3), got %d", got)
	}
	if got := modelwarframe.StateTypeOrdinalToName(-1); got != "ALL" {
		t.Errorf("out-of-range ordinal should fall back to ALL, got %q", got)
	}
}

// TestLoadStateTranslationMap 验证批量翻译映射（对齐 Java RelicsImportUtil.loadTranslationMap）：
// 跨批次（>200 关键词）仍能命中、按后缀匹配、空名行不参与、未命中不出现在映射里。
func TestLoadStateTranslationMap(t *testing.T) {
	setupImportFixDB(t)
	database.DB.Create(&modelwarframe.StateTranslation{UniqueName: "/Lotus/A/Mods/Alpha", Name: "阿尔法", Type: 6})
	database.DB.Create(&modelwarframe.StateTranslation{UniqueName: "/Lotus/B/Types/Beta", Name: "贝塔", Type: 3})
	database.DB.Create(&modelwarframe.StateTranslation{UniqueName: "/Lotus/C/Mods/Empty", Name: "", Type: 6})
	// 单段 uniqueName（对齐 CDN 里 ChamberA 这类短键）
	database.DB.Create(&modelwarframe.StateTranslation{UniqueName: "ChamberA", Name: "隔离库（等级2）", Type: 3})

	// 前 240 个关键词都无命中，真实关键词混在大量噪声之后
	keywords := make([]string, 0, 245)
	for index := 0; index < 240; index++ {
		keywords = append(keywords, fmt.Sprintf("Filler/Segment%d", index))
	}
	keywords = append(keywords, "Mods/Alpha", "Types/Beta", "Mods/Empty", "ChamberA", "Mods/Alpha")

	translations := warframe.LoadStateTranslationMap(keywords)
	if len(translations) != 3 {
		t.Fatalf("expect 3 hits, got %d: %+v", len(translations), translations)
	}
	if translations["Mods/Alpha"] != "阿尔法" || translations["Types/Beta"] != "贝塔" {
		t.Fatalf("suffix lookup mismatch: %+v", translations)
	}
	if translations["ChamberA"] != "隔离库（等级2）" {
		t.Fatalf("short uniqueName lookup mismatch: %+v", translations)
	}
	if _, ok := translations["Mods/Empty"]; ok {
		t.Fatalf("empty-name row must not be used: %+v", translations)
	}
	if _, ok := translations["Filler/Segment0"]; ok {
		t.Fatalf("unmatched keyword must be absent: %+v", translations)
	}
	// 空关键词列表不应查询数据库
	if got := warframe.LoadStateTranslationMap(nil); len(got) != 0 {
		t.Fatalf("nil keywords should yield empty map, got %+v", got)
	}
}

// TestGORMLoggerTruncatesLongSQL 验证超长 SQL 在写日志前被截断：
// 批量 INSERT（500 行实测 58KB）与多条件 LIKE 会把一条日志撑到几十 KB，挤爆日志文件与 SSE。
// 直接驱动 logger.Interface.Trace（slowThreshold 设为 1ns 使其走「慢查询」分支），
// 不依赖 GORM 查询执行与共享日志历史，避免与其它用例相互干扰。
func TestGORMLoggerTruncatesLongSQL(t *testing.T) {
	gormLogger := database.NewGORMLogger(logger.Warn, time.Nanosecond, true)
	// begin 取 1 秒前：确保 elapsed 必然超过 1ns 阈值，稳定走「慢查询」分支
	begin := time.Now().Add(-time.Second)

	// ① 短 SQL 原样输出，不截断
	shortSQL := "SELECT 'short-sql-marker-7f2a'"
	gormLogger.Trace(context.Background(), begin, func() (string, int64) { return shortSQL, 1 }, nil)
	shortSeen := false
	for _, entry := range logging.Recent(logging.LevelTrace) {
		if entry.Pack == "database.sql" && strings.Contains(entry.Message, "short-sql-marker-7f2a") {
			shortSeen = true
			if strings.Contains(entry.Message, "bytes truncated") {
				t.Fatalf("short SQL should not be truncated: %s", entry.Message)
			}
		}
	}
	if !shortSeen {
		t.Fatal("short SQL should be logged as-is through unified logging")
	}

	// ② 超长 SQL 被截断，且不包含完整长值
	longValue := strings.Repeat("A", 4000)
	longSQL := "INSERT INTO relics (unique_name) VALUES ('" + longValue + "')"
	gormLogger.Trace(context.Background(), begin, func() (string, int64) { return longSQL, 500 }, nil)

	for _, entry := range logging.Recent(logging.LevelTrace) {
		if entry.Pack != "database.sql" || !strings.Contains(entry.Message, "bytes truncated") {
			continue
		}
		if len(entry.Message) > 1300 {
			t.Fatalf("truncated SQL log still too long: %d bytes", len(entry.Message))
		}
		if strings.Contains(entry.Message, longValue) {
			t.Fatal("truncated log should not contain the full long value")
		}
		return
	}
	t.Fatal("expect a truncated SQL log entry for a long query")
}

// TestParseSQLLogLevel 验证 config.yaml 的 log.sql_level 解析：静默/错误/警告/明细，
// 空值与非法值回退 warn（保证默认不刷 SQL）。
func TestParseSQLLogLevel(t *testing.T) {
	cases := map[string]logger.LogLevel{
		"":        logger.Warn,
		"warn":    logger.Warn,
		" WARN ":  logger.Warn,
		"unknown": logger.Warn,
		"silent":  logger.Silent,
		"off":     logger.Silent,
		"error":   logger.Error,
		"info":    logger.Info,
		"debug":   logger.Info,
	}
	for input, want := range cases {
		if got := database.ParseSQLLogLevel(input); got != want {
			t.Errorf("ParseSQLLogLevel(%q) = %v, want %v", input, got, want)
		}
	}
}

// TestGORMLoggerRoutesToUnifiedLogging 验证 GORM 日志统一走 internal/logging：
// ①「记录不存在」是预期命中失败，不再刷 ERROR；② 真实 SQL 错误以 pack=database.sql + ERROR 进入统一日志。
// 说明：不调用 logging.Init，直接读取其内存历史，避免改动全局日志配置影响其它用例。
func TestGORMLoggerRoutesToUnifiedLogging(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "gormlog.db")), &gorm.Config{
		Logger: database.NewGORMLogger(logger.Warn, 200*time.Millisecond, true),
	})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&modelwarframe.StateTranslation{}); err != nil {
		t.Fatal(err)
	}

	// ① record not found 不应出现在统一日志（此前会被 GORM 默认 logger 打成 ERROR + SQL）
	var record modelwarframe.StateTranslation
	if err := db.Where("unique_name = ?", "missing").First(&record).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expect ErrRecordNotFound, got %v", err)
	}
	for _, entry := range logging.Recent(logging.LevelTrace) {
		if strings.Contains(entry.Message, "record not found") {
			t.Fatalf("record not found should be ignored, got log entry: %s", entry.Message)
		}
	}

	// ② 真实 SQL 错误必须能被统一日志捕获（pack=database.sql）
	if err := db.Raw("SELECT 1 FROM definitely_missing_table").Scan(&record).Error; err == nil {
		t.Fatal("expect SQL error for missing table")
	}
	hit := false
	for _, entry := range logging.Recent(logging.LevelTrace) {
		if entry.Pack == "database.sql" && entry.Level == logging.LevelError &&
			strings.Contains(entry.Message, "definitely_missing_table") {
			hit = true
		}
	}
	if !hit {
		t.Fatal("SQL error should be logged through unified logging with pack database.sql")
	}
}

// TestFindRelicsByNameOrRewardFallbacks 验证遗物查询四级回退（对齐 Java
// RelicsService.findAllByRelicNameOrRewardsItemName）：等值 → 名称模糊 → 奖励名模糊 → 别名替换。
func TestFindRelicsByNameOrRewardFallbacks(t *testing.T) {
	setupImportFixDB(t)
	db := database.DB
	db.Create(&modelwarframe.Relics{
		UniqueName: "/R/LithA1", Name: "古纪 A1 遗物",
		RelicRewards: []modelwarframe.RelicRewards{
			{ID: "/R/LithA1|/Lotus/StoreItems/Types/Items/MiscItems/Kuva", RelicsID: "/R/LithA1",
				RewardName: "赤毒", Rarity: 0, Tier: 0, ItemCount: 1200},
		},
	})
	db.Create(&modelwarframe.Relics{
		UniqueName: "/R/LithA2", Name: "前纪 B2 遗物",
		RelicRewards: []modelwarframe.RelicRewards{
			{ID: "/R/LithA2|/Lotus/Weapons/.../ChromaPrimeBlueprint", RelicsID: "/R/LithA2",
				RewardName: "Chroma Prime 蓝图", Rarity: 2, Tier: 0, ItemCount: 1},
		},
	})
	db.Create(&modelwarframe.Relics{UniqueName: "/R/LithA3", Name: "中纪 C3 遗物"})
	db.Create(&modelwarframe.Alias{Cn: "核桃", En: "Relic"})

	// ① 名称等值（忽略大小写）
	if got := warframe.FindRelicsByNameOrReward("古纪 A1 遗物"); len(got) != 1 || got[0].UniqueName != "/R/LithA1" {
		t.Fatalf("exact name fallback failed: %+v", got)
	}
	// ② 名称模糊
	if got := warframe.FindRelicsByNameOrReward("中纪 C3"); len(got) != 1 || got[0].UniqueName != "/R/LithA3" {
		t.Fatalf("fuzzy name fallback failed: %+v", got)
	}
	// ③ 奖励物品名模糊
	if got := warframe.FindRelicsByNameOrReward("Chroma Prime"); len(got) != 1 || got[0].UniqueName != "/R/LithA2" {
		t.Fatalf("reward name fallback failed: %+v", got)
	}
	// ④ 别名替换后再查奖励名（"核桃" → "Relic" 命中 Chroma Prime 蓝图 中的大小写不敏感匹配）
	db.Create(&modelwarframe.Relics{
		UniqueName: "/R/LithA4", Name: "后纪 D4 遗物",
		RelicRewards: []modelwarframe.RelicRewards{
			{ID: "/R/LithA4|relic-rare", RelicsID: "/R/LithA4", RewardName: "Relic 稀有部件", Rarity: 2, Tier: 0, ItemCount: 1},
		},
	})
	if got := warframe.FindRelicsByNameOrReward("核桃"); len(got) != 1 || got[0].UniqueName != "/R/LithA4" {
		t.Fatalf("alias fallback failed: %+v", got)
	}
	// 未命中
	if got := warframe.FindRelicsByNameOrReward("不存在的东西"); len(got) != 0 {
		t.Fatalf("unexpected hit: %+v", got)
	}
}
