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
