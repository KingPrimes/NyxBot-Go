// Package database 数据持久化层，基于 GORM + pure-Go SQLite。
// 替代 Java 项目的 JPA/H2，启动时自动建表并初始化默认管理员。
package database

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"nyxbot-go/internal/logging"
	"nyxbot-go/internal/model/system"
)

const adminCredentialsFile = "admin-credentials.txt"

var executableDir = func() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Dir(exePath), nil
}

// ensureDefaultAdmin 检查是否存在系统用户，无则创建随机初始管理员并写入凭据文件。
func ensureDefaultAdmin() error {
	var count int64
	if err := DB.Model(&system.SysUser{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	username, err := randomLetters(6)
	if err != nil {
		return err
	}
	password, err := randomLetters(8)
	if err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user := system.SysUser{
		UserID:   1,
		UserName: username,
		Password: string(hash),
	}
	if err := DB.Create(&user).Error; err != nil {
		return err
	}

	if err := writeAdminCredentials(username, password); err != nil {
		return err
	}
	logging.InfoPack("database", "default admin user created, credentials saved to %s", adminCredentialsFile)
	return nil
}

// writeAdminCredentials 将初始管理员凭据写入可执行文件同级的 admin-credentials.txt。
// 若文件已存在则跳过（不覆盖）。
func writeAdminCredentials(username, password string) error {
	dir, err := executableDir()
	if err != nil {
		return err
	}
	path := filepath.Join(dir, adminCredentialsFile)
	if _, err := os.Stat(path); err == nil {
		return nil
	}

	content := strings.Join([]string{
		"NyxBot initial administrator credentials",
		"",
		fmt.Sprintf("username: %s", username),
		fmt.Sprintf("password: %s", password),
		"",
		"Please change the password after first login.",
	}, "\n") + "\n"

	return os.WriteFile(path, []byte(content), 0600)
}

// randomLetters 生成指定长度的随机字母串，用于初始密码生成。
func randomLetters(length int) (string, error) {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	result := make([]byte, length)
	for i := range result {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		if err != nil {
			return "", err
		}
		result[i] = letters[n.Int64()]
	}
	return string(result), nil
}
