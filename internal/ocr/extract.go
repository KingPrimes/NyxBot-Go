package ocr

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// extractAsset 将内嵌资产释放到 dir 目录，返回磁盘路径。
// 文件名携带内容哈希（前 4 字节）：内容变化会产生新文件，存在即直接复用，
// 进程内/多进程并发安全（临时文件 + 原子重命名）。
func extractAsset(dir, baseName string, data []byte) (string, error) {
	sum := sha256.Sum256(data)
	ext := filepath.Ext(baseName)
	stem := strings.TrimSuffix(baseName, ext)
	path := filepath.Join(dir, fmt.Sprintf("%s-%s%s", stem, hex.EncodeToString(sum[:4]), ext))

	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("创建缓存目录 %s: %w", dir, err)
	}

	tmp := fmt.Sprintf("%s.tmp.%d", path, os.Getpid())
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return "", fmt.Errorf("写入临时文件: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return "", fmt.Errorf("重命名资产: %w", err)
	}
	return path, nil
}

// resolveCacheDir 返回资产释放目录：系统缓存目录下的 nyxbot/ocr。
// 系统缓存目录不可用（如容器内缺失 HOME）时回退到可执行文件同级的 data/ocr。
func resolveCacheDir() (string, error) {
	if base, err := os.UserCacheDir(); err == nil {
		return filepath.Join(base, "nyxbot", "ocr"), nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("定位可执行文件: %w", err)
	}
	return filepath.Join(filepath.Dir(exe), "data", "ocr"), nil
}
