package database

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"nyxbot-go/internal/model/system"
)

func TestWriteAdminCredentialsDoesNotOverwriteExistingFile(t *testing.T) {
	tmp := t.TempDir()
	old, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })

	path := filepath.Join(tmp, adminCredentialsFile)
	oldExecutableDir := executableDir
	executableDir = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { executableDir = oldExecutableDir })
	if err := os.WriteFile(path, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := writeAdminCredentials("abc", "123"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "keep" {
		t.Fatalf("expected existing credentials file to remain unchanged, got %q", string(data))
	}
}

func TestEnsureDefaultAdminCreatesCredentials(t *testing.T) {
	tmp := t.TempDir()
	old, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })

	db, err := gorm.Open(sqlite.Open(filepath.Join(tmp, "test.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	DB = db
	t.Cleanup(func() {
		sqlDB, err := DB.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	oldExecutableDir := executableDir
	executableDir = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { executableDir = oldExecutableDir })
	if err := DB.AutoMigrate(&system.SysUser{}); err != nil {
		t.Fatal(err)
	}
	if err := ensureDefaultAdmin(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(tmp, adminCredentialsFile)); err != nil {
		t.Fatalf("expected credentials file: %v", err)
	}
}
