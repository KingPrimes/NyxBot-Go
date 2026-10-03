// 阶段10 订阅指令黑盒测试：ordinal 契约、订阅/取消/列表落库逻辑。
package tests

import (
	"strings"
	"testing"

	"nyxbot-go/internal/database"
	"nyxbot-go/internal/enum/nyxbot"
	modelwarframe "nyxbot-go/internal/model/warframe"
	"nyxbot-go/internal/warframe"
)

// TestSubscribeCodeContract 验证 code=enum 下标契约（对齐 Java values()[code]）。
func TestSubscribeCodeContract(t *testing.T) {
	cases := []struct {
		code int
		want nyxbot.SubscribeType
	}{
		{1, nyxbot.SubArbitration},
		{5, nyxbot.SubInvasions},
		{8, nyxbot.SubFissures},
		{9, nyxbot.SubNews},
		{13, nyxbot.SubDuviriCycle},
	}
	for _, c := range cases {
		got, ok := warframe.ParseSubscribeType(c.code)
		if !ok || got != c.want {
			t.Fatalf("code=%d 应解析为 %v, 实际 %v ok=%v", c.code, c.want, got, ok)
		}
	}
	for _, bad := range []int{0, 14, -1} {
		if _, ok := warframe.ParseSubscribeType(bad); ok {
			t.Fatalf("code=%d 应非法", bad)
		}
	}
	if got, ok := warframe.ParseInvasionReward(1); !ok || got != nyxbot.InvRewardDetoniteInjector {
		t.Fatalf("入侵奖励 code1 应为突变原聚合物, 实际 %v", got)
	}
	if got, ok := warframe.ParseMissionType(1); !ok || got != nyxbot.MTSurvival {
		t.Fatalf("任务类型 code1 应为生存, 实际 %v", got)
	}
}

// TestSubscribeSubParameterCodeBoundaries verifies the first/last accepted
// mission and reward codes and rejects their adjacent out-of-range values.
func TestSubscribeSubParameterCodeBoundaries(t *testing.T) {
	missionCases := []struct {
		code int
		want nyxbot.MissionType
	}{
		{1, nyxbot.MTSurvival},
		{26, nyxbot.MTCorruption},
	}
	for _, test := range missionCases {
		got, ok := warframe.ParseMissionType(test.code)
		if !ok || got != test.want {
			t.Errorf("mission code %d = %q, %v; want %q, true", test.code, got, ok, test.want)
		}
	}
	for _, code := range []int{-1, 0, 27} {
		if got, ok := warframe.ParseMissionType(code); ok {
			t.Errorf("mission code %d unexpectedly parsed as %q", code, got)
		}
	}

	rewardCases := []struct {
		code int
		want nyxbot.InvasionReward
	}{
		{1, nyxbot.InvRewardDetoniteInjector},
		{7, nyxbot.InvRewardExilusAdapter},
	}
	for _, test := range rewardCases {
		got, ok := warframe.ParseInvasionReward(test.code)
		if !ok || got != test.want {
			t.Errorf("reward code %d = %q, %v; want %q, true", test.code, got, ok, test.want)
		}
	}
	for _, code := range []int{-1, 0, 8} {
		if got, ok := warframe.ParseInvasionReward(code); ok {
			t.Errorf("reward code %d unexpectedly parsed as %q", code, got)
		}
	}
}

// TestSubscribeLifecycle 验证订阅/重复/列出/取消的完整流程。
func TestSubscribeLifecycle(t *testing.T) {
	setupStage10DB(t)
	const groupID, userID = 123456789, 987654321

	cmd := &warframe.SubscribeCommand{BotUID: 1, GroupID: groupID, GroupName: "测试群", UserID: userID, UserName: "测试用户"}
	if !warframe.ParseSubscribeCommand(cmd, "8-1-4") {
		t.Fatal("解析订阅 8-1-4 失败")
	}
	if cmd.SubType != nyxbot.SubFissures || cmd.Mission == nil || *cmd.Mission != nyxbot.MTSurvival ||
		cmd.Tier == nil || *cmd.Tier != 4 {
		t.Fatalf("订阅解析错误: %+v", cmd)
	}
	if got := warframe.Subscribe(cmd); !strings.Contains(got, "订阅成功") {
		t.Fatalf("首次订阅应成功: %q", got)
	}
	if got := warframe.Subscribe(cmd); !strings.Contains(got, "已订阅") {
		t.Fatalf("重复订阅应提示已订阅: %q", got)
	}
	if info := warframe.UserSubscriptionInfo(groupID, userID); info == "" ||
		!strings.Contains(info, "裂隙") || !strings.Contains(info, "生存") {
		t.Fatalf("订阅列表应含裂隙 生存: %q", info)
	}
	var ruleCount int64
	if err := database.DB.Model(&modelwarframe.MissionSubscribeUserCheckType{}).Count(&ruleCount).Error; err != nil {
		t.Fatal(err)
	}
	if ruleCount != 1 {
		t.Fatalf("应有 1 条规则, 实际 %d", ruleCount)
	}

	subType, _, _, _, ok := warframe.ParseUnsubscribeParams(8, []string{"8"})
	if !ok {
		t.Fatal("解析取消类型失败")
	}
	if got := warframe.Unsubscribe(groupID, userID, subType, nil, nil, nil); !strings.Contains(got, "成功取消 1") {
		t.Fatalf("取消应移除 1 条: %q", got)
	}
	if got := warframe.UserSubscriptionInfo(groupID, userID); got != "" {
		t.Fatalf("取消后列表应为空: %q", got)
	}
	var groupCount int64
	if err := database.DB.Model(&modelwarframe.MissionSubscribe{}).Where("sub_group = ?", groupID).Count(&groupCount).Error; err != nil {
		t.Fatal(err)
	}
	if groupCount != 0 {
		t.Fatalf("空订阅组应删除, 剩余 %d", groupCount)
	}
}

// TestSubscribeHelpEnums 验证帮助图编号表排除 0 且编号正确。
func TestSubscribeHelpEnums(t *testing.T) {
	sub, mission, reward := warframe.SubscribeHelpEnums()
	if _, ok := sub[0]; ok {
		t.Fatal("帮助图不应含编号 0")
	}
	if sub[8] != "裂隙" {
		t.Fatalf("编号 8 应为裂隙, 实际 %q", sub[8])
	}
	if reward[1] == "" || mission[1] != "生存" {
		t.Fatalf("帮助枚举映射异常")
	}
}

// TestUnsubscribeNoMatch 取消不存在的规则应返回未找到（对齐 Java toRemove.isEmpty()）。
func TestUnsubscribeNoMatch(t *testing.T) {
	setupStage10DB(t)
	const groupID, userID = 123456788, 987654322
	cmd := &warframe.SubscribeCommand{BotUID: 1, GroupID: groupID, GroupName: "群", UserID: userID, UserName: "用户"}
	if !warframe.ParseSubscribeCommand(cmd, "8-1-4") {
		t.Fatal("解析订阅失败")
	}
	warframe.Subscribe(cmd)
	// 匹配不到 8-1-3 的规则。
	if got := warframe.Unsubscribe(groupID, userID, nyxbot.SubFissures, ptr(nyxbot.MTSurvival), ptr(3), nil); got != "未找到匹配的订阅规则" {
		t.Fatalf("无匹配时应返回未找到: %q", got)
	}
	// A negative cancellation must not trigger empty-user cleanup or remove the
	// unmatched rule, its owner, or the containing group.
	for name, model := range map[string]any{
		"subscription groups": &modelwarframe.MissionSubscribe{},
		"subscription users":  &modelwarframe.MissionSubscribeUser{},
		"subscription rules":  &modelwarframe.MissionSubscribeUserCheckType{},
	} {
		var count int64
		if err := database.DB.Model(model).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("%s count = %d after unmatched cancellation, want 1", name, count)
		}
	}
}

// TestSubscribeInvalidParams 非法/边界参数应判为非法，而非吞成 0。
func TestSubscribeInvalidParams(t *testing.T) {
	setupStage10DB(t)
	cmd := &warframe.SubscribeCommand{BotUID: 1, GroupID: 1, UserID: 1}
	if warframe.ParseSubscribeCommand(cmd, "8-1-abc") {
		t.Fatalf("tier 非数字应解析失败: %+v", cmd)
	}
	if warframe.ParseSubscribeCommand(cmd, "-") {
		t.Fatal("空分隔符内容应解析失败")
	}
	if warframe.ParseSubscribeCommand(cmd, "") {
		t.Fatal("空串应解析失败")
	}
}

// TestParseSubscribeCommandStrict 验证订阅参数严格校验：
// 保留空段不压缩、按订阅类型拒绝多余段、裂隙等级只接受 1~5，任何不合法输入都拒绝。
func TestParseSubscribeCommandStrict(t *testing.T) {
	valid := []string{
		"1",     // 仲裁（无子参数）
		"1-2",   // 仲裁 + 任务类型
		"5",     // 入侵（无子参数）
		"5-1",   // 入侵 + 奖励类型
		"8",     // 裂隙（无子参数）
		"8-1",   // 裂隙 + 任务类型
		"8-1-4", // 裂隙 + 任务类型 + 等级
		"9",     // 无参数类型
	}
	for _, raw := range valid {
		cmd := &warframe.SubscribeCommand{}
		if !warframe.ParseSubscribeCommand(cmd, raw) {
			t.Fatalf("合法输入被拒绝: %q", raw)
		}
	}

	invalid := []struct{ raw, why string }{
		{"", "空串"},
		{"0", "订阅类型编号越界（0）"},
		{"14", "订阅类型编号越界（14）"},
		{"-8", "首段为空"},
		{"8-", "尾段为空"},
		{"8--4", "中间空段不得被压缩成 8-4"},
		{"8-1-4-7", "裂隙多余段"},
		{"8-1-0", "裂隙等级 0 越界"},
		{"8-1-6", "裂隙等级 6 越界"},
		{"8-abc", "任务类型非数字"},
		{"8-1-abc", "裂隙等级非数字"},
		{"9-2", "无参数类型不接受多余段"},
		{"1-2-3", "仲裁多余段"},
		{"5-1-2", "入侵多余段"},
	}
	for _, tc := range invalid {
		cmd := &warframe.SubscribeCommand{}
		if warframe.ParseSubscribeCommand(cmd, tc.raw) {
			t.Fatalf("非法输入被接受（%s）: %q -> %+v", tc.why, tc.raw, cmd)
		}
	}
}

func ptr[T any](v T) *T { return &v }
