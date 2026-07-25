package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nyxbot-go/internal/database"
)

// adminCredentialsFileName 首启生成的管理员凭据文件名，固定写在可执行文件同级目录。
const adminCredentialsFileName = "admin-credentials.txt"

// closeGlobalDB 关闭 database 包的全局连接并置空，避免 Windows 下临时目录被句柄锁定无法清理。
func closeGlobalDB(t *testing.T) {
	t.Helper()
	if database.DB == nil {
		return
	}
	sqlDB, err := database.DB.DB()
	if err == nil {
		_ = sqlDB.Close()
	}
	database.DB = nil
}

// TestInitCreatesAdminCredentialsOnce 验证数据库初始化时的管理员引导行为：
// 无系统用户时创建随机管理员并把凭据写入可执行文件同级的 admin-credentials.txt，
// 再次初始化不会覆盖已存在的凭据文件。
func TestInitCreatesAdminCredentialsOnce(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	// 测试运行时可执行文件位于 go-build 临时目录，凭据文件随之自动清理
	credPath := filepath.Join(filepath.Dir(exe), adminCredentialsFileName)
	_ = os.Remove(credPath) // 清除同进程内前序测试可能留下的文件，保证独立性
	t.Cleanup(func() { _ = os.Remove(credPath) })

	database.Init(filepath.Join(t.TempDir(), "test.db"), false)
	closeGlobalDB(t) // 立即关闭连接：第二次 Init 会覆盖全局 DB，拖延关闭会导致句柄泄漏

	data, err := os.ReadFile(credPath)
	if err != nil {
		t.Fatalf("首次初始化未生成凭据文件: %v", err)
	}
	if !strings.Contains(string(data), "username:") || !strings.Contains(string(data), "password:") {
		t.Errorf("凭据文件内容不完整:\n%s", string(data))
	}

	// 篡改凭据文件后再次初始化，文件内容应保持不变
	if err := os.WriteFile(credPath, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	database.Init(filepath.Join(t.TempDir(), "test2.db"), false)
	closeGlobalDB(t)

	data, err = os.ReadFile(credPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "keep" {
		t.Error("再次初始化覆盖了已存在的凭据文件")
	}
}
