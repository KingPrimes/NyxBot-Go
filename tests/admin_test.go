package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"nyxbot-go/internal/database"
	modelsystem "nyxbot-go/internal/model/system"
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

// resetAdminCredentialsFile 删除并登记清理凭据文件，避免同进程内前序用例互相影响。
// 测试运行时可执行文件位于 go-build 临时目录，凭据文件随之自动清理。
func resetAdminCredentialsFile(t *testing.T) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(filepath.Dir(exe), adminCredentialsFileName)
	_ = os.Remove(path)
	t.Cleanup(func() { _ = os.Remove(path) })
	return path
}

// readAdminCredentials 解析凭据文件中的用户名与密码。
func readAdminCredentials(t *testing.T, path string) (string, string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取凭据文件失败: %v", err)
	}
	var username, password string
	for _, line := range strings.Split(string(raw), "\n") {
		if value, ok := strings.CutPrefix(line, "username: "); ok {
			username = strings.TrimSpace(value)
		}
		if value, ok := strings.CutPrefix(line, "password: "); ok {
			password = strings.TrimSpace(value)
		}
	}
	if username == "" || password == "" {
		t.Fatalf("凭据文件内容不完整:\n%s", string(raw))
	}
	return username, password
}

// removeDatabaseFiles 删除 SQLite 主库文件及其可能的附属文件，模拟 data 目录被清空。
func removeDatabaseFiles(t *testing.T, dbPath string) {
	t.Helper()
	for _, path := range []string{dbPath, dbPath + "-wal", dbPath + "-shm", dbPath + "-journal"} {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			t.Fatalf("删除数据库文件 %s 失败: %v", path, err)
		}
	}
}

// assertAdminCredentialsUsable 断言凭据文件里的账号密码能通过库中 bcrypt 哈希校验（即真的能登录）。
func assertAdminCredentialsUsable(t *testing.T, username, password string) {
	t.Helper()
	var user modelsystem.SysUser
	if err := database.DB.Where("user_name = ?", username).First(&user).Error; err != nil {
		t.Fatalf("凭据文件中的用户名 %q 不在数据库里: %v", username, err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		t.Fatalf("凭据文件中的密码与数据库哈希不匹配: %v", err)
	}
}

// TestInitWritesAdminCredentialsOnFreshDatabase 验证空库初始化：创建随机管理员，
// 凭据文件里的账号密码与库中哈希一致（可用于登录）。
func TestInitWritesAdminCredentialsOnFreshDatabase(t *testing.T) {
	credPath := resetAdminCredentialsFile(t)

	database.Init(filepath.Join(t.TempDir(), "fresh.db"), false, "silent", 0)
	defer closeGlobalDB(t)

	username, password := readAdminCredentials(t, credPath)
	assertAdminCredentialsUsable(t, username, password)
}

// TestInitKeepsAdminCredentialsWhenUsersExist 验证库里已有系统用户时（普通重启）不覆盖凭据文件。
func TestInitKeepsAdminCredentialsWhenUsersExist(t *testing.T) {
	credPath := resetAdminCredentialsFile(t)
	dbPath := filepath.Join(t.TempDir(), "existing.db")

	database.Init(dbPath, false, "silent", 0)
	closeGlobalDB(t) // 关闭后才能用同一路径再次打开

	if err := os.WriteFile(credPath, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	database.Init(dbPath, false, "silent", 0)
	defer closeGlobalDB(t)

	data, err := os.ReadFile(credPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "keep" {
		t.Errorf("系统中已有用户时不应覆盖凭据文件, 实际内容:\n%s", string(data))
	}
}

// TestInitRewritesAdminCredentialsAfterDatabaseRemoved 验证 data 目录中的数据库被删除后，
// 重新生成的管理员会覆盖旧的 admin-credentials.txt：
// 否则文件里留下的是上一套已失效的账号密码，无法登录。
func TestInitRewritesAdminCredentialsAfterDatabaseRemoved(t *testing.T) {
	credPath := resetAdminCredentialsFile(t)
	dbPath := filepath.Join(t.TempDir(), "removed.db")

	database.Init(dbPath, false, "silent", 0)
	closeGlobalDB(t)
	staleUsername, stalePassword := readAdminCredentials(t, credPath)

	// 数据库被移除，凭据文件仍在（复现 bug 场景）
	removeDatabaseFiles(t, dbPath)
	database.Init(dbPath, false, "silent", 0)
	defer closeGlobalDB(t)

	username, password := readAdminCredentials(t, credPath)
	assertAdminCredentialsUsable(t, username, password)
	if username == staleUsername && password == stalePassword {
		t.Fatalf("库被删除后凭据文件仍是旧的账号密码: %s / %s", username, password)
	}
}

// TestInitRollsBackAdminWhenCredentialsWriteFails 验证凭据文件写入失败时会回滚刚创建的管理员：
// 库回到空（count=0），下次启动可重新生成并覆盖凭据文件；否则会留下
// 「库里已有管理员、但明文密码无人知晓」的死局（Init 会 panic 退出，属预期）。
func TestInitRollsBackAdminWhenCredentialsWriteFails(t *testing.T) {
	// 用一个普通文件占住目录位置：凭据路径落在它下面时 os.CreateTemp 必然失败
	blocker := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocker, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	previous := database.SetAdminCredentialsPath(filepath.Join(blocker, adminCredentialsFileName))
	t.Cleanup(func() { database.SetAdminCredentialsPath(previous) })

	dbPath := filepath.Join(t.TempDir(), "rollback.db")
	func() {
		defer func() {
			if recover() == nil {
				t.Error("凭据文件写入失败时 Init 应当报错退出")
			}
		}()
		database.Init(dbPath, false, "silent", 0)
	}()
	defer closeGlobalDB(t)

	if database.DB == nil {
		t.Fatal("Init 失败后全局 DB 应仍然可用（用于校验回滚结果）")
	}
	var count int64
	if err := database.DB.Model(&modelsystem.SysUser{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("凭据写入失败后应回滚管理员，实际仍有 %d 个用户", count)
	}
}
