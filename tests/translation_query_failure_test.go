// 翻译查询故障（数据库不可用）回归测试：
// 回退原文、不登记未翻译清单、经全局日志管线限频告警（与「缺译名」明确区分）。
package tests

import (
	"testing"
	"time"

	"nyxbot-go/internal/database"
	"nyxbot-go/internal/logging"
	modelwarframe "nyxbot-go/internal/model/warframe"
	"nyxbot-go/internal/warframe"
)

// closeGlobalDBForFailure 关闭全局数据库的底层连接，模拟查询故障。
func closeGlobalDBForFailure(t *testing.T) {
	t.Helper()
	sqlDB, err := database.DB.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestTranslationQueryFailureFallsBackWithoutRegistry 验证数据库故障时：
// 各类翻译查询回退原文，且不把键登记进未翻译清单（避免把故障当成缺译名）。
func TestTranslationQueryFailureFallsBackWithoutRegistry(t *testing.T) {
	setupImportFixDB(t)
	if err := database.DB.AutoMigrate(&modelwarframe.Nodes{}); err != nil {
		t.Fatal(err)
	}
	path := useUntranslatedFile(t)
	// 预置有效数据，确保「回退」来自查询故障而不是缺数据
	if err := database.DB.Create(&modelwarframe.StateTranslation{
		UniqueName: "/Lotus/StoreItems/Weapons/ChromaPrime", Name: "Chroma Prime", Type: 8,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.DB.Create(&modelwarframe.Nodes{
		UniqueName: "/Node/Abaddon", Name: "阿巴顿", SystemName: "海王星",
	}).Error; err != nil {
		t.Fatal(err)
	}

	closeGlobalDBForFailure(t)

	if got := warframe.TranslateNode("/Node/Abaddon"); got != "/Node/Abaddon" {
		t.Fatalf("节点查询故障应回退原文: %q", got)
	}
	if got := warframe.TranslateStateName("/Lotus/StoreItems/Weapons/ChromaPrime"); got != "/Lotus/StoreItems/Weapons/ChromaPrime" {
		t.Fatalf("物品查询故障应回退原文: %q", got)
	}
	if got := warframe.TranslateStateNameDirect("/Lotus/Language/Calendar/EventAlpha"); got != "/Lotus/Language/Calendar/EventAlpha" {
		t.Fatalf("Direct 查询故障应回退原文: %q", got)
	}
	if record, ok := warframe.FindStateTranslationExact("/Lotus/StoreItems/Weapons/ChromaPrime"); ok || record.UniqueName != "" {
		t.Fatalf("等值查询故障应视为未命中: %+v", record)
	}
	if items := readUntranslatedItems(t, path); len(items) != 0 {
		t.Fatalf("查询故障不应登记未翻译清单: %+v", items)
	}
}

// TestTranslationQueryFailureWarnThrottled 验证查询故障经全局日志管线告警：
// 限频窗口内多次失败只产生一条 WARN；窗口关闭（<=0）时每次失败都告警。
func TestTranslationQueryFailureWarnThrottled(t *testing.T) {
	setupImportFixDB(t)
	closeGlobalDBForFailure(t)

	previous := warframe.SetTranslationQueryWarnWindow(time.Hour)
	t.Cleanup(func() { warframe.SetTranslationQueryWarnWindow(previous) })

	subscription := logging.Subscribe(64)
	t.Cleanup(func() { logging.Unsubscribe(subscription) })

	// drainWarns 收集订阅通道中本测试产生的 warframe.translate WARN（静默 200ms 后结束）。
	drainWarns := func() int {
		count := 0
		timer := time.NewTimer(200 * time.Millisecond)
		defer timer.Stop()
		for {
			select {
			case entry := <-subscription:
				if entry.Pack == "warframe.translate" && entry.Level == logging.LevelWarn {
					count++
				}
			case <-timer.C:
				return count
			}
		}
	}

	// 限频窗口内：连续三类查询失败只应告警一次
	warframe.TranslateNode("/Node/A")
	warframe.TranslateStateNameSuffix("Mods/Alpha")
	if _, ok := warframe.FindStateTranslationExact("/Lotus/Whatever"); ok {
		t.Fatal("查询故障应返回未命中")
	}
	if got := drainWarns(); got != 1 {
		t.Fatalf("限频窗口内应只告警一次, got %d", got)
	}

	// 窗口 <= 0（不限频）：每次失败都告警
	warframe.SetTranslationQueryWarnWindow(0)
	warframe.TranslateNode("/Node/B")
	warframe.TranslateNode("/Node/C")
	if got := drainWarns(); got != 2 {
		t.Fatalf("不限频时两次失败应告警两次, got %d", got)
	}
}
