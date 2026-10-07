package tests

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"nyxbot-go/internal/ocr"
)

// resolveOCRModelDir 返回测试用 OCR 模型目录：
// 优先环境变量 NYXBOT_TEST_OCR_MODEL_DIR，否则尝试与仓库相邻的 ocr-bench 实验目录
//（D:\Demos\ocr-bench\models\v6，含 PP-OCRv6 的 det.onnx 与 rec.onnx）。
func resolveOCRModelDir(t *testing.T) string {
	t.Helper()
	if dir := os.Getenv("NYXBOT_TEST_OCR_MODEL_DIR"); dir != "" {
		return dir
	}
	return filepath.Join("..", "..", "ocr-bench", "models", "v6")
}

// recognizeRivenSample 用给定配置识别紫卡样例图，返回全部文本（按识别行拼接）。
// 模型文件不随仓库提供（运行期下载），缺失时跳过。
func recognizeRivenSample(t *testing.T, cfg ocr.Config) string {
	t.Helper()
	if _, err := os.Stat(filepath.Join(cfg.ModelDir, "det.onnx")); err != nil {
		t.Skipf("模型目录 %s 不可用，跳过（可用 NYXBOT_TEST_OCR_MODEL_DIR 指定）", cfg.ModelDir)
	}

	f, err := os.Open(filepath.Join("data", "ocr", "riven_sample.png"))
	if err != nil {
		t.Fatalf("打开测试图片失败: %v", err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("解码测试图片失败: %v", err)
	}

	engine, err := ocr.New(cfg)
	if err != nil {
		t.Fatalf("创建 OCR 引擎失败: %v", err)
	}
	defer engine.Destroy()

	results, err := engine.Recognize(img)
	if err != nil {
		t.Fatalf("识别失败: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("未识别到任何文本框")
	}

	var sb strings.Builder
	for _, r := range results {
		sb.WriteString(r.Text)
		sb.WriteByte('\n')
	}
	return sb.String()
}

// TestOCREngineRecognize 默认（BGR）模式端到端识别紫卡截图（PP-OCRv6 + 内嵌 onnxruntime 库）。
func TestOCREngineRecognize(t *testing.T) {
	text := recognizeRivenSample(t, ocr.Config{ModelDir: resolveOCRModelDir(t)})
	t.Logf("识别结果:\n%s", text)

	for _, want := range []string{"守望者", "装填速度", "6.5%", "暴击几率", "段位"} {
		if !strings.Contains(text, want) {
			t.Errorf("识别结果缺少 %q", want)
		}
	}
}

// TestOCREngineRecognizeRGBMode UseRGB 开关关闭 R/B 交换后仍可正常识别。
func TestOCREngineRecognizeRGBMode(t *testing.T) {
	text := recognizeRivenSample(t, ocr.Config{ModelDir: resolveOCRModelDir(t), UseRGB: true})
	t.Logf("RGB 模式识别结果:\n%s", text)

	for _, want := range []string{"守望者", "装填速度", "段位"} {
		if !strings.Contains(text, want) {
			t.Errorf("识别结果缺少 %q", want)
		}
	}
}

// TestOCRVerifyModel 校验逻辑：完整文件通过；大小不符 / 哈希不符 / 文件缺失失败。
func TestOCRVerifyModel(t *testing.T) {
	dir := t.TempDir()
	content := []byte("fake onnx model content")
	sum := sha256.Sum256(content)
	if err := os.WriteFile(filepath.Join(dir, "m.onnx"), content, 0o644); err != nil {
		t.Fatal(err)
	}
	spec := ocr.ModelSpec{FileName: "m.onnx", Size: int64(len(content)), SHA256: hex.EncodeToString(sum[:])}

	if err := ocr.VerifyModel(dir, spec); err != nil {
		t.Errorf("完整文件应校验通过: %v", err)
	}

	badSize := spec
	badSize.Size = spec.Size + 1
	if err := ocr.VerifyModel(dir, badSize); err == nil {
		t.Error("大小不符应校验失败")
	}

	badHash := spec
	badHash.SHA256 = strings.Repeat("0", 64)
	if err := ocr.VerifyModel(dir, badHash); err == nil {
		t.Error("哈希不符应校验失败")
	}

	missing := ocr.ModelSpec{FileName: "not-exist.onnx", Size: 1, SHA256: spec.SHA256}
	if err := ocr.VerifyModel(dir, missing); err == nil {
		t.Error("文件缺失应校验失败")
	}
}

// TestOCREnsureModelDownloadAndSkip 下载成功并落地；已完整存在时不重复请求。
func TestOCREnsureModelDownloadAndSkip(t *testing.T) {
	content := []byte("fake onnx model content for download test")
	sum := sha256.Sum256(content)
	dir := t.TempDir()

	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = w.Write(content)
	}))
	defer srv.Close()

	spec := ocr.ModelSpec{
		FileName: "m.onnx",
		Size:     int64(len(content)),
		SHA256:   hex.EncodeToString(sum[:]),
		Sources:  []string{srv.URL},
	}

	if err := ocr.EnsureModel(context.Background(), srv.Client(), dir, spec, ""); err != nil {
		t.Fatalf("下载失败: %v", err)
	}
	if err := ocr.VerifyModel(dir, spec); err != nil {
		t.Errorf("下载后校验应通过: %v", err)
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("应请求 1 次，实际 %d", got)
	}

	if err := ocr.EnsureModel(context.Background(), srv.Client(), dir, spec, ""); err != nil {
		t.Fatalf("二次确保失败: %v", err)
	}
	if got := hits.Load(); got != 1 {
		t.Errorf("校验已通过时不应重新下载，请求数 %d", got)
	}
}

// TestOCREnsureModelSourceFallback 第一源失败时回退第二源（双源回退）。
func TestOCREnsureModelSourceFallback(t *testing.T) {
	content := []byte("fallback content")
	sum := sha256.Sum256(content)
	dir := t.TempDir()

	fail := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer fail.Close()
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(content)
	}))
	defer ok.Close()

	spec := ocr.ModelSpec{
		FileName: "m.onnx",
		Size:     int64(len(content)),
		SHA256:   hex.EncodeToString(sum[:]),
		Sources:  []string{fail.URL, ok.URL},
	}
	if err := ocr.EnsureModel(context.Background(), fail.Client(), dir, spec, ""); err != nil {
		t.Fatalf("应回退到第二源并成功: %v", err)
	}
	if err := ocr.VerifyModel(dir, spec); err != nil {
		t.Errorf("下载后校验应通过: %v", err)
	}
}

// TestOCREnsureModelAllSourcesFail 全部源失败时报错，且不残留 .part 临时文件。
func TestOCREnsureModelAllSourcesFail(t *testing.T) {
	dir := t.TempDir()
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer bad.Close()

	spec := ocr.ModelSpec{
		FileName: "m.onnx",
		Size:     100,
		SHA256:   strings.Repeat("a", 64),
		Sources:  []string{bad.URL},
	}
	if err := ocr.EnsureModel(context.Background(), bad.Client(), dir, spec, ""); err == nil {
		t.Fatal("全部源失败应返回错误")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("失败后不应残留文件: %v", entries)
	}
}

// TestOCRUPrepareLocalModels Prepare 校验通过本地模型后引擎就绪（不触发下载）。
func TestOCRUPrepareLocalModels(t *testing.T) {
	modelDir := resolveOCRModelDir(t)
	if _, err := os.Stat(filepath.Join(modelDir, "det.onnx")); err != nil {
		t.Skipf("模型目录 %s 不可用，跳过（可用 NYXBOT_TEST_OCR_MODEL_DIR 指定）", modelDir)
	}

	ocr.Prepare(context.Background(), ocr.Config{ModelDir: modelDir, AutoDownload: false})
	engine, err := ocr.Ready()
	if err != nil {
		t.Fatalf("本地模型应准备就绪: %v", err)
	}
	if engine == nil {
		t.Fatal("Ready 返回空引擎")
	}
}

// TestOCRUPrepareMissingWithoutDownload 模型缺失且关闭自动下载时，Prepare 失败且 Ready 返回错误。
func TestOCRUPrepareMissingWithoutDownload(t *testing.T) {
	ocr.Prepare(context.Background(), ocr.Config{ModelDir: t.TempDir(), AutoDownload: false})
	if _, err := ocr.Ready(); err == nil {
		t.Error("模型缺失且未开启下载时不应就绪")
	}
}
