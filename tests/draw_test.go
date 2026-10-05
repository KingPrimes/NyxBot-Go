// internal/draw 绘图模块测试（阶段 9）：
// 覆盖 12 类 Warframe 图片的导出函数与空输入兜底，
// 生成的图片保留到项目根目录 temp/ 临时测试目录下
package tests

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"nyxbot-go/internal/draw"
	"nyxbot-go/internal/enum/drawplugin"
)

// drawOutDir 测试图片输出目录（项目根 temp/，主人指定保留）。
func drawOutDir(t *testing.T) string {
	t.Helper()
	root := projectRootDir(t)
	dir := filepath.Join(root, "temp")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// projectRootDir 向上查找含 go.mod 的项目根目录（go test 的 cwd 可能是包目录）。
func projectRootDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above cwd")
		}
		dir = parent
	}
}

// assertPNG 校验字节为有效 PNG 并写盘，返回解码尺寸。
func assertPNG(t *testing.T, name string, data []byte) (int, int) {
	t.Helper()
	if len(data) == 0 {
		t.Fatalf("%s: empty bytes", name)
	}
	if !bytes.HasPrefix(data, []byte{0x89, 'P', 'N', 'G'}) {
		t.Fatalf("%s: not a PNG (magic mismatch)", name)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("%s: decode png: %v", name, err)
	}
	path := filepath.Join(drawOutDir(t), name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("%s: write %s: %v", name, path, err)
	}
	bounds := img.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		t.Fatalf("%s: invalid size %dx%d", name, bounds.Dx(), bounds.Dy())
	}
	return bounds.Dx(), bounds.Dy()
}

// now 测试基准时间（固定偏移保证可重复）。
func now() time.Time { return time.Now() }

// TestDrawHelpImage 帮助图：多列标签 + 看板娘。
func TestDrawHelpImage(t *testing.T) {
	data := draw.DrawHelp([]string{
		"警报", "突击", "执刑官猎杀", "奸商", "每日特惠", "入侵", "裂隙", "九重天",
		"钢铁奖励", "平原", "三赏金", "双衍轮换", "电波", "1999日历", "仲裁", "仲裁表",
		"/WM", "/WR", "/CD", "/XT", "金垃圾", "银垃圾", "核桃查询", "开核桃",
	})
	w, _ := assertPNG(t, "draw_help.png", data)
	if w < 1000 {
		t.Fatalf("help image too narrow: %d", w)
	}
}

// TestDrawArbitrationImage 仲裁卡片图。
func TestDrawArbitrationImage(t *testing.T) {
	base := now()
	data := draw.DrawArbitration(&draw.Arbitration{
		Activation: base.Add(-30 * time.Minute),
		Expiry:     base.Add(90 * time.Minute),
		ID:         "arb-test",
		Node:       "谷神星 Bode",
		Enemy:      "Grineer",
		Type:       "拦截",
	})
	assertPNG(t, "draw_arbitration.png", data)

	// 不值得参与的仲裁（派系不匹配）
	data = draw.DrawArbitration(&draw.Arbitration{
		Activation: base.Add(-1 * time.Hour),
		Expiry:     base.Add(2 * time.Hour),
		Node:       "地球 Gaia",
		Enemy:      "Corpus",
		Type:       "捕获",
	})
	assertPNG(t, "draw_arbitration_notworth.png", data)
}

// TestFormatTimestampLocalTimeZone 验证时间文本按本地时区渲染（对齐 Java TimeZoneUtil 取系统时区）。
// API 时间为 UTC，若直接 Format 会与本地时间差一个时区偏移。
func TestFormatTimestampLocalTimeZone(t *testing.T) {
	original := time.Local
	t.Cleanup(func() { time.Local = original })

	utcNoon := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)

	time.Local = time.UTC
	if got := draw.FormatTimestamp(utcNoon); got != "2026-08-07 12:00:00" {
		t.Fatalf("UTC 时区: got %q, want %q", got, "2026-08-07 12:00:00")
	}

	time.Local = time.FixedZone("CST", 8*3600)
	if got := draw.FormatTimestamp(utcNoon); got != "2026-08-07 20:00:00" {
		t.Fatalf("UTC+8 时区: got %q, want %q", got, "2026-08-07 20:00:00")
	}
}

// TestDrawArbitrationsImage 值得参与的仲裁列表图。
func TestDrawArbitrationsImage(t *testing.T) {
	base := now()
	list := []*draw.Arbitration{
		{Activation: base.Add(-1 * time.Hour), Expiry: base.Add(1 * time.Hour), Node: "谷神星 Bode", Enemy: "Grineer", Type: "拦截"},
		{Activation: base.Add(-2 * time.Hour), Expiry: base.Add(30 * time.Minute), Node: "水星 M Prime", Enemy: "Infested", Type: "防御"},
		{Activation: base.Add(-3 * time.Hour), Expiry: base.Add(2 * time.Hour), Node: "地球 Gaia", Enemy: "Corpus", Type: "捕获"},
		{Activation: base.Add(-1 * time.Hour), Expiry: base.Add(4 * time.Hour), Node: "谷神星 Draco", Enemy: "Grineer", Type: "防御"},
	}
	assertPNG(t, "draw_arbitrations.png", draw.DrawArbitrations(list))
}

// TestDrawAllInfoImage 系统信息图。
func TestDrawAllInfoImage(t *testing.T) {
	total := int64(512 * 1024 * 1024 * 1024)
	used := int64(386 * 1024 * 1024 * 1024)
	gb := int64(1024 * 1024 * 1024 * 1024)
	gbUsed := int64(902 * 1024 * 1024 * 1024)
	data := draw.DrawAllInfo(&draw.AllInfo{
		CpuInfo: &draw.CpuInfo{
			Model:     "12th Gen Intel(R) Core(TM) i7-12700H",
			Cores:     14,
			Threads:   20,
			Frequency: 2.3,
			CacheSize: 24576,
			UserUsage: 23.456,
			WaitUsage: 1.2,
			SysUsage:  8.891,
			IdleUsage: 66.453,
		},
		PackageVersion: &draw.PackageVersion{Name: "NyxBot", Version: "1.0.8"},
		JvmInfo: &draw.JvmInfo{
			Version:         "17.0.9+9",
			MaxMemory:       8 * 1024 * 1024 * 1024,
			UsedMemory:      3 * 1024 * 1024 * 1024,
			FreeMemory:      5 * 1024 * 1024 * 1024,
			UsedMemoryRatio: 37.5,
			FreeMemoryRatio: 62.5,
		},
		SystemInfo: &draw.SystemInfo{
			OsName:       "Windows 11",
			OsArch:       "amd64",
			ComputerName: "DESKTOP-KP",
			ComputerIp:   "192.168.1.100",
		},
		SysFileInfos: &draw.SysFileInfos{
			SysFileInfos: []*draw.SysFileInfo{
				{DirName: "C:", TypeName: "本地磁盘", FileType: "NTFS", Total: &total, Used: &used},
				{DirName: "D:", TypeName: "本地磁盘", FileType: "NTFS", Total: &gb, Used: &gbUsed},
			},
		},
	})
	assertPNG(t, "draw_all_info.png", data)
}

// TestDrawActiveMissionImage 裂隙图（含虚空风暴/钢铁模式变体）。
func TestDrawActiveMissionImage(t *testing.T) {
	base := now()
	missions := []*draw.ActiveMission{
		{
			ID:          "am1",
			Activation:  base.Add(-10 * time.Minute),
			Expiry:      base.Add(20 * time.Minute),
			MissionType: drawplugin.MTSurvival,
			Modifier:    drawplugin.VoidT1,
			Node:        "地球 Tamu",
			Faction:     drawplugin.FactionGrineer,
		},
		{
			ID:          "am2",
			Activation:  base.Add(-5 * time.Minute),
			Expiry:      base.Add(55 * time.Minute),
			MissionType: drawplugin.MTDefense,
			Modifier:    drawplugin.VoidT4,
			Node:        "虚空 Mot",
			Faction:     drawplugin.FactionCorrupted,
			VoidStorms:  true,
		},
		{
			ID:          "am3",
			Activation:  base.Add(-2 * time.Minute),
			Expiry:      base.Add(8 * time.Minute),
			MissionType: drawplugin.MTExtermination,
			Modifier:    drawplugin.VoidT2,
			Node:        "火星 Olympus",
			Faction:     drawplugin.FactionCorpus,
			Hard:        true,
		},
	}
	assertPNG(t, "draw_active_mission.png", draw.DrawActiveMission(missions))
}

// TestDrawInvasionImage 入侵图（进攻/防守进度与奖励）。
func TestDrawInvasionImage(t *testing.T) {
	base := now()
	count := 400.0
	goal := 1000.0
	three := 3
	two := 2
	invasions := []*draw.Invasion{
		{
			ID:              "inv1",
			Activation:      base.Add(-1 * time.Hour),
			Expiry:          base.Add(2 * time.Hour),
			Faction:         drawplugin.FactionGrineer,
			DefenderFaction: drawplugin.FactionCorpus,
			Node:            "地球 Coba",
			Count:           &count,
			Goal:            &goal,
			AttackerReward:  []*draw.Reward{{Items: []string{"奥席金属 X", "内融核心"}, CountedItems: []*draw.RewardItem{{Name: "奥席金属", Count: &three}}}},
			DefenderReward:  &draw.Reward{Credits: 10000, CountedItems: []*draw.RewardItem{{Name: "合金板", Count: &two}}},
		},
		{
			ID:              "inv2",
			Activation:      base.Add(-30 * time.Minute),
			Expiry:          base.Add(3 * time.Hour),
			Faction:         drawplugin.FactionInfest,
			DefenderFaction: drawplugin.FactionGrineer,
			Node:            "谷神星 Draco",
			AttackerReward:  []*draw.Reward{{Items: []string{"未确认"}}},
			DefenderReward:  &draw.Reward{Credits: 5000},
		},
	}
	assertPNG(t, "draw_invasion.png", draw.DrawInvasion(invasions))
}

// TestDrawAllCycleImage 平原循环图（五平原周期）。
func TestDrawAllCycleImage(t *testing.T) {
	base := now()
	data := draw.DrawAllCycle(&draw.AllCycle{
		EarthCycle: &draw.EarthCycle{Activation: base.Add(-10 * time.Minute), Expiry: base.Add(80 * time.Minute), IsDay: true, State: "白昼"},
		CetusCycle: &draw.CetusCycle{IsDay: false, Expiry: base.Add(50 * time.Minute), Activation: base.Add(-70 * time.Minute), Cycle: "night", State: "夜晚"},
		CambionCycle: &draw.CambionCycle{Active: "FASS", TimeLeft: "45m", Expiry: base.Add(45 * time.Minute),
			Activation: base.Add(-45 * time.Minute)},
		VallisCycle: &draw.VallisCycle{IsWarm: false, TimeLeft: "1h", Expiry: base.Add(1 * time.Hour),
			Activation: base.Add(-1 * time.Hour)},
		ZarimanCycle: &draw.ZarimanCycle{IsCorpus: true, TimeLeft: "30m", Expiry: base.Add(30 * time.Minute),
			Activation: base.Add(-30 * time.Minute)},
	})
	assertPNG(t, "draw_all_cycle.png", data)
}

// TestDrawMarketOrdersImage 市场订单图 + 可能要查询的物品列表图。
func TestDrawMarketOrdersImage(t *testing.T) {
	base := now()
	platinum := 45
	quantity := 2
	rank := 5
	reputation := 1234
	price := 300
	orderType := drawplugin.TransBuy
	orders := &draw.Orders{
		Name:   "Rhino Prime 蓝图",
		Form:   draw.MarketPlatformPC,
		IsBy:   boolPtr(true),
		IsMax:  boolPtr(false),
		Ducats: &price,
		Orders: []*draw.OrderWithUser{
			{
				ID:       "o1",
				Type:     orderType,
				Platinum: &platinum,
				Quantity: &quantity,
				Rank:     &rank,
				User: &draw.MarketUser{
					ID:         "u1",
					IngameName: "KingPrimes",
					Reputation: &reputation,
					Platform:   draw.MarketPlatformPC,
					Status:     drawplugin.MarketStatusOnline,
					Activity:   &draw.MarketUserActivity{Type: drawplugin.MarketActInOrbiter},
					LastSeen:   base.Add(-5 * time.Minute),
				},
			},
		},
	}
	assertPNG(t, "draw_market_orders.png", draw.DrawMarketOrders(orders))
	assertPNG(t, "draw_market_possible_items.png", draw.DrawMarketOrdersList([]string{"Rhino Prime 蓝图", "Soma Prime 枪机", "Vectis Prime"}))
}

// TestDrawMarketRivenImage 紫卡拍卖图。
func TestDrawMarketRivenImage(t *testing.T) {
	base := now()
	price := 500
	rep := 888
	value := 1.5
	positive := true
	data := draw.DrawMarketRiven(&draw.MarketRiven{
		ItemName: "步枪",
		Payload: &draw.MarketRivenPayload{
			Auctions: []*draw.MarketRivenAuction{
				{
					BuyoutPrice: &price,
					Note:        "极品词条",
					Visible:     boolPtr(true),
					Item: &draw.MarketRivenItem{
						Type:          "riven",
						ModRank:       intPtr(0),
						Name:          "极品步枪",
						WeaponURLName: "rubico_prime",
						Polarity:      drawplugin.PolarityMadurai,
						MasteryLevel:  intPtr(16),
						Attributes: []*draw.MarketRivenAttribute{
							{Value: &value, Positive: &positive, URLName: "damage"},
						},
					},
					StartingPrice:     &price,
					MinimalReputation: &rep,
					Owner: &draw.MarketRivenOwner{
						Reputation: &rep,
						Locale:     "zh",
						LastSeen:   base.Add(-10 * time.Minute),
					},
					Platform: "pc",
					Created:  base.Add(-2 * time.Hour),
					Updated:  base.Add(-1 * time.Hour),
				},
			},
		},
	})
	assertPNG(t, "draw_market_riven.png", data)
}

// TestDrawMarketLichSisterImage 玄骸/姐妹拍卖图。
func TestDrawMarketLichSisterImage(t *testing.T) {
	base := now()
	price := 200
	rep := 5
	data := draw.DrawMarketLichSister(&draw.MarketLichSister{
		Payload: &draw.MarketLichSisterPayload{
			ItemName: "信条·铁钩手甲",
			Auctions: []*draw.MarketLichSisterAuction{
				{
					BuyoutPrice: &price,
					Note:        "带幻纹",
					Visible:     boolPtr(true),
					Item: &draw.MarketLichSisterItem{
						Type:           "weapon",
						Damage:         intPtr(45),
						WeaponURLName:  "tenet_grattler",
						HavingEphemera: boolPtr(true),
						Element:        drawplugin.ElemToxin,
					},
					StartingPrice:     &price,
					MinimalReputation: &rep,
					Owner: &draw.MarketLichSisterOwner{
						Reputation: &rep,
						Locale:     "zh",
						LastSeen:   base.Add(-20 * time.Minute),
					},
					Platform: "pc",
					Created:  base.Add(-1 * time.Hour),
					Updated:  base.Add(-30 * time.Minute),
				},
			},
		},
	})
	assertPNG(t, "draw_market_lich_sister.png", data)
}

// TestDrawRelicsImage 遗物查询图。
func TestDrawRelicsImage(t *testing.T) {
	two := 2
	data := draw.DrawRelics([]*draw.Relics{
		{
			Name: "Lith A1",
			Rewards: []*draw.RelicReward{
				{Name: "Rhino Prime 蓝图", Rarity: drawplugin.RarityCommon},
				{Name: "合金板", Rarity: drawplugin.RarityUncommon, ItemCount: &two},
				{Name: "Forma 蓝图", Rarity: drawplugin.RarityRare},
			},
		},
		{
			Name: "Meso B2",
			Rewards: []*draw.RelicReward{
				{Name: "Soma Prime 枪机", Rarity: drawplugin.RarityUncommon},
				{Name: "Soma Prime 蓝图", Rarity: drawplugin.RarityRare},
			},
		},
	})
	assertPNG(t, "draw_relics.png", data)
}

// TestDrawWarframeSubscribeImage 订阅帮助图。
func TestDrawWarframeSubscribeImage(t *testing.T) {
	data := draw.DrawWarframeSubscribe(
		map[int]string{1: "裂隙", 2: "仲裁", 3: "入侵", 4: "警报", 5: "赏金", 6: "奸商", 7: "电波", 8: "突击"},
		map[int]string{1: "救援", 2: "破坏", 3: "捕获", 4: "歼灭", 5: "防御", 6: "生存", 7: "拦截"},
		map[int]string{1: "合金板", 2: "奥席金属", 3: "内融核心"},
	)
	assertPNG(t, "draw_subscribe.png", data)
}

// TestDrawRivenAnalyseTrendImage 紫卡分析趋势图。
func TestDrawRivenAnalyseTrendImage(t *testing.T) {
	num := 1.25
	attr := 250.0
	data := draw.DrawRivenAnalyseTrend([]*draw.RivenAnalyseTrend{
		{
			WeaponName: "Phenmor",
			RivenName:  "步枪",
			Num:        &num,
			WeaponType: "步枪",
			Attributes: []draw.RivenAnalyseTrendAttribute{
				{Name: "伤害", AttributeName: "伤害", Attr: &attr, LowAttr: "150.0", HighAttr: "250.0", Grade: "S", LethalLevel: "fatal", Analysis: "收益极高"},
				{Name: "多重射击", AttributeName: "多重射击", Attr: &attr, LowAttr: "100.0", HighAttr: "200.0", Grade: "A", Analysis: "收益较高"},
			},
		},
		{
			WeaponName: "Rubico Prime",
			RivenName:  "步枪",
			Dot:        "●●●○○",
			WeaponType: "狙击枪",
			Attributes: []draw.RivenAnalyseTrendAttribute{
				{Name: "暴击几率", AttributeName: "暴击几率", Attr: &attr, LowAttr: "80.0", HighAttr: "120.0", Grade: "B", Analysis: "收益中等"},
			},
		},
	})
	assertPNG(t, "draw_riven_analyse_trend.png", data)
}

// TestDrawEmptyInputs 空输入兜底：全部导出函数应返回 nil。
func TestDrawEmptyInputs(t *testing.T) {
	if data := draw.DrawHelp(nil); data != nil {
		t.Fatal("DrawHelp(nil) should return nil")
	}
	if data := draw.DrawArbitration(nil); data != nil {
		t.Fatal("DrawArbitration(nil) should return nil")
	}
	if data := draw.DrawArbitrations(nil); data != nil {
		t.Fatal("DrawArbitrations(nil) should return nil")
	}
	if data := draw.DrawAllInfo(nil); data != nil {
		t.Fatal("DrawAllInfo(nil) should return nil")
	}
	if data := draw.DrawActiveMission(nil); data != nil {
		t.Fatal("DrawActiveMission(nil) should return nil")
	}
	if data := draw.DrawInvasion(nil); data != nil {
		t.Fatal("DrawInvasion(nil) should return nil")
	}
	if data := draw.DrawAllCycle(nil); data != nil {
		t.Fatal("DrawAllCycle(nil) should return nil")
	}
	if data := draw.DrawMarketOrders(nil); data != nil {
		t.Fatal("DrawMarketOrders(nil) should return nil")
	}
	if data := draw.DrawMarketOrdersList(nil); data != nil {
		t.Fatal("DrawMarketOrdersList(nil) should return nil")
	}
	if data := draw.DrawMarketRiven(&draw.MarketRiven{}); data != nil {
		t.Fatal("DrawMarketRiven(empty) should return nil")
	}
	if data := draw.DrawMarketLichSister(&draw.MarketLichSister{}); data != nil {
		t.Fatal("DrawMarketLichSister(empty) should return nil")
	}
	if data := draw.DrawRelics(nil); data != nil {
		t.Fatal("DrawRelics(nil) should return nil")
	}
	if data := draw.DrawWarframeSubscribe(nil, nil, nil); data != nil {
		t.Fatal("DrawWarframeSubscribe(nil,nil,nil) should return nil")
	}
	if data := draw.DrawRivenAnalyseTrend(nil); data != nil {
		t.Fatal("DrawRivenAnalyseTrend(nil) should return nil")
	}
}

// boolPtr / intPtr 指针构造辅助。
func boolPtr(v bool) *bool { return &v }
func intPtr(v int) *int    { return &v }
