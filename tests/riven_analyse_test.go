// 紫卡分析测试：解析/计算/分析链路（临时 sqlite 造数）+ OCR 全链路（模型存在时）。
package tests

import (
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"nyxbot-go/internal/database"
	modelwarframe "nyxbot-go/internal/model/warframe"
	"nyxbot-go/internal/ocr"
	"nyxbot-go/internal/warframe/rivenanalyse"
)

// setupRivenDB 初始化临时 sqlite 并置入紫卡分析所需的武器与趋势测试数据。
func setupRivenDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "riven.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&modelwarframe.Weapons{}, &modelwarframe.RivenAnalyseTrend{}); err != nil {
		t.Fatal(err)
	}
	weapons := []modelwarframe.Weapons{
		{
			UniqueName: "/Lotus/Weapons/Tenno/守望者", Name: "守望者",
			ProductCategory: 1, // LongGuns
			CriticalChance:  0.30, CriticalMultiplier: 2.0, ProcChance: 0.16,
			FireRate: 1.5, TotalDamage: 400, MagazineSize: 10, ReloadTime: 2.0,
			Description: "狙击枪", OmegaAttenuation: 1.35, DamagePerShot: "[60,60,280]",
		},
		{
			UniqueName: "/Lotus/Weapons/Tenno/测试枪", Name: "测试枪",
			ProductCategory: 1,
			CriticalChance:  0.30, CriticalMultiplier: 2.0, ProcChance: 0.16,
			FireRate: 1.5, TotalDamage: 400, MagazineSize: 10, ReloadTime: 2.0,
			Description: "", OmegaAttenuation: 1.35, DamagePerShot: "[100,100,100]",
		},
	}
	trends := []modelwarframe.RivenAnalyseTrend{
		{Name: "暴击几率", Rifle: 150},
		{Name: "装填速度", Rifle: 50},
		{Name: "触发时间", Rifle: 45},
		{Name: "对Grineer伤害", Rifle: 45},
	}
	if err := db.Create(&weapons).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&trends).Error; err != nil {
		t.Fatal(err)
	}
	database.DB = db
	t.Cleanup(func() { closeGlobalDB(t) })
}

// TestRivenAnalyseCompute 真实样本（守望者-0 的 v6 OCR 输出）端到端解析与计算：
// 武器名/紫卡名提取、词条收集与噪音过滤、负属性致命度、星级乱码行不干扰武器匹配。
func TestRivenAnalyseCompute(t *testing.T) {
	setupRivenDB(t)

	lines := []string{
		"本",
		"104",
		"守望者 Argi-",
		"fevacron",
		"x1.06 对 Grineer 的伤害",
		"+6.5%装填速度",
		"+19.1%暴击几率",
		"-10.1% 触发时间",
		"110",
		"段位 14",
		"会会会会计会会会", // 星级乱码：不应干扰武器匹配
	}
	models := rivenanalyse.NewCalculator(database.DB).Analyse(lines)
	if len(models) == 0 {
		t.Fatal("应匹配到武器并产出分析结果")
	}
	m := models[0]
	if m.WeaponName != "守望者" {
		t.Fatalf("武器名应为守望者（乱码行不得干扰），实际 %q", m.WeaponName)
	}
	if m.RivenName != "Argi-fevacron" {
		t.Fatalf("紫卡名应为拼接结果 Argi-fevacron，实际 %q", m.RivenName)
	}
	if m.WeaponType != "主要武器" {
		t.Fatalf("武器类型应为主要武器，实际 %q", m.WeaponType)
	}
	if len(m.Attributes) != 4 {
		t.Fatalf("应收集 4 个词条（价格/段位等噪音被过滤），实际 %d: %+v", len(m.Attributes), m.Attributes)
	}

	byName := make(map[string]int, len(m.Attributes))
	for i, a := range m.Attributes {
		byName[a.Name] = i
	}
	for _, want := range []string{"对Grineer的伤害", "装填速度", "暴击几率", "触发时间"} {
		if _, ok := byName[want]; !ok {
			t.Errorf("缺少词条 %q（实际：%v）", want, byName)
		}
	}

	// 负属性（-10.1% 触发时间）才有致命度；正属性应为空
	if idx, ok := byName["触发时间"]; ok && m.Attributes[idx].LethalLevel == "" {
		t.Error("负属性（-10.1% 触发时间）应计算致命度")
	}
	if idx, ok := byName["暴击几率"]; ok && m.Attributes[idx].LethalLevel != "" {
		t.Error("正属性不应有致命度")
	}
	// 歧视词条走歧视偏差分支（含 Grineer 判断）
	if idx, ok := byName["对Grineer的伤害"]; ok && m.Attributes[idx].AttrDiff == "" {
		t.Error("歧视词条应产出偏差文本")
	}
}

// TestRivenAnalyseFormula 双词条精确公式断言：
// 低/高区间 = 0.9/1.1 × 基准值 × 倾向 × 修正系数（2 词条无负 → 0.99）。
func TestRivenAnalyseFormula(t *testing.T) {
	setupRivenDB(t)

	lines := []string{"测试枪 Abc-", "def", "+100%暴击几率", "+50%装填速度"}
	models := rivenanalyse.NewCalculator(database.DB).Analyse(lines)
	if len(models) != 1 {
		t.Fatalf("应产出 1 张结果卡，实际 %d", len(models))
	}
	m := models[0]
	if m.RivenName != "Abc-def" {
		t.Fatalf("紫卡名拼接异常: %q", m.RivenName)
	}
	if len(m.Attributes) != 2 {
		t.Fatalf("应收集 2 个词条，实际 %d", len(m.Attributes))
	}

	crit := m.Attributes[0]
	if crit.Name != "暴击几率" {
		t.Fatalf("首词条应为暴击几率，实际 %q", crit.Name)
	}
	// 暴击几率：baseVal=150, 倾向=1.35, 修正=0.99
	if crit.LowAttr != "180.4275" || crit.HighAttr != "220.5225" {
		t.Errorf("暴击几率低/高区间异常: low=%q high=%q", crit.LowAttr, crit.HighAttr)
	}
	// 满级等效值 ≈ 100 × rankScale(≈1.6038)
	if crit.Attr == nil || math.Abs(*crit.Attr-160.38) > 0.05 {
		t.Errorf("满级等效值异常: %v", crit.Attr)
	}
	// 步枪暴击 0.30 > 0.18 → S 评分；比率 ≈ 160.38 / 30
	if crit.Grade != "S" {
		t.Errorf("暴击几率评分应为 S，实际 %q", crit.Grade)
	}
	if !strings.HasPrefix(crit.Ratio, "5.3") {
		t.Errorf("暴击几率比率异常: %q", crit.Ratio)
	}
	if !strings.HasPrefix(crit.AttrDiff, "-20") {
		t.Errorf("暴击几率偏差应约 -20%%，实际 %q", crit.AttrDiff)
	}

	reload := m.Attributes[1]
	if reload.Name != "装填速度" {
		t.Fatalf("次词条应为装填速度，实际 %q", reload.Name)
	}
	// 装填速度：baseVal=50 → low=60.1425 high=73.5075
	if reload.LowAttr != "60.1425" || reload.HighAttr != "73.5075" {
		t.Errorf("装填速度低/高区间异常: low=%q high=%q", reload.LowAttr, reload.HighAttr)
	}
	if reload.Grade != "C" {
		t.Errorf("步枪装填速度评分应为 C，实际 %q", reload.Grade)
	}
}

// TestRivenAnalyseNoWeapon 未识别到可匹配武器时返回空（不 panic）。
func TestRivenAnalyseNoWeapon(t *testing.T) {
	setupRivenDB(t)
	models := rivenanalyse.NewCalculator(database.DB).Analyse([]string{"不存在的武器 Xxx-", "yyy", "+10%暴击几率"})
	if len(models) != 0 {
		t.Fatalf("未匹配武器应返回空结果，实际 %d", len(models))
	}
}

// TestRivenAnalyseOCRFlow OCR 全链路：真实模型识别紫卡截图 → 解析计算产出结果卡。
// 模型文件不随仓库提供（运行期下载），缺失时跳过。
func TestRivenAnalyseOCRFlow(t *testing.T) {
	modelDir := resolveOCRModelDir(t)
	for _, name := range []string{"det.onnx", "rec.onnx"} {
		if _, err := os.Stat(filepath.Join(modelDir, name)); err != nil {
			t.Skipf("模型目录 %s 缺少 %s，跳过（可用 NYXBOT_TEST_OCR_MODEL_DIR 指定）", modelDir, name)
		}
	}
	setupRivenDB(t)

	f, err := os.Open(filepath.Join("data", "ocr", "riven_sample.png"))
	if err != nil {
		t.Fatalf("打开测试图片失败: %v", err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("解码测试图片失败: %v", err)
	}

	engine, err := ocr.New(ocr.Config{ModelDir: modelDir})
	if err != nil {
		t.Fatalf("创建 OCR 引擎失败: %v", err)
	}
	defer engine.Destroy()

	results, err := engine.Recognize(img)
	if err != nil {
		t.Fatalf("识别失败: %v", err)
	}
	lines := make([]string, 0, len(results))
	for _, r := range results {
		lines = append(lines, r.Text)
	}

	models := rivenanalyse.NewCalculator(database.DB).Analyse(lines)
	if len(models) == 0 {
		t.Fatalf("全链路应产出分析结果，OCR 行：%v", lines)
	}
	m := models[0]
	if m.WeaponName != "守望者" {
		t.Fatalf("武器名应为守望者，实际 %q（OCR 行：%v）", m.WeaponName, lines)
	}
	names := make(map[string]bool)
	for _, a := range m.Attributes {
		names[a.Name] = true
	}
	for _, want := range []string{"暴击几率", "装填速度"} {
		if !names[want] {
			t.Errorf("缺少词条 %q（OCR 行：%v）", want, lines)
		}
	}
	t.Logf("OCR 行：%v\n分析词条：%v", lines, names)
}
