// 仲裁节点分级导入测试（对齐 plans/arbitration-tier-plan.md §8.1）：
// 覆盖经 SetCDNFileFetcher 注入的 CDN 拉取链路、按 key 覆盖式 upsert、空 key 过滤，
// 以及空数据/非法载荷/拉取失败一律报错且不落库（纯 CDN，无本地兜底）。
package tests

import (
	"context"
	"errors"
	"testing"

	"gorm.io/gorm"

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

// assertTierAbsent 断言库中不存在某节点的分级行。
func assertTierAbsent(t *testing.T, nodeKey string) {
	t.Helper()
	err := database.DB.Where(&modelwarframe.ArbitrationTier{Key: nodeKey}).
		First(&modelwarframe.ArbitrationTier{}).Error
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("%s 应已被快照式替换删除, got err=%v", nodeKey, err)
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

// TestImportArbitrationTiersReplacesSnapshot 验证分级导入走 warframe/arbitration_tiers.json、
// 空 key 记录被过滤，且 CDN 文件被当作**权威快照**：二次导入未出现的 key 会被删除
// （否则节点被移除后旧评级会残留，继续影响前瞻列表——PR #15 评审），评级变更则覆盖原行。
func TestImportArbitrationTiersReplacesSnapshot(t *testing.T) {
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

	// 二次导入：快照未包含 SolNode106（应被删除），SolNode450 评级改为 B（应覆盖原行）
	warframe.SetCDNFileFetcher(func(string) ([]byte, error) {
		return []byte(`[{"key":"SolNode450","tier":"B"}]`), nil
	})
	if err := newNodeImporter(t).ImportArbitrationTiers(context.Background()); err != nil {
		t.Fatalf("二次导入: %v", err)
	}
	assertTier(t, "SolNode450", "B")
	if count := countArbitrationTiers(t); count != 1 {
		t.Fatalf("快照式替换后应只剩 1 行, got %d", count)
	}
	assertTierAbsent(t, "SolNode106")
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
