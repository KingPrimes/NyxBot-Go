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
// 只有库里一个用户都没有时才会走到写文件这一步，因此这里的写入必须覆盖旧文件：
// data 目录的数据库被删除（或换成空库）后会重新生成管理员，若沿用旧的 admin-credentials.txt，
// 文件里就是一组登录不了的账号密码。
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

	// 先写凭据文件再落库：写文件失败时库仍是空的（count 保持为 0），
	// 下次启动会重新生成并再次覆盖凭据文件；反过来先落库再写文件一旦失败，
	// 就会留下「库里已有管理员、但明文密码无人知晓」的死局。
	if err := writeAdminCredentials(username, password); err != nil {
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

	logging.InfoPack("database", "default admin user created, credentials saved to %s", adminCredentialsFile)
	return nil
}

// writeAdminCredentials 把初始管理员凭据覆盖写入可执行文件同级的 admin-credentials.txt。
// 该文件是明文初始密码的唯一存放处（库里只有 bcrypt 哈希），所以新建管理员时必须覆盖旧内容，
// 否则数据库被删除后重新生成的管理员无法按文件里的账号登录。
// 先写 ".tmp" 再原子改名，避免写入中断留下半截凭据；权限 0600（仅属主可读）。
func writeAdminCredentials(username, password string) error {
	dir, err := executableDir()
	if err != nil {
		return err
	}
	path := filepath.Join(dir, adminCredentialsFile)

	content := strings.Join([]string{
		"NyxBot initial administrator credentials",
		"",
		fmt.Sprintf("username: %s", username),
		fmt.Sprintf("password: %s", password),
		"",
		"Please change the password after first login.",
	}, "\n") + "\n"

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(content), 0600); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
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
