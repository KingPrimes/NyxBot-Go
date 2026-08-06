// 官方导出文件下载器，对应 Java NyxBot 的 ExportFilePath
// 流程：下载 LZMA 索引 → 解压 → 解析 "文件名!hash" → 与 ./data/keys.json 对比 →
// 仅下载 hash 变化的文件到 ./data/export/；失败时降级使用本地缓存
package warframe

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ulikunitz/xz/lzma"

	"nyxbot-go/internal/logging"
)

const (
	exportIndexURL = "https://origin.warframe.com/PublicExport/index_%s.txt.lzma" // LZMA 索引
	exportFileURL  = "http://content.warframe.com/PublicExport/Manifest/%s"       // 文件下载（http，对齐 Java）
	exportDir      = "./data/export"                                              // 导出文件目录
	exportKeysFile = "./data/keys.json"                                           // 文件名 -> hash 映射
	exportLzmaDir  = "./data/lzma"                                                // LZMA 索引目录
	exportDataURL  = "http://content.warframe.com/PublicExport/%s"                // 通用导出 URL（备用）
)

// ExportFilePath 管理导出文件索引与下载。
type ExportFilePath struct {
	client *http.Client
	locale string // 语言前缀，如 "zh"
}

// NewExportFilePath 创建导出文件管理器；client 为 nil 时使用 http.DefaultClient。
func NewExportFilePath(client *http.Client, locale string) *ExportFilePath {
	if client == nil {
		client = http.DefaultClient
	}
	if locale == "" {
		locale = "zh"
	}
	return &ExportFilePath{client: client, locale: locale}
}

// SeverExportFiles 执行一次导出文件同步（对齐 Java severExportFiles）：
// 下载索引 → 对比 hash → 下载变化文件；无变化返回 nil，全部成功返回 nil。
// 返回 changed 表示是否发生了文件更新。
func (exporter *ExportFilePath) SeverExportFiles(ctx context.Context) (bool, error) {
	index, err := exporter.fetchAndParseIndex(ctx)
	if err != nil {
		if exporter.localCacheExists() {
			return false, nil
		}
		return false, err
	}

	changed, err := exporter.saveKeysAndDiff(index)
	if err != nil {
		return false, err
	}
	if len(changed) == 0 {
		return false, nil
	}

	for _, filename := range changed {
		if strings.Contains(filename, "ExportRecipes") || strings.Contains(filename, "ExportFusionBundles") {
			continue
		}
		if err := exporter.downloadFile(ctx, filename); err != nil {
			logging.WarnPack("warframe.export", "export file %s download failed: %v", filename, err)
			if exporter.localCacheExists() {
				continue
			}
			return false, err
		}
	}
	return true, nil
}

// fetchAndParseIndex 下载并解压 LZMA 索引，解析为 文件名 -> hash 映射。
func (exporter *ExportFilePath) fetchAndParseIndex(ctx context.Context) (map[string]string, error) {
	if err := os.MkdirAll(exportLzmaDir, 0o755); err != nil {
		return nil, err
	}
	lzmaPath := filepath.Join(exportLzmaDir, fmt.Sprintf("index_%s.txt.lzma", exporter.locale))
	plainPath := filepath.Join(exportLzmaDir, fmt.Sprintf("index_%s.txt", exporter.locale))

	url := fmt.Sprintf(exportIndexURL, exporter.locale)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	response, err := exporter.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("download export index: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 400 {
		return nil, fmt.Errorf("export index returned HTTP %d", response.StatusCode)
	}
	compressed, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("read export index: %w", err)
	}
	if err := os.WriteFile(lzmaPath, compressed, 0o644); err != nil {
		return nil, err
	}

	plain, err := lzmaDecompress(compressed)
	if err != nil {
		return nil, fmt.Errorf("decompress export index: %w", err)
	}
	if err := os.WriteFile(plainPath, plain, 0o644); err != nil {
		return nil, err
	}
	return parseIndexLines(plain), nil
}

// lzmaDecompress 使用 LZMAInputStream 语义解压 .lzma 文件。
func lzmaDecompress(compressed []byte) ([]byte, error) {
	reader, err := lzma.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, err
	}
	return io.ReadAll(reader)
}

// parseIndexLines 解析索引行 "文件名!hash"。
func parseIndexLines(plain []byte) map[string]string {
	result := make(map[string]string)
	scanner := bufio.NewScanner(bytes.NewReader(plain))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "!", 2)
		if len(parts) == 2 && parts[0] != "" {
			result[parts[0]] = parts[1]
		}
	}
	return result
}

// saveKeysAndDiff 写回 keys.json 并返回 hash 有变化的文件名集合。
func (exporter *ExportFilePath) saveKeysAndDiff(index map[string]string) ([]string, error) {
	old := loadKeys()
	changed := make([]string, 0)
	for filename, hash := range index {
		if old[filename] != hash {
			changed = append(changed, filename)
		}
	}
	sort.Strings(changed)

	if err := os.MkdirAll(filepath.Dir(exportKeysFile), 0o755); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(index)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(exportKeysFile, encoded, 0o644); err != nil {
		return nil, err
	}
	return changed, nil
}

// loadKeys 读取 keys.json；不存在返回空映射。
func loadKeys() map[string]string {
	raw, err := os.ReadFile(exportKeysFile)
	if err != nil {
		return map[string]string{}
	}
	var result map[string]string
	if err := json.Unmarshal(raw, &result); err != nil {
		return map[string]string{}
	}
	return result
}

// downloadFile 下载单个导出文件到 ./data/export/。
func (exporter *ExportFilePath) downloadFile(ctx context.Context, filename string) error {
	if err := os.MkdirAll(exportDir, 0o755); err != nil {
		return err
	}
	url := fmt.Sprintf(exportFileURL, filename)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	response, err := exporter.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 400 {
		return fmt.Errorf("export file returned HTTP %d", response.StatusCode)
	}
	target := filepath.Join(exportDir, filename)
	tmp := target + ".tmp"
	file, err := os.Create(tmp)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(file, response.Body)
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	return os.Rename(tmp, target)
}

// localCacheExists 判断本地导出缓存是否可用（目录存在且非空）。
func (exporter *ExportFilePath) localCacheExists() bool {
	entries, err := os.ReadDir(exportDir)
	return err == nil && len(entries) > 0
}

// Resolve 解析导出文件本地路径：前缀 -> 完整文件名（对齐 Java resolve）。
// 优先从 keys.json 映射取（取 "_" 前部分），未命中回退 "{prefix}_zh.json"。
func (exporter *ExportFilePath) Resolve(prefix string) string {
	keys := loadKeys()
	best := ""
	for filename := range keys {
		keyPrefix, _, _ := strings.Cut(filename, "_")
		if keyPrefix == prefix {
			if best == "" || filename < best {
				best = filename
			}
		}
	}
	if best == "" {
		best = fmt.Sprintf("%s_%s.json", prefix, exporter.locale)
	}
	return filepath.Join(exportDir, best)
}

// ReadExportFile 读取指定前缀的导出文件内容；文件不存在返回 nil。
func (exporter *ExportFilePath) ReadExportFile(prefix string) ([]byte, error) {
	path := exporter.Resolve(prefix)
	return os.ReadFile(path)
}

// ExportDataURL 返回通用导出 URL（供测试与外部使用）。
func ExportDataURL(filename string) string {
	return fmt.Sprintf(exportDataURL, filename)
}

// ExportHTTPClient 供导出相关函数复用的默认客户端（带超时）。
func ExportHTTPClient() *http.Client {
	return &http.Client{Timeout: 30 * time.Second}
}
