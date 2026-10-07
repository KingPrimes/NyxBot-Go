// Package ocr 提供截图文字识别能力：基于 PP-OCRv6 模型与 onnxruntime CPU 推理，
// 推理引擎为 GetcharZp/go-ocr（纯 Go 绑定，无 CGO）。
//
// 内嵌资产：onnxruntime 动态库（按平台 build tag 选择）与 PP-OCRv6 字典，
// 首次使用时释放到缓存目录（引擎需从文件路径加载）。
// 模型文件（det.onnx / rec.onnx）不入库：启动时由 Prepare 后台校验，
// 缺失/损坏时按 DownloadSource 双源回退下载到 ModelDir，就绪后经 Ready 获取引擎。
package ocr

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"path/filepath"
	"sync"

	gocr "github.com/getcharzp/go-ocr"
	"github.com/getcharzp/go-ocr/paddle"
)

// ErrDestroyed 表示引擎已释放（Destroy 之后），不能继续识别。
var ErrDestroyed = errors.New("OCR 引擎已释放")

// Config 引擎配置。
type Config struct {
	// ModelDir 模型目录，需包含 det.onnx 与 rec.onnx。
	ModelDir string
	// ThreadCount 识别并发 session 数，默认 1；适配低配服务器时可保持默认。
	ThreadCount int
	// UseRGB 按 RGB 通道顺序喂入模型。默认 false（BGR）：PaddleOCR 系模型以
	// BGR 训练，BGR 输入实测精度更优（平均置信度 +1.2%），并修复个别截图中
	// 价格数字多识别字符的问题；仅当换用以 RGB 训练的模型时才需要开启。
	UseRGB bool
	// AutoDownload 模型缺失/损坏时自动下载（Prepare 使用）。
	AutoDownload bool
	// DownloadSource 模型下载源：""/"auto"=ModelScope 优先、HuggingFace 回退；
	// 或指定 "modelscope" / "huggingface"（Prepare 使用）。
	DownloadSource string
}

// Engine 封装 go-ocr 的 PaddleOCR 推理引擎。
// 并发安全：Recognize 可并发调用（onnxruntime session 的 Run 线程安全），
// Destroy 与其他操作互斥（等待进行中的识别结束后释放，释放后 Recognize 返回 ErrDestroyed）。
type Engine struct {
	mu     sync.RWMutex
	inner  *paddle.Engine
	useRGB bool
}

// New 创建 OCR 引擎：释放内嵌的 onnxruntime 库与字典到缓存目录，再加载模型。
// 模型文件缺失时不在此处报错（由 go-ocr 加载时返回错误）。
func New(cfg Config) (*Engine, error) {
	cacheDir, err := resolveCacheDir()
	if err != nil {
		return nil, fmt.Errorf("确定资产缓存目录: %w", err)
	}
	libPath, err := extractAsset(cacheDir, ortLibName, ortLibBytes)
	if err != nil {
		return nil, fmt.Errorf("释放 onnxruntime 库: %w", err)
	}
	dictPath, err := extractAsset(cacheDir, "ppocrv6_dict.txt", dictBytes)
	if err != nil {
		return nil, fmt.Errorf("释放字典: %w", err)
	}

	inner, err := paddle.NewEngine(paddle.Config{
		OnnxRuntimeLibPath: libPath,
		DetModelPath:       filepath.Join(cfg.ModelDir, "det.onnx"),
		RecModelPath:       filepath.Join(cfg.ModelDir, "rec.onnx"),
		DictPath:           dictPath,
		ThreadCount:        cfg.ThreadCount,
	})
	if err != nil {
		return nil, fmt.Errorf("初始化 OCR 引擎: %w", err)
	}
	return &Engine{inner: inner, useRGB: cfg.UseRGB}, nil
}

// Recognize 对图像执行文字检测与识别，返回文本行结果。
func (e *Engine) Recognize(img image.Image) ([]gocr.RecResult, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.inner == nil {
		return nil, ErrDestroyed
	}
	// PaddleOCR 模型以 BGR 训练，而 go-ocr 预处理按 RGB 顺序写入输入张量；
	// 默认传入交换 R/B 的图让引擎实际收到 BGR（UseRGB 时保持原样）。
	// 缩放与通道交换均为线性操作，调用前交换与在预处理内交换数学等价
	//（仅有浮点舍入量级的差异）。
	if !e.useRGB {
		img = swapRB(img)
	}
	return e.inner.RunOCR(img)
}

// Destroy 释放引擎资源（onnxruntime session 与内存）。
// 与 Recognize 互斥：等待进行中的识别结束；重复调用安全。
func (e *Engine) Destroy() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.inner == nil {
		return
	}
	e.inner.Destroy()
	e.inner = nil
}

// swapRB 返回 R/B 通道互换后的图像副本。
func swapRB(src image.Image) image.Image {
	b := src.Bounds()
	dst := image.NewNRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.NRGBAModel.Convert(src.At(x, y)).(color.NRGBA)
			dst.SetNRGBA(x, y, color.NRGBA{R: c.B, G: c.G, B: c.R, A: c.A})
		}
	}
	return dst
}
