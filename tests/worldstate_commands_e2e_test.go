// 世界状态指令「真实载荷 → 指令回复图片」全链路回归测试。
//
// 这是用户报错场景的直接复现路径：机器人收到指令 → CommandRegistry 派发 → 世界状态查询
// → 绘图 → 回复图片。修复前链路会在查询层抛
// `parse world state envelope failed: json: cannot unmarshal object into ... _id of type string`，
// 指令只能回复「世界状态数据解析失败」文本；本用例对每条世界状态指令断言「回复的是图片」，
// 载荷与官方 worldState.php 逐字段一致（见 worldstate_extended_json_test.go 的 realPayloadRaw）。
package tests

import (
	"strings"
	"testing"
	"time"

	zero "github.com/wdvxdr1123/ZeroBot"

	"nyxbot-go/internal/database"
	modelbot "nyxbot-go/internal/model/bot"
	modelsystem "nyxbot-go/internal/model/system"
	modelwarframe "nyxbot-go/internal/model/warframe"
	"nyxbot-go/internal/onebot"
	"nyxbot-go/internal/warframe"
)

// worldStateCommands 覆盖全部依赖 WorldState 原始数据的指令（触发词取自 nyxbot.Codes）。
var worldStateCommands = []string{
	"突击", "裂隙", "钢铁裂隙", "九重天", "入侵", "奸商", "每日特惠",
	"警报", "执刑官猎杀", "平原", "希图斯", "双衍王境", "电波", "1999",
}

// TestWorldStateCommandsEndToEnd 验证真实载荷下每条世界状态指令都能回复图片而非报错文案。
func TestWorldStateCommandsEndToEnd(t *testing.T) {
	setupWorldStateBatchDB(t)
	// 指令执行日志（对齐 commands.go 的 log_info 写入）
	if err := database.DB.AutoMigrate(&modelsystem.LogInfo{},
		&modelbot.BotAdmin{}, &modelbot.GroupWhite{}, &modelbot.ProveWhite{},
		&modelbot.GroupBlack{}, &modelbot.ProveBlack{}); err != nil {
		t.Fatal(err)
	}
	seedWorldStateCommandData(t)

	// 执刑官猎杀用未来时间，保证「剩余时间」列非空
	warframe.DefaultWorldState().SetRaw([]byte(realPayloadRaw(time.Now().Add(2 * time.Hour).UnixMilli())))

	caller := &recordingCaller{}
	const botUID = int64(13001)
	zero.APICallers.Store(botUID, caller)
	previousConfig := zero.BotConfig
	zero.BotConfig = zero.Config{MaxProcessTime: 15 * time.Second}
	registry := onebot.NewCommandRegistry(false)
	if err := registry.Register(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		registry.Close()
		zero.BotConfig = previousConfig
		zero.APICallers.Delete(botUID)
	})

	bot := zero.GetBot(botUID)
	if bot == nil {
		t.Fatal("test Bot 不可用")
	}

	for index, command := range worldStateCommands {
		before := len(caller.snapshot())
		echoPrivateMessage(t, bot, botUID, int64(23001+index), command)
		waitFor(t, func() bool { return len(caller.snapshot()) > before },
			"指令 "+command+" 未回复")
		requests := caller.snapshot()
		reply := requests[len(requests)-1].Params["message"]
		if !replyContainsImage(reply) {
			t.Fatalf("指令 %q 应回复图片，实际回复文本: %q（真实载荷解析链路可能再次回归）",
				command, replyText(reply))
		}
		if text := replyText(reply); strings.Contains(text, "解析失败") {
			t.Fatalf("指令 %q 命中世界状态解析失败: %q", command, text)
		}
	}
}

// seedWorldStateCommandData 置入指令链路需要的本地表数据（节点/赏金翻译/奖励池/电波挑战）。
func seedWorldStateCommandData(t *testing.T) {
	t.Helper()
	nodes := []modelwarframe.Nodes{
		{UniqueName: "SolNode61", Name: "谷神星 Bode", SystemName: "谷神星", FactionIndex: 0, MissionIndex: 5},
		{UniqueName: "SolNode153", Name: "木星 Elara", SystemName: "木星", FactionIndex: 1, MissionIndex: 2},
		{UniqueName: "CrewBattleNode522", Name: "比邻星 卡律布狄斯", SystemName: "面纱比邻星", FactionIndex: 0, MissionIndex: 60},
		{UniqueName: "SolNode24", Name: "地球 珠穆朗玛", SystemName: "地球", FactionIndex: 0, MissionIndex: 9},
		{UniqueName: "SolNode73", Name: "火星 席芭莉丝", SystemName: "火星", FactionIndex: 0, MissionIndex: 7},
		{UniqueName: "SolNode102", Name: "海王星 拉里萨", SystemName: "海王星", FactionIndex: 0, MissionIndex: 1},
		{UniqueName: "SaturnHUB", Name: "土星 克罗尼娅", SystemName: "土星", FactionIndex: 0, MissionIndex: 100},
		{UniqueName: "SolNode72", Name: "天王星 卡德卢斯", SystemName: "天王星", FactionIndex: 0, MissionIndex: 4},
	}
	for i := range nodes {
		if err := database.DB.Create(&nodes[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	// 赏金任务类型/描述按最后三段查 state_translation（对齐 Java getLastThreeSegments）
	if err := database.DB.Create(&modelwarframe.StateTranslation{
		UniqueName: "Levels/Jobs/CetusBounty1", Name: "赏金任务", Description: "清剿感染体", Type: 0,
	}).Error; err != nil {
		t.Fatal(err)
	}
	pool := modelwarframe.RewardPool{UniqueName: "/Lotus/Rewards/Pool1"}
	if err := database.DB.Create(&pool).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.DB.Create(&modelwarframe.Reward{
		ID: "r1", PoolID: pool.UniqueName, Item: "现金", Rarity: 1, ItemCount: 5000,
	}).Error; err != nil {
		t.Fatal(err)
	}
	// 电波挑战：standing 1000/4500/7000 分别对应每日/每周/精英（对齐 Java NightWave）
	for _, record := range []modelwarframe.NightWave{
		{UniqueName: "/Lotus/Types/Challenges/NW1", Name: "击杀敌人",
			Description: "击杀 |COUNT| 名敌人", Standing: 4500, Required: 30},
		{UniqueName: "/Lotus/Types/Challenges/NW2", Name: "完成任务",
			Description: "完成 |COUNT| 次任务", Standing: 1000, Required: 3},
		{UniqueName: "/Lotus/Types/Challenges/NW3", Name: "精英挑战",
			Description: "击杀 |COUNT| 名执刑官", Standing: 7000, Required: 1},
	} {
		if err := database.DB.Create(&record).Error; err != nil {
			t.Fatal(err)
		}
	}
}
