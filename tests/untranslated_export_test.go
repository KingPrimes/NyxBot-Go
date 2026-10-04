// 未翻译内容导出回归测试（对齐 Java RelicsImportUtil.addToUntranslatedItems + exportUntranslatedItems）：
// 覆盖名称猜测规则、清单去重收集、JSON 文件写出，以及遗物导入后自动产出未翻译清单。
package tests

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nyxbot-go/internal/database"
	modelwarframe "nyxbot-go/internal/model/warframe"
	"nyxbot-go/internal/warframe"
)

// TestGuessUntranslatedName 验证未翻译名称草稿的生成规则
// （Java StringUtils.splitCamelCase：大写字母前插空格但前一位是数字时不插，再 capitalizeFully）。
func TestGuessUntranslatedName(t *testing.T) {
	cases := map[string]string{
		"/Lotus/StoreItems/Weapons/Tenno/ChromaPrimeBlueprint":      "Chroma Prime 蓝图",
		"/Lotus/StoreItems/Types/Recipes/Components/FormaBlueprint": "Forma 蓝图",
		"/Lotus/StoreItems/Types/Items/MiscItems/Kuva":              "Kuva",
		"/Lotus/StoreItems/Powersuits/Ash/AshPrimeHelmet":           "Ash Prime 头部神经光元",
		// 部件后缀按 Java 的替换顺序串联：先 Blueprint 再 Systems
		"/Lotus/StoreItems/Types/Recipes/Components/AshSystemsBlueprint": "Ash 系统 蓝图",
		// 前一位是数字时不插空格，且 capitalizeFully 会把大写 X 压成小写
		"/Lotus/StoreItems/Types/Items/MiscItems/300XKuva": "300x Kuva",
		// 无 "/" 时取整串；路径以 "/" 结尾时最后一段为空
		"SimpleName": "Simple Name",
		"/Lotus/A/":  "",
		"":           "",
	}
	for input, want := range cases {
		if got := warframe.GuessUntranslatedName(input); got != want {
			t.Errorf("GuessUntranslatedName(%q) = %q, want %q", input, got, want)
		}
	}
}

// TestCollectUntranslatedItems 验证清单按 uniqueName 去重、保留首次出现顺序、description 留空。
func TestCollectUntranslatedItems(t *testing.T) {
	items := warframe.CollectUntranslatedItems([]string{
		"/Lotus/StoreItems/Types/Items/MiscItems/KuvaBlueprint",
		"",
		"/Lotus/StoreItems/Types/Items/MiscItems/KuvaBlueprint",
		"/Lotus/StoreItems/Types/Items/MiscItems/Neurodes",
	})
	if len(items) != 2 {
		t.Fatalf("expect 2 unique items, got %d: %+v", len(items), items)
	}
	if items[0].UniqueName != "/Lotus/StoreItems/Types/Items/MiscItems/KuvaBlueprint" ||
		items[0].Name != "Kuva 蓝图" || items[0].Description != "" {
		t.Fatalf("first item mismatch: %+v", items[0])
	}
	if items[1].UniqueName != "/Lotus/StoreItems/Types/Items/MiscItems/Neurodes" || items[1].Name != "Neurodes" {
		t.Fatalf("second item mismatch: %+v", items[1])
	}
	if got := warframe.CollectUntranslatedItems(nil); len(got) != 0 {
		t.Fatalf("nil input should yield empty list, got %+v", got)
	}
}

// TestSaveUntranslatedItems 验证 JSON 文件写出：自动建目录、中文与 <...> 不转义、空清单写空数组、
// 原子替换不留 .tmp 残留。
func TestSaveUntranslatedItems(t *testing.T) {
	target := filepath.Join(t.TempDir(), "nested", "UntranslatedRewards.json")
	items := []warframe.UntranslatedItem{
		{UniqueName: "/Lotus/A/ChromaPrimeBlueprint", Name: "Chroma Prime 蓝图", Description: ""},
		{UniqueName: "/Lotus/A/AlertReward", Name: "警报 <稀有> 奖励", Description: "待补"},
	}
	if err := warframe.SaveUntranslatedItems(target, items); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	// 人工要直接编辑该文件：中文与尖括号保持原样，不能被转义成 \uXXXX
	if !strings.Contains(string(raw), "Chroma Prime 蓝图") || !strings.Contains(string(raw), "<稀有>") {
		t.Fatalf("file should keep raw UTF-8 text, got %s", string(raw))
	}
	var decoded []warframe.UntranslatedItem
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decode written file: %v (%s)", err, string(raw))
	}
	if len(decoded) != 2 || decoded[0] != items[0] || decoded[1] != items[1] {
		t.Fatalf("round-trip mismatch: %+v", decoded)
	}
	if _, err := os.Stat(target + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temp file should be renamed away, stat err = %v", err)
	}

	// 空清单写出空数组：文件始终反映最近一次导入结果，不保留陈旧内容
	if err := warframe.SaveUntranslatedItems(target, nil); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(raw)) != "[]" {
		t.Fatalf("empty list should write [], got %q", string(raw))
	}
}

// TestImportRelicsExportsUntranslatedRewards 验证遗物导入端到端：
// 已有译名的奖励被翻译落库，未命中项原样入库并合并进未翻译清单（去重、不覆盖人工填写内容）。
// 说明：ExportFilePath 的 ./data/export 路径相对进程工作目录（go test 为 tests/），
// 因此这里显式创建 tests/data/export/ExportRelicArcane_zh.json 并在用例结束清理；
// 未翻译清单路径由 useUntranslatedFile 指向临时目录。
func TestImportRelicsExportsUntranslatedRewards(t *testing.T) {
	setupImportFixDB(t)
	database.DB.Create(&modelwarframe.StateTranslation{
		UniqueName: "/Lotus/StoreItems/Types/Items/MiscItems/Kuva", Name: "赤毒", Type: 3,
	})
	untranslatedPath := useUntranslatedFile(t)
	// 清单里预先有一条同名条目（模拟人工已填译名），导入时不得重复添加、也不得覆盖
	if err := warframe.SaveUntranslatedItems(untranslatedPath, []warframe.UntranslatedItem{
		{
			UniqueName:  "/Lotus/StoreItems/Types/Recipes/Components/MissingRewardBlueprint",
			Name:        "缺失奖励蓝图",
			Description: "人工补的",
		},
	}); err != nil {
		t.Fatal(err)
	}
	warframe.SetUntranslatedPath(untranslatedPath) // 清进程内缓存，强制从磁盘加载

	workDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	exportPath := filepath.Join(workDir, "data", "export", "ExportRelicArcane_zh.json")
	if err := os.MkdirAll(filepath.Dir(exportPath), 0o755); err != nil {
		t.Fatal(err)
	}
	exportBody := `{"ExportRelicArcane":[
		{"uniqueName":"/R/RequiemI","name":"安魂 I 遗物","codexSecret":false,"relicRewards":[
			{"rewardName":"/Lotus/StoreItems/Types/Items/MiscItems/Kuva","rarity":"COMMON","tier":0,"itemCount":1200},
			{"rewardName":"/Lotus/StoreItems/Types/Recipes/Components/MissingRewardBlueprint","rarity":"UNCOMMON","tier":0,"itemCount":1},
			{"rewardName":"/Lotus/StoreItems/Types/Items/MiscItems/NewUnmappedItem","rarity":"COMMON","tier":0,"itemCount":1}
		]},
		{"uniqueName":"/R/Secret","name":"机密遗物","codexSecret":true,"relicRewards":[]}
	]}`
	if err := os.WriteFile(exportPath, []byte(exportBody), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Remove(exportPath)
		_ = os.Remove(filepath.Dir(exportPath)) // 目录非空时 Remove 失败，属预期
	})

	importer := warframe.NewDataImporter(warframe.NewExportFilePath(nil, "zh"), warframe.NewMarketAPI(nil), database.DB)
	if err := importer.ImportRelics(context.Background()); err != nil {
		t.Fatalf("ImportRelics: %v", err)
	}

	// 落库：已有译名的奖励翻译为中文，未命中项保留原始路径
	var relics []modelwarframe.Relics
	if err := database.DB.Preload("RelicRewards").Order("unique_name").Find(&relics).Error; err != nil {
		t.Fatal(err)
	}
	if len(relics) != 1 || len(relics[0].RelicRewards) != 3 {
		t.Fatalf("expect 1 relic with 3 rewards, got %+v", relics)
	}
	names := map[string]bool{}
	for _, reward := range relics[0].RelicRewards {
		names[reward.RewardName] = true
	}
	if !names["赤毒"] || !names["/Lotus/StoreItems/Types/Recipes/Components/MissingRewardBlueprint"] {
		t.Fatalf("reward names mismatch: %+v", relics[0].RelicRewards)
	}

	// 未翻译清单：预置条目保持人工内容，只追加真正的新未命中项
	items := readUntranslatedItems(t, untranslatedPath)
	if len(items) != 2 {
		t.Fatalf("expect 2 untranslated items (1 seeded + 1 new), got %d: %+v", len(items), items)
	}
	if items[0].UniqueName != "/Lotus/StoreItems/Types/Recipes/Components/MissingRewardBlueprint" ||
		items[0].Name != "缺失奖励蓝图" || items[0].Description != "人工补的" {
		t.Fatalf("seeded entry must be preserved: %+v", items[0])
	}
	if items[1].UniqueName != "/Lotus/StoreItems/Types/Items/MiscItems/NewUnmappedItem" ||
		items[1].Name != "New Unmapped Item" || items[1].Description != "" {
		t.Fatalf("newly recorded entry mismatch: %+v", items[1])
	}

	// 再导入一次：清单已包含全部未命中项，不应重复添加
	if err := importer.ImportRelics(context.Background()); err != nil {
		t.Fatalf("second ImportRelics: %v", err)
	}
	if items = readUntranslatedItems(t, untranslatedPath); len(items) != 2 {
		t.Fatalf("re-import must not duplicate entries, got %d: %+v", len(items), items)
	}
}
