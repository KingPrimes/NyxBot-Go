package ocr

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"nyxbot-go/internal/logging"
)

// downloadProgressStep 下载进度日志的字节步长（每读取约 8MB 记录一次）。
const downloadProgressStep = 8 << 20

// EnsureModel 确保模型可用：校验通过直接返回；否则按 source 选项从
// spec.Sources 回退下载（下载后校验，失败会自动尝试下一个源）。
func EnsureModel(ctx context.Context, client *http.Client, dir string, spec ModelSpec, source string) error {
	if err := VerifyModel(dir, spec); err == nil {
		return nil
	}
	sources, err := SourcesFor(spec, source)
	if err != nil {
		return err
	}
	return downloadModel(ctx, client, dir, spec, sources)
}

// downloadModel 按顺序尝试 sources 中的下载源获取模型文件：
// 下载到临时 .part 文件 → SHA256 校验 → 原子重命名到正式路径。
// 任一源成功即返回；全部失败返回汇总错误。ctx 取消可中断下载。
func downloadModel(ctx context.Context, client *http.Client, dir string, spec ModelSpec, sources []string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建模型目录 %s: %w", dir, err)
	}

	var lastErr error
	for i, source := range sources {
		if err := ctx.Err(); err != nil {
			return err
		}
		logging.InfoPack("ocr", "下载模型 %s（源 %d/%d）: %s", spec.FileName, i+1, len(sources), source)
		if err := downloadFromSource(ctx, client, dir, spec, source); err != nil {
			logging.WarnPack("ocr", "模型 %s 从 %s 下载失败: %v", spec.FileName, source, err)
			lastErr = err
			continue
		}
		logging.InfoPack("ocr", "模型 %s 下载完成并通过校验", spec.FileName)
		return nil
	}
	return fmt.Errorf("模型 %s 全部下载源失败: %w", spec.FileName, lastErr)
}

// downloadFromSource 从单个源下载并校验模型文件。
func downloadFromSource(ctx context.Context, client *http.Client, dir string, spec ModelSpec, source string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	// 唯一临时文件：并发下载（多进程/重复 Prepare）互不干扰，不会互相截断
	f, err := os.CreateTemp(dir, spec.FileName+".*.part")
	if err != nil {
		return err
	}
	partPath := f.Name()
	defer func() {
		f.Close()
		// 成功路径已重命名，此处对残留（失败）的 .part 做清理
		_ = os.Remove(partPath)
	}()

	h := sha256.New()
	pr := &progressReader{inner: resp.Body, total: resp.ContentLength, name: spec.FileName}
	if _, err := io.Copy(io.MultiWriter(f, h), pr); err != nil {
		return err
	}

	sum := hex.EncodeToString(h.Sum(nil))
	if sum != spec.SHA256 {
		return fmt.Errorf("SHA256 校验失败：期望 %s，实际 %s", spec.SHA256, sum)
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(partPath, filepath.Join(dir, spec.FileName))
}

// progressReader 统计已读取字节并按步长输出进度日志（支持 Content-Length 未知的情况）。
type progressReader struct {
	inner    io.Reader
	name     string
	total    int64 // 预期总大小，<=0 表示未知
	read     int64
	nextMark int64 // 下一次打点的字节位置
}

// Read 实现 io.Reader，读取到打点位置时记录进度。
func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.inner.Read(b)
	p.read += int64(n)
	if p.read >= p.nextMark {
		p.nextMark = p.read + downloadProgressStep
		if p.total > 0 {
			logging.InfoPack("ocr", "模型 %s 下载进度：%d%%（%d/%d 字节）", p.name, p.read*100/p.total, p.read, p.total)
		} else {
			logging.InfoPack("ocr", "模型 %s 下载进度：已接收 %d 字节", p.name, p.read)
		}
	}
	return n, err
}
