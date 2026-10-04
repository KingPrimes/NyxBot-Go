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

// adminCredentialsPathOverride 非空时替代「可执行文件同级」的默认凭据路径（供黑盒测试注入）。
var adminCredentialsPathOverride string

// SetAdminCredentialsPath 覆盖 admin-credentials.txt 的路径，空串恢复默认（可执行文件同级），
// 返回原值（供黑盒测试验证写入失败等分支）。
func SetAdminCredentialsPath(path string) string {
	previous := adminCredentialsPathOverride
	adminCredentialsPathOverride = path
	return previous
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

	user := system.SysUser{
		UserID:   1,
		UserName: username,
		Password: string(hash),
	}
	if err := DB.Create(&user).Error; err != nil {
		return err
	}

	// 落库成功后再写凭据文件，保证文件内容始终对应库里真正建成的那个管理员
	//（两个进程同时初始化空库时，Create 失败的一方不会碰文件）。
	// 写文件失败则回滚刚建的管理员：库回到空（count=0），下次启动重新生成并覆盖凭据文件；
	// 否则会留下「库里已有管理员、但明文密码无人知晓」的死局。
	if err := writeAdminCredentials(username, password); err != nil {
		if rollbackErr := DB.Delete(&user).Error; rollbackErr != nil {
			logging.ErrorPack("database",
				"rollback default admin after credential write failure failed: %v", rollbackErr)
		}
		return err
	}

	logging.InfoPack("database", "default admin user created, credentials saved to %s", adminCredentialsFile)
	return nil
}

// writeAdminCredentials 把初始管理员凭据覆盖写入可执行文件同级的 admin-credentials.txt。
// 该文件是明文初始密码的唯一存放处（库里只有 bcrypt 哈希），所以新建管理员时必须覆盖旧内容，
// 否则数据库被删除后重新生成的管理员无法按文件里的账号登录。
// 用同目录的 os.CreateTemp（默认 0600、文件名唯一）写临时文件再原子改名：
// 既避免复用固定 ".tmp" 名继承旧文件的宽松权限（Unix 上可能被同机其他用户读到明文密码），
// 也避免两个写者撞同一临时文件；任何一步失败都删除临时文件。
func writeAdminCredentials(username, password string) error {
	path := adminCredentialsPathOverride
	if path == "" {
		dir, err := executableDir()
		if err != nil {
			return err
		}
		path = filepath.Join(dir, adminCredentialsFile)
	}

	content := strings.Join([]string{
		"NyxBot initial administrator credentials",
		"",
		fmt.Sprintf("username: %s", username),
		fmt.Sprintf("password: %s", password),
		"",
		"Please change the password after first login.",
	}, "\n") + "\n"

	tmp, err := os.CreateTemp(filepath.Dir(path), adminCredentialsFile+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.WriteString(content); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
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
