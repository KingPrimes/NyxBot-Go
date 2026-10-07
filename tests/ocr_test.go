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
