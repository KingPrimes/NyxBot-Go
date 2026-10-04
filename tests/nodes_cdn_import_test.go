// 星图节点导入回归测试（对齐 Java NodeService.initData：ExportRegions 导出 + CDN nodes.json）：
// 覆盖两种载荷解析、CDN 只补官方导出缺失的节点（同名节点保留官方导出数据）、空主键过滤，
// 以及 CDN 不可用时保留官方导出数据。
package tests

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nyxbot-go/internal/database"
	"nyxbot-go/internal/logging"
	modelwarframe "nyxbot-go/internal/model/warframe"
	"nyxbot-go/internal/warframe"
)

// TestParseNodesExportAndCDN 验证两种节点载荷的解析：
// ExportRegions 取顶层数组、CDN nodes.json 为裸数组，字段与模型同构，空 uniqueName 一律丢弃。
func TestParseNodesExportAndCDN(t *testing.T) {
	exportRecords, err := warframe.ParseNodesExport([]byte(`{"ExportRegions":[
		{"uniqueName":"SolNode94","name":"阿波罗多罗斯撞击坑","systemName":"水星","systemIndex":0,"nodeType":0,"masteryReq":0,"missionIndex":2,"factionIndex":2,"minEnemyLevel":6,"maxEnemyLevel":11},
		{"uniqueName":"","name":"空主键节点","systemName":"水星"}
	]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(exportRecords) != 1 {
		t.Fatalf("expect 1 export node (空主键被过滤), got %d: %+v", len(exportRecords), exportRecords)
	}
	first := exportRecords[0]
	if first.UniqueName != "SolNode94" || first.Name != "阿波罗多罗斯撞击坑" || first.SystemName != "水星" ||
		first.MissionIndex != 2 || first.FactionIndex != 2 || first.MinEnemyLevel != 6 || first.MaxEnemyLevel != 11 {
		t.Fatalf("export node fields mismatch: %+v", first)
	}

	cdnRecords, err := warframe.ParseNodesCDN([]byte(`[
		{"uniqueName":"CrewBattleNode501","name":"魔多星团","systemName":"土星比邻星域","systemIndex":0,"nodeType":0,"masteryReq":0,"missionIndex":60,"factionIndex":0,"minEnemyLevel":55,"maxEnemyLevel":60},
		{"uniqueName":"","name":"空主键节点"}
	]`))
	if err != nil {
		t.Fatal(err)
	}
	if len(cdnRecords) != 1 {
		t.Fatalf("expect 1 CDN node (空主键被过滤), got %d: %+v", len(cdnRecords), cdnRecords)
	}
	if cdnRecords[0].UniqueName != "CrewBattleNode501" || cdnRecords[0].SystemName != "土星比邻星域" ||
		cdnRecords[0].MissionIndex != 60 {
		t.Fatalf("CDN node fields mismatch: %+v", cdnRecords[0])
	}

	// 顶层键缺失视为解析失败（对齐 Java：rootNode.get(key) 为 null 时抛异常）；非法 JSON 同样失败
	if _, err := warframe.ParseNodesExport([]byte(`{"ExportWeapons":[]}`)); err == nil {
		t.Fatal("missing ExportRegions key should fail, 对齐 Java parseFromExport")
	}
	if _, err := warframe.ParseNodesExport([]byte(`{`)); err == nil {
		t.Fatal("invalid export JSON should fail")
	}
	if _, err := warframe.ParseNodesCDN([]byte(`{`)); err == nil {
		t.Fatal("invalid CDN JSON should fail")
	}
}

// setupNodesExportFile 写出 tests/data/export/ExportRegions_zh.json 并登记清理。
// ExportFilePath 的 ./data/export 相对进程工作目录（go test 为 tests/）。
func setupNodesExportFile(t *testing.T, body string) {
	t.Helper()
	workDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	exportPath := filepath.Join(workDir, "data", "export", "ExportRegions_zh.json")
	if err := os.MkdirAll(filepath.Dir(exportPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exportPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Remove(exportPath)
		_ = os.Remove(filepath.Dir(exportPath)) // 目录非空时 Remove 失败，属预期
	})
}

// newNodeImporter 构造指向全局测试库的导入器。
func newNodeImporter(t *testing.T) *warframe.DataImporter {
	t.Helper()
	return warframe.NewDataImporter(warframe.NewExportFilePath(nil, "zh"), warframe.NewMarketAPI(nil), database.DB)
}

// TestImportNodesAddsCDNOnlyNodes 验证节点导入会同时落官方导出与 CDN nodes.json：
// CDN 独有节点（九重天/活动节点）被补进库，同名节点保持官方导出数据
// （Java 的 saveAll 会整行覆盖，实测会把 SolNode229 的派系/等级冲成 CDN 的旧值）。
func TestImportNodesAddsCDNOnlyNodes(t *testing.T) {
	setupImportFixDB(t)
	if err := database.DB.AutoMigrate(&modelwarframe.Nodes{}); err != nil {
		t.Fatal(err)
	}
	setupNodesExportFile(t, `{"ExportRegions":[
		{"uniqueName":"SolNode94","name":"阿波罗多罗斯撞击坑","systemName":"水星","systemIndex":0,"nodeType":0,"masteryReq":0,"missionIndex":2,"factionIndex":2,"minEnemyLevel":6,"maxEnemyLevel":11},
		{"uniqueName":"SolNode229","name":"魔胎之境","systemName":"火卫二","systemIndex":16,"nodeType":0,"masteryReq":0,"missionIndex":28,"factionIndex":2,"minEnemyLevel":15,"maxEnemyLevel":30}
	]}`)

	fetchedPath := ""
	previous := warframe.SetCDNFileFetcher(func(path string) ([]byte, error) {
		fetchedPath = path
		return []byte(`[
			{"uniqueName":"CrewBattleNode501","name":"魔多星团","systemName":"土星比邻星域","systemIndex":0,"nodeType":0,"masteryReq":0,"missionIndex":60,"factionIndex":0,"minEnemyLevel":55,"maxEnemyLevel":60},
			{"uniqueName":"SolNode229","name":"魔胎之境","systemName":"火卫二","systemIndex":0,"nodeType":0,"masteryReq":0,"missionIndex":28,"factionIndex":1,"minEnemyLevel":0,"maxEnemyLevel":0},
			{"uniqueName":"","name":"空主键节点"}
		]`), nil
	})
	t.Cleanup(func() { warframe.SetCDNFileFetcher(previous) })

	if err := newNodeImporter(t).ImportNodes(context.Background()); err != nil {
		t.Fatalf("ImportNodes: %v", err)
	}
	if fetchedPath != "warframe/nodes.json" {
		t.Fatalf("CDN 文件路径应为 warframe/nodes.json, 实际 %q", fetchedPath)
	}

	var nodes []modelwarframe.Nodes
	if err := database.DB.Order("unique_name").Find(&nodes).Error; err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 3 {
		t.Fatalf("expect 3 nodes (2 导出 + 1 CDN 独有，空主键不入库), got %d: %+v", len(nodes), nodes)
	}
	byKey := make(map[string]modelwarframe.Nodes, len(nodes))
	for _, node := range nodes {
		if node.UniqueName == "" {
			t.Fatalf("空 uniqueName 不应入库: %+v", node)
		}
		byKey[node.UniqueName] = node
	}
	// CDN 独有节点（官方导出里没有）
	crew, ok := byKey["CrewBattleNode501"]
	if !ok || crew.Name != "魔多星团" || crew.MissionIndex != 60 || crew.MinEnemyLevel != 55 {
		t.Fatalf("CDN 独有节点未导入: %+v", crew)
	}
	// 导出独有节点保持原样
	if sol94 := byKey["SolNode94"]; sol94.Name != "阿波罗多罗斯撞击坑" || sol94.MaxEnemyLevel != 11 {
		t.Fatalf("导出独有节点被破坏: %+v", sol94)
	}
	// 同名节点保留官方导出数据（CDN 只补缺；factionIndex 会用于入侵/警报派系渲染）
	if sol229 := byKey["SolNode229"]; sol229.FactionIndex != 2 || sol229.SystemIndex != 16 ||
		sol229.MinEnemyLevel != 15 || sol229.MaxEnemyLevel != 30 {
		t.Fatalf("同名节点应保留官方导出数据: %+v", sol229)
	}
}

// TestImportNodesKeepsExportWhenCDNFails 验证 CDN 不可用/内容非法时：
// 导入不报错（不阻断启动与更新任务），官方导出节点仍然落库，并留下 WARN 日志。
func TestImportNodesKeepsExportWhenCDNFails(t *testing.T) {
	setupImportFixDB(t)
	if err := database.DB.AutoMigrate(&modelwarframe.Nodes{}); err != nil {
		t.Fatal(err)
	}
	setupNodesExportFile(t, `{"ExportRegions":[
		{"uniqueName":"SolNode94","name":"阿波罗多罗斯撞击坑","systemName":"水星","systemIndex":0,"nodeType":0,"masteryReq":0,"missionIndex":2,"factionIndex":2,"minEnemyLevel":6,"maxEnemyLevel":11}
	]}`)

	for _, source := range []string{"unavailable", "invalid"} {
		previous := warframe.SetCDNFileFetcher(func(string) ([]byte, error) {
			if source == "unavailable" {
				return nil, os.ErrDeadlineExceeded
			}
			return []byte(`{not json`), nil
		})
		err := newNodeImporter(t).ImportNodes(context.Background())
		warframe.SetCDNFileFetcher(previous)
		if err != nil {
			t.Fatalf("CDN %s 时不应让导入失败: %v", source, err)
		}
		var count int64
		if err := database.DB.Model(&modelwarframe.Nodes{}).Where("unique_name = ?", "SolNode94").Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("CDN %s 时导出节点应仍可用, count=%d", source, count)
		}
	}

	hit := false
	for _, entry := range logging.Recent(logging.LevelWarn) {
		if entry.Pack == "warframe.import" && strings.Contains(entry.Message, "CDN nodes.json") {
			hit = true
		}
	}
	if !hit {
		t.Fatal("CDN 不可用时应有 warframe.import 的 WARN 日志")
	}
}
