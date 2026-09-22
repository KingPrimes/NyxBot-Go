// 订阅数据的退群级联清理，对齐 Java WarframeTaskSubscribePlugin.onGroupDecrease：
//   - Bot 被移出群：删除该群订阅组及其全部用户与规则
//   - 普通成员退群：只删除该用户及其规则，用户清空后连带删除订阅组
//
// 事件处理器在收到 group_decrease 通知时，先把当时的订阅组 id / 用户 id 入队，
// 由后台 goroutine 稍后执行删除，避免 OneBot 侧等待数据库（对齐 Java 的异步事件处理习惯）。
//
// 之所以在入队时固定 id 而不是执行时按群号重查：Bot 被踢后若立刻被重新拉回并产生新订阅，
// 按群号重查会误删新数据；固定 id 只会删掉本次事件对应的旧记录。
package warframe

import (
	"errors"
	"sync"
	"time"

	"gorm.io/gorm"

	"nyxbot-go/internal/database"
	"nyxbot-go/internal/logging"
	modelwarframe "nyxbot-go/internal/model/warframe"
)

// subscriptionCleanupDelay 退群清理延迟：等待「Bot 被踢后立刻被拉回」的抖动窗口过去。
// 声明为变量以便黑盒测试将其置零，避免测试等待真实延迟。
var subscriptionCleanupDelay = 5 * time.Second

// subscriptionCleanupJob 一次待执行的退群清理（botKicked=true 表示删整个订阅组）。
type subscriptionCleanupJob struct {
	subscriptionID uint
	userID         int64
	groupID        int64
	botKicked      bool
}

var (
	subscriptionCleanupOnce sync.Once
	subscriptionCleanupCh   = make(chan subscriptionCleanupJob, 64)
)

// HandleGroupDecrease 处理群成员减少事件（退群/被踢），对齐 Java onGroupDecrease。
// groupID 为事件所在群号，userID 为离开的成员，botUID 为当前 Bot 自身 QQ 号。
// 数据库不可用或该群没有订阅时静默返回（不视为错误）。
func HandleGroupDecrease(groupID, userID, botUID int64) {
	if database.DB == nil || groupID <= 0 {
		return
	}
	var subscription modelwarframe.MissionSubscribe
	err := database.DB.Where("sub_group = ?", groupID).First(&subscription).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logging.WarnPack("warframe.subscribe", "query subscription for group %d failed: %v", groupID, err)
		}
		return
	}

	job := subscriptionCleanupJob{
		subscriptionID: subscription.ID,
		userID:         userID,
		groupID:        groupID,
		botKicked:      botUID > 0 && userID == botUID,
	}
	startSubscriptionCleanupWorker()
	select {
	case subscriptionCleanupCh <- job:
	default:
		// 队列已满（异常高频退群）：同步兜底执行，避免遗漏清理
		applySubscriptionCleanup(job)
	}
}

// startSubscriptionCleanupWorker 懒启动清理 worker（仅启动一次）。
func startSubscriptionCleanupWorker() {
	subscriptionCleanupOnce.Do(func() {
		go runSubscriptionCleanupWorker(subscriptionCleanupCh)
	})
}

// runSubscriptionCleanupWorker 消费清理任务并延迟执行；channel 关闭时退出。
// 每条任务执行前**重新读取**延迟值，便于测试在运行期调整。
func runSubscriptionCleanupWorker(jobs <-chan subscriptionCleanupJob) {
	for job := range jobs {
		// 逐条延迟执行，保证同一 worker 内的处理顺序（先到的事件先清）
		time.Sleep(subscriptionCleanupDelay)
		applySubscriptionCleanup(job)
	}
}

// SetSubscriptionCleanupDelay 设置退群清理延迟（供测试注入；传 0 表示立即执行）。
func SetSubscriptionCleanupDelay(delay time.Duration) {
	if delay < 0 {
		delay = 0
	}
	subscriptionCleanupDelay = delay
}

// applySubscriptionCleanup 执行一次清理（Bot 被踢删整组，否则只删该用户）。
func applySubscriptionCleanup(job subscriptionCleanupJob) {
	if database.DB == nil {
		return
	}
	if job.botKicked {
		deleteSubscriptionGroup(job.subscriptionID)
		logging.InfoPack("warframe.subscribe", "Bot 被移出群 %d，已清除该群全部订阅数据", job.groupID)
		return
	}
	deleteSubscriptionUser(job.subscriptionID, job.userID)
	logging.InfoPack("warframe.subscribe", "用户 %d 退出群 %d，已清除其订阅数据", job.userID, job.groupID)
}

// deleteSubscriptionGroup 删除订阅组及其全部用户与规则。
// 表间无数据库级联约束，故按 组 → 用户 → 规则 自下而上逐级删除。
func deleteSubscriptionGroup(subscriptionID uint) {
	if database.DB == nil || subscriptionID == 0 {
		return
	}
	var users []modelwarframe.MissionSubscribeUser
	if err := database.DB.Where("sub_id = ?", subscriptionID).Find(&users).Error; err != nil {
		logging.ErrorPack("warframe.subscribe", "query users of subscription %d failed: %v", subscriptionID, err)
		return
	}
	for i := range users {
		deleteRulesOfUser(users[i].ID)
	}
	if err := database.DB.Where("sub_id = ?", subscriptionID).Delete(&modelwarframe.MissionSubscribeUser{}).Error; err != nil {
		logging.ErrorPack("warframe.subscribe", "delete users of subscription %d failed: %v", subscriptionID, err)
		return
	}
	if err := database.DB.Delete(&modelwarframe.MissionSubscribe{}, subscriptionID).Error; err != nil {
		logging.ErrorPack("warframe.subscribe", "delete subscription %d failed: %v", subscriptionID, err)
	}
}

// deleteSubscriptionUser 删除指定用户及其规则；用户清空后连带删除订阅组（对齐 Java removeIf + cleanup）。
func deleteSubscriptionUser(subscriptionID uint, userID int64) {
	if database.DB == nil || subscriptionID == 0 || userID == 0 {
		return
	}
	var user modelwarframe.MissionSubscribeUser
	err := database.DB.Where("sub_id = ? AND user_id = ?", subscriptionID, userID).First(&user).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logging.ErrorPack("warframe.subscribe", "query user %d in subscription %d failed: %v", userID, subscriptionID, err)
		}
		return
	}
	deleteRulesOfUser(user.ID)
	if err := database.DB.Delete(&modelwarframe.MissionSubscribeUser{}, user.ID).Error; err != nil {
		logging.ErrorPack("warframe.subscribe", "delete user %d failed: %v", userID, err)
		return
	}
	removeSubscriptionIfEmpty(subscriptionID)
}

// deleteRulesOfUser 删除某用户下的全部订阅规则。
func deleteRulesOfUser(subuID uint) {
	if database.DB == nil || subuID == 0 {
		return
	}
	if err := database.DB.Where("subu_id = ?", subuID).Delete(&modelwarframe.MissionSubscribeUserCheckType{}).Error; err != nil {
		logging.ErrorPack("warframe.subscribe", "delete rules of user %d failed: %v", subuID, err)
	}
}

// removeSubscriptionIfEmpty 订阅组已无用户时删除该组。
func removeSubscriptionIfEmpty(subscriptionID uint) {
	if database.DB == nil || subscriptionID == 0 {
		return
	}
	var remaining int64
	if err := database.DB.Model(&modelwarframe.MissionSubscribeUser{}).
		Where("sub_id = ?", subscriptionID).Count(&remaining).Error; err != nil {
		logging.ErrorPack("warframe.subscribe", "count users of subscription %d failed: %v", subscriptionID, err)
		return
	}
	if remaining > 0 {
		return
	}
	if err := database.DB.Delete(&modelwarframe.MissionSubscribe{}, subscriptionID).Error; err != nil {
		logging.ErrorPack("warframe.subscribe", "delete empty subscription %d failed: %v", subscriptionID, err)
	}
}
