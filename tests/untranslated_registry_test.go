// 未翻译清单注册表回归测试：
// 覆盖运行期未命中登记（世界状态物品/奖励/挑战/赏金/节点）、登记去重、文件已有条目跳过、
// 人工编辑内容不被覆盖、清单文件损坏时不写回、批量合并去重。
package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"nyxbot-go/internal/database"
	modelwarframe "nyxbot-go/internal/model/warframe"
	"nyxbot-go/internal/warframe"
)

// useUntranslatedFile 把未翻译清单指向临时目录并返回路径，用例结束还原原路径。
func useUntranslatedFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "UntranslatedRelicsRewardsName.json")
	previous := warframe.SetUntranslatedPath(path)
	t.Cleanup(func() { warframe.SetUntranslatedPath(previous) })
	return path
}

// readUntranslatedItems 读取未翻译清单文件并解析；文件不存在返回 nil。
func readUntranslatedItems(t *testing.T, path string) []warframe.UntranslatedItem {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	var items []warframe.UntranslatedItem
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatalf("decode %s: %v (%s)", path, err, string(raw))
	}
	return items
}

// TestRecordUntranslatedDeduplicates 验证同一 uniqueName 只登记一次，草稿名按 Java 规则生成。
func TestRecordUntranslatedDeduplicates(t *testing.T) {
	path := useUntranslatedFile(t)

	warframe.RecordUntranslated("/Lotus/Types/Items/MiscItems/VoidTraces")
	warframe.RecordUntranslated("/Lotus/Types/Items/MiscItems/VoidTraces")
	warframe.RecordUntranslated("/Lotus/StoreItems/Types/Recipes/Components/FormaBlueprint")
	warframe.RecordUntranslated("")

	items := readUntranslatedItems(t, path)
	if len(items) != 2 {
		t.Fatalf("expect 2 deduped items, got %d: %+v", len(items), items)
	}
	if items[0].UniqueName != "/Lotus/Types/Items/MiscItems/VoidTraces" || items[0].Name != "Void Traces" {
		t.Fatalf("first item mismatch: %+v", items[0])
	}
	if items[1].Name != "Forma 蓝图" {
		t.Fatalf("guessed name mismatch: %+v", items[1])
	}
}

// TestRecordUntranslatedSkipsExistingFileEntries 验证清单文件里已有的 uniqueName 不再添加，
// 且人工填写的 name/description 不会被覆盖。
func TestRecordUntranslatedSkipsExistingFileEntries(t *testing.T) {
	path := useUntranslatedFile(t)
	existing := []warframe.UntranslatedItem{
		{UniqueName: "/Lotus/Types/Items/MiscItems/VoidTraces", Name: "虚空光体", Description: "人工补的描述"},
	}
	if err := warframe.SaveUntranslatedItems(path, existing); err != nil {
		t.Fatal(err)
	}
	// 重新指向同一路径：清掉进程内缓存，强制从磁盘加载（模拟新进程 / 人工编辑后重启）
	warframe.SetUntranslatedPath(path)

	warframe.RecordUntranslated("/Lotus/Types/Items/MiscItems/VoidTraces")
	warframe.RecordUntranslated("/Lotus/Types/Items/MiscItems/Gallium")

	items := readUntranslatedItems(t, path)
	if len(items) != 2 {
		t.Fatalf("expect 2 items (1 existing + 1 new), got %d: %+v", len(items), items)
	}
	if items[0].UniqueName != "/Lotus/Types/Items/MiscItems/VoidTraces" ||
		items[0].Name != "虚空光体" || items[0].Description != "人工补的描述" {
		t.Fatalf("hand-edited entry must be preserved: %+v", items[0])
	}
	if items[1].UniqueName != "/Lotus/Types/Items/MiscItems/Gallium" || items[1].Name != "Gallium" {
		t.Fatalf("new entry mismatch: %+v", items[1])
	}
}

// TestRecordUntranslatedKeepsCorruptFileUntouched 验证清单文件语法损坏时不写回（宁可不记也不覆盖人工内容）。
func TestRecordUntranslatedKeepsCorruptFileUntouched(t *testing.T) {
	path := useUntranslatedFile(t)
	broken := []byte(`[{"uniqueName": "/Lotus/A", "name": `)
	if err := os.WriteFile(path, broken, 0o644); err != nil {
		t.Fatal(err)
	}
	warframe.SetUntranslatedPath(path)

	warframe.RecordUntranslated("/Lotus/Types/Items/MiscItems/VoidTraces")

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != string(broken) {
		t.Fatalf("corrupt file must stay untouched, got %q", string(raw))
	}
}

// TestMergeUntranslatedItemsDeduplicates 验证批量合并（遗物导入路径）按 uniqueName 去重并保持顺序。
func TestMergeUntranslatedItemsDeduplicates(t *testing.T) {
	path := useUntranslatedFile(t)

	added, err := warframe.MergeUntranslatedItems([]warframe.UntranslatedItem{
		{UniqueName: "/Lotus/A/ChromaPrimeBlueprint"},
		{UniqueName: "/Lotus/B/Kuva"},
		{UniqueName: "/Lotus/A/ChromaPrimeBlueprint"},
		{UniqueName: ""},
	})
	if err != nil {
		t.Fatal(err)
	}
	if added != 2 {
		t.Fatalf("first merge should add 2, got %d", added)
	}
	// 第二次合并：A 已存在（且人工改名），C 是新的
	added, err = warframe.MergeUntranslatedItems([]warframe.UntranslatedItem{
		{UniqueName: "/Lotus/A/ChromaPrimeBlueprint", Name: "Chroma Prime 蓝图（人工）"},
		{UniqueName: "/Lotus/C/Neurodes"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if added != 1 {
		t.Fatalf("second merge should add 1, got %d", added)
	}

	items := readUntranslatedItems(t, path)
	if len(items) != 3 {
		t.Fatalf("expect 3 items, got %d: %+v", len(items), items)
	}
	if items[0].Name != "Chroma Prime 蓝图" {
		t.Fatalf("existing entry must not be overwritten by merge: %+v", items[0])
	}
	if items[2].UniqueName != "/Lotus/C/Neurodes" || items[2].Name != "Neurodes" {
		t.Fatalf("merged item mismatch: %+v", items[2])
	}
}

// TestUntranslatedWritesRespectManualEdits 验证写回以磁盘为准：
// 运行期登记新未命中时，人工填写的译名不被覆盖，人工删掉的条目也不会被复活。
func TestUntranslatedWritesRespectManualEdits(t *testing.T) {
	path := useUntranslatedFile(t)

	warframe.RecordUntranslated("/Lotus/Types/Items/MiscItems/VoidTraces")
	// 人工把草稿名改成正式译名
	if err := warframe.SaveUntranslatedItems(path, []warframe.UntranslatedItem{
		{UniqueName: "/Lotus/Types/Items/MiscItems/VoidTraces", Name: "虚空光体", Description: "人工补的"},
	}); err != nil {
		t.Fatal(err)
	}

	warframe.RecordUntranslated("/Lotus/Types/Items/MiscItems/Gallium")
	items := readUntranslatedItems(t, path)
	if len(items) != 2 || items[0].Name != "虚空光体" || items[0].Description != "人工补的" {
		t.Fatalf("hand-edited entry must survive a later write: %+v", items)
	}
	if items[1].UniqueName != "/Lotus/Types/Items/MiscItems/Gallium" {
		t.Fatalf("new miss should be appended: %+v", items)
	}

	// 人工删掉已补好翻译的条目（只留 Gallium），后续写入不得把它复活
	if err := warframe.SaveUntranslatedItems(path, []warframe.UntranslatedItem{items[1]}); err != nil {
		t.Fatal(err)
	}
	warframe.RecordUntranslated("/Lotus/Types/Items/MiscItems/Neurodes")

	items = readUntranslatedItems(t, path)
	if len(items) != 2 {
		t.Fatalf("expect 2 items after manual deletion, got %d: %+v", len(items), items)
	}
	for _, item := range items {
		if item.UniqueName == "/Lotus/Types/Items/MiscItems/VoidTraces" {
			t.Fatalf("manually deleted entry must not come back: %+v", items)
		}
	}
	if items[1].UniqueName != "/Lotus/Types/Items/MiscItems/Neurodes" {
		t.Fatalf("new miss should be appended after the disk content: %+v", items)
	}
}

// TestRuntimeTranslationMissesRecorded 验证世界状态翻译未命中会登记到清单：
// 物品名（TranslateStateName）、警报奖励/日历字段（TranslateStateNameDirect）、
// 节点（TranslateNode，未找到或没有名称）；命中项不登记。
func TestRuntimeTranslationMissesRecorded(t *testing.T) {
	setupImportFixDB(t)
	if err := database.DB.AutoMigrate(&modelwarframe.Nodes{}); err != nil {
		t.Fatal(err)
	}
	path := useUntranslatedFile(t)

	database.DB.Create(&modelwarframe.StateTranslation{
		UniqueName: "/Lotus/StoreItems/Weapons/ChromaPrime", Name: "Chroma Prime", Type: 8,
	})
	database.DB.Create(&modelwarframe.Nodes{
		UniqueName: "/Node/Abaddon", Name: "阿巴顿", SystemName: "海王星",
	})
	database.DB.Create(&modelwarframe.Nodes{UniqueName: "/Node/NoName", Name: ""})

	// 命中：不登记
	if got := warframe.TranslateStateName("/Lotus/StoreItems/Weapons/ChromaPrime"); got != "Chroma Prime" {
		t.Fatalf("state name hit mismatch: %q", got)
	}
	if got := warframe.TranslateNode("/Node/Abaddon"); got != "阿巴顿(海王星)" {
		t.Fatalf("node hit mismatch: %q", got)
	}
	if items := readUntranslatedItems(t, path); len(items) != 0 {
		t.Fatalf("hits must not be recorded, got %+v", items)
	}

	// 未命中：登记（物品 / 警报奖励 / 节点未找到 / 节点无名称）
	if got := warframe.TranslateStateName("/Lotus/Types/Items/MiscItems/VoidTraces"); got != "/Lotus/Types/Items/MiscItems/VoidTraces" {
		t.Fatalf("state name miss should fallback to raw, got %q", got)
	}
	warframe.TranslateStateNameDirect("/Lotus/Language/Calendar/EventAlpha")
	warframe.TranslateNode("/Node/Unknown")
	warframe.TranslateNode("/Node/NoName")
	// 再次未命中：去重后不重复添加
	warframe.TranslateStateName("/Lotus/Types/Items/MiscItems/VoidTraces")

	items := readUntranslatedItems(t, path)
	got := make([]string, 0, len(items))
	for _, item := range items {
		got = append(got, item.UniqueName)
	}
	want := map[string]bool{
		"/Lotus/Types/Items/MiscItems/VoidTraces": true,
		"/Lotus/Language/Calendar/EventAlpha":     true,
		"/Node/Unknown":                           true,
		"/Node/NoName":                            true,
	}
	if len(items) != len(want) {
		t.Fatalf("expect %d misses recorded, got %d: %v", len(want), len(items), got)
	}
	for _, uniqueName := range got {
		if !want[uniqueName] {
			t.Fatalf("unexpected recorded miss %q (all: %v)", uniqueName, got)
		}
	}
	for _, item := range items {
		if item.UniqueName == "/Lotus/Types/Items/MiscItems/VoidTraces" && item.Name != "Void Traces" {
			t.Fatalf("miss draft name mismatch: %+v", item)
		}
	}
}
