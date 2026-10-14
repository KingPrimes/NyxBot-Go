// 奖励池导入回归：|COUNT| 原始模板入库（渲染/序列化时替换），
// 对齐 Java Reward.getItem() 与 NightWave「入库保留原始文本」范式。
package tests

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"nyxbot-go/internal/database"
	"nyxbot-go/internal/draw"
	modelwarframe "nyxbot-go/internal/model/warframe"
	"nyxbot-go/internal/warframe"
)

// TestImportRewardPoolKeepsCountTemplate 验证导入保留 |COUNT| 模板，序列化与渲染时展开。
func TestImportRewardPoolKeepsCountTemplate(t *testing.T) {
	setupImportFixDB(t)
	if err := database.DB.AutoMigrate(&modelwarframe.RewardPool{}, &modelwarframe.Reward{}); err != nil {
		t.Fatal(err)
	}
	previous := warframe.SetCDNFileFetcher(func(path string) ([]byte, error) {
		if path != "warframe/reward_pool.json" {
			t.Fatalf("CDN 文件路径应为 warframe/reward_pool.json, 实际 %q", path)
		}
		return []byte(`[{"uniqueName":"/Lotus/Rewards/Pool1","rewards":[
			{"id":"r1","item":"永冻晶矿 X |COUNT|","rarity":"COMMON","itemCount":100},
			{"id":"r2","item":"控制模块","rarity":"RARE","itemCount":2}
		]}]`), nil
	})
	t.Cleanup(func() { warframe.SetCDNFileFetcher(previous) })

	importer := warframe.NewDataImporter(warframe.NewExportFilePath(nil, "zh"), warframe.NewMarketAPI(nil), database.DB)
	if err := importer.ImportRewardPool(context.Background()); err != nil {
		t.Fatalf("ImportRewardPool: %v", err)
	}

	// 入库保留原始模板（不做导入时替换）
	var rewards []modelwarframe.Reward
	if err := database.DB.Order("id").Find(&rewards).Error; err != nil {
		t.Fatal(err)
	}
	if len(rewards) != 2 {
		t.Fatalf("expect 2 rewards, got %d: %+v", len(rewards), rewards)
	}
	if rewards[0].Item != "永冻晶矿 X |COUNT|" || rewards[0].ItemCount != 100 {
		t.Fatalf("模板未原样入库: %+v", rewards[0])
	}

	// 序列化展开（对齐 Java getItem()：API/导出输出替换后的文本）
	raw, err := json.Marshal(rewards[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"item":"永冻晶矿 X 100"`) || !strings.Contains(string(raw), `"rarity":"COMMON"`) {
		t.Fatalf("序列化应展开 |COUNT|: %s", string(raw))
	}

	// 渲染侧格式化：赏金卡片同样只显示一次数量
	if got := draw.FormatSyndicateReward(rewards[0].Item, rewards[0].ItemCount); got != "永冻晶矿 X 100" {
		t.Fatalf("渲染替换失败: %q", got)
	}
}
