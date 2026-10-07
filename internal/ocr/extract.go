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
// 文件名携带内容哈希（前 4 字节）：内容变化会产生新文件；
// 复用缓存前校验文件大小（防截断/损坏的缓存导致反复加载失败）；
// 临时文件用 os.CreateTemp 唯一命名，进程内/多进程并发安全（原子重命名）。
func extractAsset(dir, baseName string, data []byte) (string, error) {
	sum := sha256.Sum256(data)
	ext := filepath.Ext(baseName)
	stem := strings.TrimSuffix(baseName, ext)
	path := filepath.Join(dir, fmt.Sprintf("%s-%s%s", stem, hex.EncodeToString(sum[:4]), ext))

	if info, err := os.Stat(path); err == nil && info.Size() == int64(len(data)) {
		return path, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("创建缓存目录 %s: %w", dir, err)
	}

	f, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return "", fmt.Errorf("创建临时文件: %w", err)
	}
	tmp := f.Name()
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return "", fmt.Errorf("写入临时文件: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return "", fmt.Errorf("关闭临时文件: %w", err)
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
