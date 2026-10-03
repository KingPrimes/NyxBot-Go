// 订阅数据的退群级联清理，对齐 Java WarframeTaskSubscribePlugin.onGroupDecrease：
//   - Bot 被移出群：删除该群订阅组及其全部用户与规则
//   - 普通成员退群：只删除该用户及其规则，用户清空后连带删除订阅组
//
// 处理顺序：事件到达时先对旧用户记录主键做快照，并在进入延迟队列**之前**立即
// 在事务中删除旧订阅组/旧用户/旧规则；随后才把快照交给后台 goroutine 做延迟兜底。
//
// 之所以要「先删旧记录 + 只按快照删除」而不是「稍后按 sub_id/user_id 重查后删」：
// findOrCreateSubscription/findOrCreateUser 会复用已存在的记录（按 sub_group、
// 按 (sub_id,user_id)），若旧记录在抖动窗口期内仍在库中，窗口期内重建的订阅就会
// 复用同一主键，延迟任务反查时会把新数据一并删掉。旧记录先删掉后，新订阅只能新建
// 记录（三张表主键均为 AUTOINCREMENT，主键不复用），延迟任务只按快照主键删除，不会波及新记录。
package warframe

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"gorm.io/gorm"

	"nyxbot-go/internal/database"
	"nyxbot-go/internal/logging"
	modelwarframe "nyxbot-go/internal/model/warframe"
)

// defaultSubscriptionCleanupDelay 默认退群清理延迟：留给「Bot 被踢后立刻被拉回」的抖动窗口。
// 真正的旧记录删除在 HandleGroupDecrease 中**立即**完成（进入延迟之前），
// 这里的延迟仅用于对旧记录快照再做一次幂等兜底清理。
const defaultSubscriptionCleanupDelay = 5 * time.Second

// subscriptionCleanupDelayNS 当前退群清理延迟（纳秒）。
// 用原子变量而非普通变量：worker goroutine 会读取它，测试（SetSubscriptionCleanupDelay）
// 会在运行期写入，普通变量构成数据竞争。
var subscriptionCleanupDelayNS atomic.Int64

func init() { subscriptionCleanupDelayNS.Store(int64(defaultSubscriptionCleanupDelay)) }

// subscriptionCleanupJob 一次待执行的退群清理。
//
// db 为事件发生时捕获的数据库句柄：延迟阶段不再读全局 database.DB，既避免与「测试运行期
// 替换全局句柄」构成数据竞争，也保证清理始终作用于事件发生时的那个库。
//
// userPKs 是事件发生时对旧用户记录主键做的快照：延迟阶段只按这些主键删除规则，
// 不按 sub_id/user_id 反查，因此窗口期内由 findOrCreateSubscription/findOrCreateUser
// 新建的记录不会被延迟任务误删（三张表主键均为 AUTOINCREMENT，主键不复用）。
type subscriptionCleanupJob struct {
	db             *gorm.DB
	subscriptionID uint
	userPKs        []uint
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
//
// 关键顺序：先对旧用户主键做快照，再在进入延迟队列之前**立即**删除旧订阅组/旧用户/旧规则，
// 最后才把快照交给延迟队列兜底。这样即使 Bot 在窗口期内被拉回、用户重新订阅，
// findOrCreateSubscription/findOrCreateUser 也只会新建记录（新主键），
// 延迟任务按旧快照删除，不会波及新记录。
func HandleGroupDecrease(groupID, userID, botUID int64) {
	db := database.DB // 只在此处读一次全局句柄，并随 job 传给延迟阶段
	if db == nil || groupID <= 0 {
		return
	}
	var subscription modelwarframe.MissionSubscribe
	err := db.Where("sub_group = ?", groupID).First(&subscription).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logging.WarnPack("warframe.subscribe", "query subscription for group %d failed: %v", groupID, err)
		}
		return
	}

	job := subscriptionCleanupJob{
		db:             db,
		subscriptionID: subscription.ID,
		userID:         userID,
		groupID:        groupID,
		botKicked:      botUID > 0 && userID == botUID,
	}
	job.userPKs = snapshotCleanupUsers(job)

	// 延迟前立即隔离/删除旧记录（事务内 规则 → 用户 → 订阅组）
	if err := applySubscriptionCleanup(job); err != nil {
		logging.ErrorPack("warframe.subscribe", "immediate cleanup for group %d failed: %v", groupID, err)
		return
	}
	if job.botKicked {
		logging.InfoPack("warframe.subscribe", "Bot 被移出群 %d，已清除该群全部订阅数据", groupID)
	} else {
		logging.InfoPack("warframe.subscribe", "用户 %d 退出群 %d，已清除其订阅数据", userID, groupID)
	}

	// 延迟阶段只对上面的旧记录快照做幂等兜底
	startSubscriptionCleanupWorker()
	select {
	case subscriptionCleanupCh <- job:
	default:
		// 队列已满（异常高频退群）：同步兜底执行，避免遗漏清理
		if err := applySubscriptionCleanup(job); err != nil {
			logging.ErrorPack("warframe.subscribe", "fallback cleanup for group %d failed: %v", groupID, err)
		}
	}
}

// startSubscriptionCleanupWorker 懒启动清理 worker（仅启动一次）。
func startSubscriptionCleanupWorker() {
	subscriptionCleanupOnce.Do(func() {
		go runSubscriptionCleanupWorker(subscriptionCleanupCh)
	})
}

// runSubscriptionCleanupWorker 消费清理任务并延迟重放快照清理（幂等）；channel 关闭时退出。
// 每条任务执行前**重新读取**延迟值，便于测试在运行期调整（原子读，见 subscriptionCleanupDelayNS）。
func runSubscriptionCleanupWorker(jobs <-chan subscriptionCleanupJob) {
	for job := range jobs {
		// 逐条延迟执行，保证同一 worker 内的处理顺序（先到的事件先清）
		time.Sleep(time.Duration(subscriptionCleanupDelayNS.Load()))
		if err := applySubscriptionCleanup(job); err != nil {
			logging.ErrorPack("warframe.subscribe", "delayed cleanup for group %d failed: %v", job.groupID, err)
		}
	}
}

// SetSubscriptionCleanupDelay 设置退群清理延迟（供测试注入；传 0 表示立即执行）。
func SetSubscriptionCleanupDelay(delay time.Duration) {
	if delay < 0 {
		delay = 0
	}
	subscriptionCleanupDelayNS.Store(int64(delay))
}

// snapshotCleanupUsers 快照本次事件要清理的旧用户记录主键：
// Bot 被踢时为该订阅组全部用户，普通成员退群时仅该成员。使用 job.db（事件时的句柄）。
func snapshotCleanupUsers(job subscriptionCleanupJob) []uint {
	if job.db == nil {
		return nil
	}
	query := job.db.Model(&modelwarframe.MissionSubscribeUser{}).Where("sub_id = ?", job.subscriptionID)
	if !job.botKicked {
		query = query.Where("user_id = ?", job.userID)
	}
	var users []modelwarframe.MissionSubscribeUser
	if err := query.Find(&users).Error; err != nil {
		logging.ErrorPack("warframe.subscribe", "snapshot users of subscription %d failed: %v", job.subscriptionID, err)
		return nil
	}
	userPKs := make([]uint, 0, len(users))
	for i := range users {
		userPKs = append(userPKs, users[i].ID)
	}
	return userPKs
}

// applySubscriptionCleanup 按快照幂等清理旧记录（延迟前后可重复执行）。
// 事务内按 规则 → 用户 → 订阅组 顺序删除：任一步失败（含 deleteRulesOfUser）即回滚并返回错误，
// 只有全部成功才提交。Bot 被踢时删整个订阅组，普通成员退群时仅在该组已无用户时删组。
// 全程使用 job.db（事件发生时的句柄），不再读全局 database.DB。
func applySubscriptionCleanup(job subscriptionCleanupJob) error {
	if job.db == nil || job.subscriptionID == 0 {
		return nil
	}
	return job.db.Transaction(func(tx *gorm.DB) error {
		if err := deleteRulesOfUser(tx, job.userPKs...); err != nil {
			return fmt.Errorf("delete rules of subscription %d: %w", job.subscriptionID, err)
		}
		if err := deleteSubscriptionUsers(tx, job.userPKs); err != nil {
			return fmt.Errorf("delete users of subscription %d: %w", job.subscriptionID, err)
		}
		if job.botKicked {
			if err := deleteSubscriptionGroup(tx, job.subscriptionID); err != nil {
				return fmt.Errorf("delete subscription %d: %w", job.subscriptionID, err)
			}
			return nil
		}
		return removeSubscriptionIfEmpty(tx, job.subscriptionID)
	})
}

// deleteSubscriptionUsers 按主键快照删除订阅用户记录。
func deleteSubscriptionUsers(tx *gorm.DB, userPKs []uint) error {
	if len(userPKs) == 0 {
		return nil
	}
	return tx.Delete(&modelwarframe.MissionSubscribeUser{}, userPKs).Error
}

// deleteSubscriptionGroup 删除订阅组记录（其用户与规则已按快照删除）。
func deleteSubscriptionGroup(tx *gorm.DB, subscriptionID uint) error {
	if subscriptionID == 0 {
		return nil
	}
	return tx.Delete(&modelwarframe.MissionSubscribe{}, subscriptionID).Error
}

// deleteRulesOfUser 删除指定用户记录下的全部订阅规则，返回删除错误。
func deleteRulesOfUser(tx *gorm.DB, subuIDs ...uint) error {
	if len(subuIDs) == 0 {
		return nil
	}
	return tx.Where("subu_id IN ?", subuIDs).Delete(&modelwarframe.MissionSubscribeUserCheckType{}).Error
}

// removeSubscriptionIfEmpty 订阅组已无用户时删除该组。
// 统计必须与删除处于同一事务，否则读不到尚未提交的删除结果。
func removeSubscriptionIfEmpty(tx *gorm.DB, subscriptionID uint) error {
	if subscriptionID == 0 {
		return nil
	}
	var remaining int64
	if err := tx.Model(&modelwarframe.MissionSubscribeUser{}).
		Where("sub_id = ?", subscriptionID).Count(&remaining).Error; err != nil {
		return fmt.Errorf("count users of subscription %d: %w", subscriptionID, err)
	}
	if remaining > 0 {
		return nil
	}
	if err := deleteSubscriptionGroup(tx, subscriptionID); err != nil {
		return fmt.Errorf("delete empty subscription %d: %w", subscriptionID, err)
	}
	return nil
}
