// 仲裁节点分级导入测试（对齐 plans/arbitration-tier-plan.md §8.1）：
// 覆盖经 SetCDNFileFetcher 注入的 CDN 拉取链路、按 key 覆盖式 upsert、空 key 过滤，
// 以及空数据/非法载荷/拉取失败一律报错且不落库（纯 CDN，无本地兜底）。
package tests

import (
	"context"
	"errors"
	"testing"

	"nyxbot-go/internal/database"
	modelwarframe "nyxbot-go/internal/model/warframe"
	"nyxbot-go/internal/warframe"
)

// assertTier 断言库中某节点的评级。
func assertTier(t *testing.T, nodeKey, want string) {
	t.Helper()
	var row modelwarframe.ArbitrationTier
	if err := database.DB.Where(&modelwarframe.ArbitrationTier{Key: nodeKey}).First(&row).Error; err != nil {
		t.Fatalf("查询 %s 分级失败: %v", nodeKey, err)
	}
	if row.Tier != want {
		t.Fatalf("%s 评级应为 %s, got %s", nodeKey, want, row.Tier)
	}
}

// countArbitrationTiers 返回分级表行数。
func countArbitrationTiers(t *testing.T) int64 {
	t.Helper()
	var count int64
	if err := database.DB.Model(&modelwarframe.ArbitrationTier{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	return count
}

// TestImportArbitrationTiersUpsertsByKey 验证分级导入走 warframe/arbitration_tiers.json、
// 空 key 记录被过滤，且重复导入按 key 覆盖（评级变更更新原行、不产生重复行）。
func TestImportArbitrationTiersUpsertsByKey(t *testing.T) {
	setupImportFixDB(t)
	if err := database.DB.AutoMigrate(&modelwarframe.ArbitrationTier{}); err != nil {
		t.Fatal(err)
	}

	fetchedPath := ""
	previous := warframe.SetCDNFileFetcher(func(path string) ([]byte, error) {
		fetchedPath = path
		return []byte(`[
			{"key":"SolNode450","tier":"S"},
			{"key":"SolNode106","tier":"A"},
			{"key":"","tier":"S"}
		]`), nil
	})
	t.Cleanup(func() { warframe.SetCDNFileFetcher(previous) })

	if err := newNodeImporter(t).ImportArbitrationTiers(context.Background()); err != nil {
		t.Fatalf("ImportArbitrationTiers: %v", err)
	}
	if fetchedPath != "warframe/arbitration_tiers.json" {
		t.Fatalf("CDN 文件路径应为 warframe/arbitration_tiers.json, 实际 %q", fetchedPath)
	}

	assertTier(t, "SolNode450", "S")
	assertTier(t, "SolNode106", "A")
	if count := countArbitrationTiers(t); count != 2 {
		t.Fatalf("空 key 记录应被过滤，落库应为 2 行, got %d", count)
	}

	// 二次导入：评级变更覆盖原行，未出现的行保留（与 ImportAlias 一致的 upsert 语义）
	warframe.SetCDNFileFetcher(func(string) ([]byte, error) {
		return []byte(`[{"key":"SolNode450","tier":"B"}]`), nil
	})
	if err := newNodeImporter(t).ImportArbitrationTiers(context.Background()); err != nil {
		t.Fatalf("二次导入: %v", err)
	}
	assertTier(t, "SolNode450", "B")
	assertTier(t, "SolNode106", "A")
	if count := countArbitrationTiers(t); count != 2 {
		t.Fatalf("重复导入不应产生重复行, got %d", count)
	}
}

// TestImportArbitrationTiersRejectsBadPayload 空数组、非法 JSON、全空 key 与拉取失败
// 都应返回错误，且一行都不落库。
func TestImportArbitrationTiersRejectsBadPayload(t *testing.T) {
	setupImportFixDB(t)
	if err := database.DB.AutoMigrate(&modelwarframe.ArbitrationTier{}); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name      string
		body      string
		fetchFail bool
	}{
		{name: "空数组", body: `[]`},
		{name: "非法 JSON", body: `<html>not json</html>`},
		{name: "全空 key", body: `[{"key":"","tier":"S"}]`},
		{name: "拉取失败", fetchFail: true},
	}
	for _, tc := range cases {
		previous := warframe.SetCDNFileFetcher(func(string) ([]byte, error) {
			if tc.fetchFail {
				return nil, errors.New("cdn unavailable")
			}
			return []byte(tc.body), nil
		})
		err := newNodeImporter(t).ImportArbitrationTiers(context.Background())
		warframe.SetCDNFileFetcher(previous)
		if err == nil {
			t.Fatalf("%s 应返回错误", tc.name)
		}
	}

	if count := countArbitrationTiers(t); count != 0 {
		t.Fatalf("失败用例不应落库, got %d 行", count)
	}
}
