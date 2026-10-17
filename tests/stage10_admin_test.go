// 阶段10 管理类指令测试：权限元数据、执行器接口契约、命令挂载存在性，
// 以及「更新分级」「更新仲裁」经 CommandRegistry 的全链路派发（命令匹配 → 超管校验 → 执行器）。
package tests

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	zero "github.com/wdvxdr1123/ZeroBot"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"nyxbot-go/internal/database"
	"nyxbot-go/internal/enum/nyxbot"
	modelbot "nyxbot-go/internal/model/bot"
	modelsystem "nyxbot-go/internal/model/system"
	"nyxbot-go/internal/onebot"
	"nyxbot-go/internal/warframe"
)

// TestDataImporterImplementsDataUpdateExecutor 编译期断言：DataImporter 满足管理更新接口。
func TestDataImporterImplementsDataUpdateExecutor(t *testing.T) {
	var _ onebot.DataUpdateExecutor = (*warframe.DataImporter)(nil)
	var executor onebot.DataUpdateExecutor = (*warframe.DataImporter)(nil)
	if executor == nil {
		t.Fatal("接口断言不应为 nil")
	}
}

// TestAdminCommandPermissions 管理类指令必须为 SUPER_ADMIN 且命令元数据存在。
func TestAdminCommandPermissions(t *testing.T) {
	adminCodes := []nyxbot.Codes{
		nyxbot.CmdUpdateWfResMarketItems,
		nyxbot.CmdUpdateWfResMarketRiven,
		nyxbot.CmdUpdateWfSister,
		nyxbot.CmdUpdateWfTar,
		nyxbot.CmdUpdateWfArbitration,
		nyxbot.CmdUpdateWfArbitrationTiers,
	}
	for _, code := range adminCodes {
		info, ok := nyxbot.CodesInfo[code]
		if !ok {
			t.Fatalf("%s 缺少命令元数据", code)
		}
		if info.Permissions != nyxbot.PermSuperAdmin {
			t.Fatalf("%s 权限应为超级管理员, 实际 %v", code, info.Permissions)
		}
	}
}

// fakeDataExecutor 记录管理更新指令派发到了哪个执行器方法；各方法立即成功返回。
type fakeDataExecutor struct {
	mu    sync.Mutex
	calls []string
}

// record 追加一次调用记录。
func (executor *fakeDataExecutor) record(name string) {
	executor.mu.Lock()
	executor.calls = append(executor.calls, name)
	executor.mu.Unlock()
}

// snapshot 返回已记录的调用名副本。
func (executor *fakeDataExecutor) snapshot() []string {
	executor.mu.Lock()
	defer executor.mu.Unlock()
	return append([]string(nil), executor.calls...)
}

func (executor *fakeDataExecutor) UpdateOrdersItems(context.Context) error {
	executor.record("orders-items")
	return nil
}

func (executor *fakeDataExecutor) UpdateRivenItems(context.Context) error {
	executor.record("riven-items")
	return nil
}

func (executor *fakeDataExecutor) UpdateLichSister(context.Context) error {
	executor.record("lich-sister")
	return nil
}

func (executor *fakeDataExecutor) UpdateTranslation(context.Context) error {
	executor.record("translation")
	return nil
}

func (executor *fakeDataExecutor) UpdateArbitration(context.Context) error {
	executor.record("arbitration")
	return nil
}

func (executor *fakeDataExecutor) UpdateArbitrationTiers(context.Context) error {
	executor.record("arbitration-tiers")
	return nil
}

// TestArbitrationUpdateCommandsEndToEnd 验证新增的两条超管指令全链路派发：
// 命令匹配 → 权限校验 → handler 立即回「已发布任务，正在更新！」→ 异步调用执行器对应方法。
func TestArbitrationUpdateCommandsEndToEnd(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "admin-e2e.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	// 指令派发会走权限与黑白名单校验，并写入 log_infos
	if err := db.AutoMigrate(
		&modelbot.BotAdmin{}, &modelbot.GroupWhite{}, &modelbot.ProveWhite{},
		&modelbot.GroupBlack{}, &modelbot.ProveBlack{},
		&modelsystem.LogInfo{},
	); err != nil {
		t.Fatal(err)
	}
	database.DB = db
	t.Cleanup(func() { closeGlobalDB(t) })

	const (
		botUID   = int64(14001)
		adminUID = int64(24001)
	)
	if err := database.DB.Create(&modelbot.BotAdmin{
		BotUID: botUID, AdminUID: adminUID, Permissions: string(nyxbot.PermSuperAdmin),
	}).Error; err != nil {
		t.Fatal(err)
	}

	caller := &recordingCaller{}
	zero.APICallers.Store(botUID, caller)
	previousConfig := zero.BotConfig
	zero.BotConfig = zero.Config{MaxProcessTime: 10 * time.Second}
	registry := onebot.NewCommandRegistry(false)
	if err := registry.Register(); err != nil {
		t.Fatal(err)
	}
	executor := newBlockingExecutor()
	registry.SetDataExecutor(executor)
	t.Cleanup(func() {
		executor.releaseAll() // 失败路径下放行被阻塞的执行 goroutine，避免泄漏
		registry.Close()
		zero.BotConfig = previousConfig
		zero.APICallers.Delete(botUID)
	})

	bot := zero.GetBot(botUID)
	if bot == nil {
		t.Fatal("test Bot 不可用")
	}

	cases := []struct {
		command string
		want    string
	}{
		{command: "更新仲裁", want: "arbitration"},
		{command: "更新分级", want: "arbitration-tiers"},
	}
	for _, tc := range cases {
		before := len(caller.snapshot())
		echoPrivateMessage(t, bot, botUID, adminUID, tc.command)

		// 执行器此刻被阻塞：确认回复必须先到。若 handler 改成同步执行（先跑完再回复），
		// 这里会一直等到超时，测试即失败——这正是本用例要锁住的「先回执、后异步执行」。
		waitFor(t, func() bool {
			for _, request := range caller.snapshot()[before:] {
				if strings.Contains(replyText(request.Params["message"]), "已发布任务") {
					return true
				}
			}
			return false
		}, "指令 "+tc.command+" 应在执行器完成前回复「已发布任务，正在更新！」")

		select {
		case name := <-executor.started:
			if name != tc.want {
				t.Fatalf("指令 %q 应派发到 %s, 实际 %s", tc.command, tc.want, name)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("指令 %q 未派发到执行器", tc.command)
		}

		// 放行后异步执行应正常收尾
		executor.unblock()
		want := tc.want
		waitFor(t, func() bool {
			return slices.Contains(executor.snapshot(), want)
		}, "放行后指令 "+tc.command+" 应执行完成")
	}
}

// blockingExecutor 在被调用时先通知测试、再阻塞，直到 unblock 放行才记录并返回；
// 用于验证管理指令「立即回执、后台异步执行」的时序（同步执行会卡住回执断言）。
type blockingExecutor struct {
	fakeDataExecutor
	started chan string   // 每次被调用发一个方法名（缓冲，不阻塞）
	release chan struct{} // 每次放行一个调用（缓冲，不阻塞）
}

// newBlockingExecutor 创建阻塞执行器。
func newBlockingExecutor() *blockingExecutor {
	return &blockingExecutor{
		started: make(chan string, 8),
		release: make(chan struct{}, 8),
	}
}

func (executor *blockingExecutor) UpdateArbitration(context.Context) error {
	return executor.block("arbitration")
}

func (executor *blockingExecutor) UpdateArbitrationTiers(context.Context) error {
	return executor.block("arbitration-tiers")
}

// block 通知测试已被调用后阻塞，放行后才记录并返回。
func (executor *blockingExecutor) block(name string) error {
	select {
	case executor.started <- name:
	default:
	}
	<-executor.release
	executor.record(name)
	return nil
}

// unblock 放行一次阻塞中的执行调用。
func (executor *blockingExecutor) unblock() {
	executor.release <- struct{}{}
}

// releaseAll 塞满放行令牌，确保失败路径下被阻塞的执行 goroutine 不再泄漏。
func (executor *blockingExecutor) releaseAll() {
	for range cap(executor.release) {
		select {
		case executor.release <- struct{}{}:
		default:
		}
	}
}

// TestArbitrationUpdateCommandsDenyNonAdmin 非超管发指令不得派发到执行器。
func TestArbitrationUpdateCommandsDenyNonAdmin(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "admin-deny.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&modelbot.BotAdmin{}, &modelbot.GroupWhite{}, &modelbot.ProveWhite{},
		&modelbot.GroupBlack{}, &modelbot.ProveBlack{},
		&modelsystem.LogInfo{},
	); err != nil {
		t.Fatal(err)
	}
	database.DB = db
	t.Cleanup(func() { closeGlobalDB(t) })

	const (
		botUID  = int64(14002)
		userUID = int64(24002)
	)
	// 该用户仅为普通管理员，不足 SUPER_ADMIN
	if err := database.DB.Create(&modelbot.BotAdmin{
		BotUID: botUID, AdminUID: userUID, Permissions: string(nyxbot.PermAdmin),
	}).Error; err != nil {
		t.Fatal(err)
	}

	caller := &recordingCaller{}
	zero.APICallers.Store(botUID, caller)
	previousConfig := zero.BotConfig
	zero.BotConfig = zero.Config{MaxProcessTime: 10 * time.Second}
	registry := onebot.NewCommandRegistry(false)
	if err := registry.Register(); err != nil {
		t.Fatal(err)
	}
	executor := &fakeDataExecutor{}
	registry.SetDataExecutor(executor)
	t.Cleanup(func() {
		registry.Close()
		zero.BotConfig = previousConfig
		zero.APICallers.Delete(botUID)
	})

	bot := zero.GetBot(botUID)
	if bot == nil {
		t.Fatal("test Bot 不可用")
	}

	echoPrivateMessage(t, bot, botUID, userUID, "更新分级")
	echoPrivateMessage(t, bot, botUID, userUID, "更新仲裁")
	// 权限拒绝发生在派发前，给异步链路留出充分的反向断言窗口
	time.Sleep(200 * time.Millisecond)
	if calls := executor.snapshot(); len(calls) != 0 {
		t.Fatalf("非超管不应触发执行器, got %v", calls)
	}
}
