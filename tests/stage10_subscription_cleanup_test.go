// 阶段10 订阅退群级联清理黑盒测试，对齐 Java WarframeTaskSubscribePlugin.onGroupDecrease。
// 覆盖：成员退群删其订阅（组保留）、最后一名成员退群连组删除、Bot 被踢删整组、无订阅群静默返回。
package tests

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	zero "github.com/wdvxdr1123/ZeroBot"

	"nyxbot-go/internal/database"
	"nyxbot-go/internal/enum/nyxbot"
	modelbot "nyxbot-go/internal/model/bot"
	modelsystem "nyxbot-go/internal/model/system"
	modelwarframe "nyxbot-go/internal/model/warframe"
	"nyxbot-go/internal/onebot"
	"nyxbot-go/internal/warframe"
)

// setupSubscriptionCleanupTest 准备订阅表并把清理延迟置零，测试结束后恢复默认延迟。
func setupSubscriptionCleanupTest(t *testing.T) {
	t.Helper()
	setupWorldStateBatchDB(t)
	warframe.SetSubscriptionCleanupDelay(0)
	t.Cleanup(func() { warframe.SetSubscriptionCleanupDelay(5 * time.Second) })
}

// newSubscriptionGroup 创建订阅组，返回其主键。
func newSubscriptionGroup(t *testing.T, groupID int64) uint {
	t.Helper()
	subscription := modelwarframe.MissionSubscribe{
		GroupName: "测试群", SubBotUID: 10001, SubGroup: groupID,
	}
	if err := database.DB.Create(&subscription).Error; err != nil {
		t.Fatal(err)
	}
	return subscription.ID
}

// addSubscriptionUser 为指定订阅组新增用户与其规则，返回用户主键。
func addSubscriptionUser(t *testing.T, subID uint, userID int64, subType nyxbot.SubscribeType) uint {
	t.Helper()
	user := modelwarframe.MissionSubscribeUser{
		SubID: subID, UserID: userID, UserName: "测试用户",
	}
	if err := database.DB.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	rule := modelwarframe.MissionSubscribeUserCheckType{
		SubuID: user.ID, Subscribe: string(subType),
	}
	if err := database.DB.Create(&rule).Error; err != nil {
		t.Fatal(err)
	}
	return user.ID
}

// countRows 统计指定模型的记录数（query 为空表示全表）。
func countRows(t *testing.T, model any, query string, args ...any) int64 {
	t.Helper()
	var count int64
	tx := database.DB.Model(model)
	if query != "" {
		tx = tx.Where(query, args...)
	}
	if err := tx.Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	return count
}

// TestSubscriptionCleanupOnMemberLeave 验证成员退群：只删该用户与其规则，订阅组与同群他人保留。
func TestSubscriptionCleanupOnMemberLeave(t *testing.T) {
	setupSubscriptionCleanupTest(t)
	const groupID, botUID = int64(90001), int64(10001)
	const leavingUser, stayingUser = int64(20001), int64(20002)

	subID := newSubscriptionGroup(t, groupID)
	leavingPK := addSubscriptionUser(t, subID, leavingUser, nyxbot.SubArbitration)
	addSubscriptionUser(t, subID, stayingUser, nyxbot.SubFissures)

	warframe.HandleGroupDecrease(groupID, leavingUser, botUID)

	waitFor(t, func() bool {
		return countRows(t, &modelwarframe.MissionSubscribeUser{}, "sub_id = ? AND user_id = ?", subID, leavingUser) == 0
	}, "退群用户应被清理")
	if count := countRows(t, &modelwarframe.MissionSubscribeUserCheckType{}, "subu_id = ?", leavingPK); count != 0 {
		t.Fatalf("退群用户的规则残留 %d 条", count)
	}
	// 同群其他用户与订阅组都应保留
	if count := countRows(t, &modelwarframe.MissionSubscribeUser{}, "sub_id = ? AND user_id = ?", subID, stayingUser); count != 1 {
		t.Fatalf("同群其他用户被误删，剩余 %d 条", count)
	}
	if count := countRows(t, &modelwarframe.MissionSubscribe{}, "id = ?", subID); count != 1 {
		t.Fatalf("仍有成员的订阅组被误删")
	}
}

// TestSubscriptionCleanupRemovesEmptyGroup 验证最后一名成员退群后订阅组连带删除。
func TestSubscriptionCleanupRemovesEmptyGroup(t *testing.T) {
	setupSubscriptionCleanupTest(t)
	const groupID, botUID, userID = int64(90002), int64(10001), int64(20003)

	subID := newSubscriptionGroup(t, groupID)
	userPK := addSubscriptionUser(t, subID, userID, nyxbot.SubArbitration)

	warframe.HandleGroupDecrease(groupID, userID, botUID)

	waitFor(t, func() bool {
		return countRows(t, &modelwarframe.MissionSubscribe{}, "id = ?", subID) == 0
	}, "无成员的订阅组应被删除")
	if count := countRows(t, &modelwarframe.MissionSubscribeUserCheckType{}, "subu_id = ?", userPK); count != 0 {
		t.Fatalf("用户规则残留 %d 条", count)
	}
}

// TestSubscriptionCleanupOnBotKicked 验证 Bot 被踢：整组用户与规则全部删除。
func TestSubscriptionCleanupOnBotKicked(t *testing.T) {
	setupSubscriptionCleanupTest(t)
	const groupID, botUID = int64(90003), int64(10001)
	const userA, userB = int64(20004), int64(20005)

	subID := newSubscriptionGroup(t, groupID)
	userAPK := addSubscriptionUser(t, subID, userA, nyxbot.SubArbitration)
	addSubscriptionUser(t, subID, userB, nyxbot.SubFissures)

	// Bot 自己被移出群：userID == botUID
	warframe.HandleGroupDecrease(groupID, botUID, botUID)

	waitFor(t, func() bool {
		return countRows(t, &modelwarframe.MissionSubscribe{}, "id = ?", subID) == 0
	}, "Bot 被踢后订阅组应被删除")
	if count := countRows(t, &modelwarframe.MissionSubscribeUser{}, "sub_id = ?", subID); count != 0 {
		t.Fatalf("订阅用户残留 %d 条", count)
	}
	if count := countRows(t, &modelwarframe.MissionSubscribeUserCheckType{}, "subu_id = ?", userAPK); count != 0 {
		t.Fatalf("用户规则残留 %d 条", count)
	}
}

// TestSubscriptionCleanupWithoutSubscription 验证该群无订阅时静默返回（不报错、不产生变更）。
func TestSubscriptionCleanupWithoutSubscription(t *testing.T) {
	setupSubscriptionCleanupTest(t)
	before := countRows(t, &modelwarframe.MissionSubscribe{}, "")

	warframe.HandleGroupDecrease(99999, 20006, 10001)

	// 给可能的异步任务一点时间；即使执行也不应有任何变更
	time.Sleep(50 * time.Millisecond)
	if after := countRows(t, &modelwarframe.MissionSubscribe{}, ""); after != before {
		t.Fatalf("无订阅群不应产生变更: %d -> %d", before, after)
	}
}

// TestSubscriptionCleanupUnknownMemberPreservesGroup verifies that a leave
// notice for a user with no subscription cannot remove another member's data.
func TestSubscriptionCleanupUnknownMemberPreservesGroup(t *testing.T) {
	setupSubscriptionCleanupTest(t)
	const groupID, botUID = int64(90007), int64(10001)
	const subscribedUser, unknownUser = int64(20010), int64(29999)

	subID := newSubscriptionGroup(t, groupID)
	userPK := addSubscriptionUser(t, subID, subscribedUser, nyxbot.SubArbitration)

	warframe.HandleGroupDecrease(groupID, unknownUser, botUID)
	time.Sleep(50 * time.Millisecond)

	if count := countRows(t, &modelwarframe.MissionSubscribe{}, "id = ?", subID); count != 1 {
		t.Fatalf("subscription group count = %d, want 1", count)
	}
	if count := countRows(t, &modelwarframe.MissionSubscribeUser{}, "id = ?", userPK); count != 1 {
		t.Fatalf("subscribed user count = %d, want 1", count)
	}
	if count := countRows(t, &modelwarframe.MissionSubscribeUserCheckType{}, "subu_id = ?", userPK); count != 1 {
		t.Fatalf("subscribed user's rule count = %d, want 1", count)
	}
}

// TestSubscriptionCleanupViaNoticeEvent 验证 group_decrease 通知经 CommandRegistry
// 的事件派发链路触发清理（含非本事件类型不触发的反向断言）。
func TestSubscriptionCleanupViaNoticeEvent(t *testing.T) {
	setupSubscriptionCleanupTest(t)
	// 通知事件派发需要 Bot 黑白名单表（权限过滤）与 ZeroBot 调用器
	if err := database.DB.AutoMigrate(
		&modelbot.BotAdmin{}, &modelbot.GroupWhite{}, &modelbot.ProveWhite{},
		&modelbot.GroupBlack{}, &modelbot.ProveBlack{}, &modelsystem.LogInfo{},
	); err != nil {
		t.Fatal(err)
	}

	const groupID, botUID = int64(90006), int64(10002)
	const leavingUser, stayingUser = int64(20008), int64(20009)
	subID := newSubscriptionGroup(t, groupID)
	leavingPK := addSubscriptionUser(t, subID, leavingUser, nyxbot.SubArbitration)
	addSubscriptionUser(t, subID, stayingUser, nyxbot.SubFissures)

	caller := &recordingCaller{}
	zero.APICallers.Store(botUID, caller)
	previousConfig := zero.BotConfig
	zero.BotConfig = zero.Config{MaxProcessTime: 2 * time.Second}
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
		t.Fatal("test Bot should be available")
	}

	// 1) 非 group_decrease 事件不应触发清理
	echoNoticeEvent(t, bot, botUID, map[string]any{
		"notice_type": "group_increase", "group_id": groupID, "user_id": leavingUser,
	})
	time.Sleep(100 * time.Millisecond)
	if count := countRows(t, &modelwarframe.MissionSubscribeUser{}, "sub_id = ? AND user_id = ?", subID, leavingUser); count != 1 {
		t.Fatalf("非 group_decrease 事件不应触发清理，剩余 %d 条", count)
	}

	// 2) group_decrease 事件应删掉退群用户的订阅
	echoNoticeEvent(t, bot, botUID, map[string]any{
		"notice_type": "group_decrease", "sub_type": "leave",
		"group_id": groupID, "user_id": leavingUser,
	})
	waitFor(t, func() bool {
		return countRows(t, &modelwarframe.MissionSubscribeUser{}, "sub_id = ? AND user_id = ?", subID, leavingUser) == 0
	}, "group_decrease 通知应触发退群清理")
	if count := countRows(t, &modelwarframe.MissionSubscribeUserCheckType{}, "subu_id = ?", leavingPK); count != 0 {
		t.Fatalf("规则残留 %d 条", count)
	}
	if count := countRows(t, &modelwarframe.MissionSubscribe{}, "id = ?", subID); count != 1 {
		t.Fatal("订阅组不应被删除（仍有其他成员）")
	}
}

// echoNoticeEvent 向 ZeroBot 注入一条通知事件。
func echoNoticeEvent(t *testing.T, bot *zero.Ctx, botUID int64, fields map[string]any) {
	t.Helper()
	payload := map[string]any{
		"post_type": "notice",
		"self_id":   botUID,
		"time":      time.Now().Unix(),
	}
	for key, value := range fields {
		payload[key] = value
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	bot.Echo(encoded)
}

// TestSubscriptionCleanupIgnoresOtherGroup 验证只清理事件所在群，其它群订阅不受影响。
func TestSubscriptionCleanupIgnoresOtherGroup(t *testing.T) {
	setupSubscriptionCleanupTest(t)
	const leaveGroup, otherGroup, botUID = int64(90004), int64(90005), int64(10001)
	const userID = int64(20007)

	leaveSubID := newSubscriptionGroup(t, leaveGroup)
	leaveUserPK := addSubscriptionUser(t, leaveSubID, userID, nyxbot.SubArbitration)

	otherSubID := newSubscriptionGroup(t, otherGroup)
	otherUserPK := addSubscriptionUser(t, otherSubID, userID, nyxbot.SubArbitration)

	warframe.HandleGroupDecrease(leaveGroup, userID, botUID)

	waitFor(t, func() bool {
		return countRows(t, &modelwarframe.MissionSubscribe{}, "id = ?", leaveSubID) == 0
	}, "退群所在群的订阅应被清理")
	if count := countRows(t, &modelwarframe.MissionSubscribeUserCheckType{}, "subu_id = ?", leaveUserPK); count != 0 {
		t.Fatalf("退群群的用户规则残留 %d 条", count)
	}
	// 另一个群的同名用户订阅必须完好
	if count := countRows(t, &modelwarframe.MissionSubscribe{}, "id = ?", otherSubID); count != 1 {
		t.Fatal("其它群订阅被误删")
	}
	if count := countRows(t, &modelwarframe.MissionSubscribeUserCheckType{}, "subu_id = ?", otherUserPK); count != 1 {
		t.Fatal("其它群用户规则被误删")
	}
}

// TestSubscriptionCleanupKeepsResubscribedRecords 验证延迟兜底清理不会删除抖动窗口期内重建的订阅。
//
// 回归背景：`findOrCreateSubscription`/`findOrCreateUser` 会复用已存在记录（按 `sub_group`、
// 按 `(sub_id,user_id)`）。若旧记录在窗口期内仍留在库中，窗口期内重建的订阅会复用同一主键，
// 延迟任务按 `sub_id`/`user_id` 反查时会把新记录一并删除（数据丢失）。
// 正确顺序：事件到达时先快照旧用户主键并**立即**删除旧记录，延迟阶段只按快照主键兜底
// （三张表主键均为 AUTOINCREMENT，主键不复用，故快照不会命中新记录）。
func TestSubscriptionCleanupKeepsResubscribedRecords(t *testing.T) {
	setupSubscriptionCleanupTest(t)
	const delay = 300 * time.Millisecond
	warframe.SetSubscriptionCleanupDelay(delay)

	const groupID, botUID, userID = int64(91001), int64(10001), int64(21001)
	oldSubID := newSubscriptionGroup(t, groupID)
	oldUserPK := addSubscriptionUser(t, oldSubID, userID, nyxbot.SubArbitration)

	// 成员退群：旧订阅组/旧用户/旧规则都必须在进入延迟窗口之前就被清掉
	warframe.HandleGroupDecrease(groupID, userID, botUID)
	if count := countRows(t, &modelwarframe.MissionSubscribe{}, "id = ?", oldSubID); count != 0 {
		t.Fatal("旧订阅组应在延迟前立即删除")
	}
	if count := countRows(t, &modelwarframe.MissionSubscribeUser{}, "id = ?", oldUserPK); count != 0 {
		t.Fatal("旧订阅用户应在延迟前立即删除")
	}
	if count := countRows(t, &modelwarframe.MissionSubscribeUserCheckType{}, "subu_id = ?", oldUserPK); count != 0 {
		t.Fatal("旧用户规则应在延迟前立即删除")
	}

	// 抖动窗口内重新订阅：旧记录已删除，只能新建（新主键）
	cmd := &warframe.SubscribeCommand{BotUID: botUID, GroupID: groupID, GroupName: "测试群", UserID: userID, UserName: "测试用户"}
	if !warframe.ParseSubscribeCommand(cmd, "1") {
		t.Fatal("解析订阅参数 1 失败")
	}
	if got := warframe.Subscribe(cmd); !strings.Contains(got, "订阅成功") {
		t.Fatalf("窗口期内重新订阅应成功: %q", got)
	}

	// 等待延迟兜底任务执行完毕（此处断言的是「延迟任务什么都没删」，只能等过延迟）
	time.Sleep(delay + 700*time.Millisecond)

	var newSub modelwarframe.MissionSubscribe
	if err := database.DB.Where("sub_group = ?", groupID).First(&newSub).Error; err != nil {
		t.Fatalf("重新订阅的订阅组被延迟任务删除: %v", err)
	}
	if newSub.ID == oldSubID {
		t.Fatalf("新订阅组复用了旧主键 %d（AUTOINCREMENT 不应复用）", oldSubID)
	}
	var newUser modelwarframe.MissionSubscribeUser
	if err := database.DB.Where("sub_id = ?", newSub.ID).First(&newUser).Error; err != nil {
		t.Fatalf("重新订阅的用户记录被延迟任务删除: %v", err)
	}
	if newUser.ID == oldUserPK {
		t.Fatalf("新用户记录复用了旧主键 %d", oldUserPK)
	}
	if count := countRows(t, &modelwarframe.MissionSubscribeUserCheckType{}, "subu_id = ?", newUser.ID); count != 1 {
		t.Fatalf("重新订阅的规则被延迟任务删除，剩余 %d 条", count)
	}
}
