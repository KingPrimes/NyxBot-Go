package tests

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
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

// TestOCREngineRecognize 端到端识别紫卡截图（PP-OCRv6 + 内嵌 onnxruntime 库）。
// 模型文件不随仓库提供（运行期下载），缺失时跳过；本地开发可设置
// NYXBOT_TEST_OCR_MODEL_DIR 指向模型目录以实际执行。
func TestOCREngineRecognize(t *testing.T) {
	modelDir := resolveOCRModelDir(t)
	if _, err := os.Stat(filepath.Join(modelDir, "det.onnx")); err != nil {
		t.Skipf("模型目录 %s 不可用，跳过（可用 NYXBOT_TEST_OCR_MODEL_DIR 指定）", modelDir)
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

	engine, err := ocr.New(ocr.Config{ModelDir: modelDir})
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
	text := sb.String()
	t.Logf("识别到 %d 个文本框:\n%s", len(results), text)

	for _, want := range []string{"守望者", "装填速度", "6.5%", "暴击几率", "段位"} {
		if !strings.Contains(text, want) {
			t.Errorf("识别结果缺少 %q", want)
		}
	}
}
