package ocr

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"nyxbot-go/internal/logging"
)

// downloadTimeout 单个模型下载的总超时（62MB 模型在慢速网络下也够用）。
const downloadTimeout = 15 * time.Minute

// 包级引擎状态：Prepare 在后台写入，Ready 供指令侧读取。
var (
	stateMu     sync.RWMutex
	stateEngine *Engine // 非 nil 表示就绪
	stateErr    error   // 准备失败的原因；nil 且 engine 为 nil 表示仍在准备
)

// ErrNotReady 表示 OCR 引擎尚未准备完成（模型校验/下载中）。
var ErrNotReady = errors.New("OCR 引擎尚未就绪（模型校验/下载中）")

// Prepare 在后台准备 OCR 能力：逐个校验模型完整性，缺失/损坏且开启
// AutoDownload 时按 DownloadSource 双源回退下载，全部就绪后初始化引擎。
// 阻塞函数，供 go Prepare(ctx, cfg) 在启动时调用，不阻塞主流程；
// ctx 取消（服务关闭）会中断下载。
func Prepare(ctx context.Context, cfg Config) {
	// 重新准备：先清空旧状态；旧引擎在此销毁，避免重复准备时资源泄漏。
	stateMu.Lock()
	old := stateEngine
	stateEngine = nil
	stateErr = nil
	stateMu.Unlock()
	if old != nil {
		old.Destroy()
	}

	if _, err := SourcesFor(modelSpecs[0], cfg.DownloadSource); err != nil {
		setFailed(fmt.Errorf("下载源配置错误: %w", err))
		return
	}

	client := &http.Client{Timeout: downloadTimeout}
	for _, spec := range modelSpecs {
		verifyErr := VerifyModel(cfg.ModelDir, spec)
		if verifyErr == nil {
			logging.InfoPack("ocr", "模型 %s 校验通过", spec.FileName)
			continue
		}
		logging.WarnPack("ocr", "模型校验未通过: %v", verifyErr)
		if !cfg.AutoDownload {
			setFailed(fmt.Errorf("模型 %s 不可用且未开启自动下载（ocr.auto_download）", spec.FileName))
			return
		}
		if err := EnsureModel(ctx, client, cfg.ModelDir, spec, cfg.DownloadSource); err != nil {
			setFailed(err)
			return
		}
	}

	engine, err := New(cfg)
	if err != nil {
		setFailed(fmt.Errorf("初始化 OCR 引擎: %w", err))
		return
	}

	stateMu.Lock()
	stateEngine = engine
	stateErr = nil
	stateMu.Unlock()
	logging.InfoPack("ocr", "OCR 引擎就绪（模型目录 %s）", cfg.ModelDir)
}

// Ready 返回已就绪的识别引擎；仍在准备时返回 ErrNotReady，准备失败时返回失败原因。
func Ready() (*Engine, error) {
	stateMu.RLock()
	defer stateMu.RUnlock()
	if stateEngine != nil {
		return stateEngine, nil
	}
	if stateErr != nil {
		return nil, stateErr
	}
	return nil, ErrNotReady
}

// setFailed 记录准备失败状态（清空引擎引用，保证 Ready 不会返回旧引擎）并输出错误日志。
func setFailed(err error) {
	stateMu.Lock()
	stateEngine = nil
	stateErr = err
	stateMu.Unlock()
	logging.ErrorPack("ocr", "OCR 准备失败: %v", err)
}
