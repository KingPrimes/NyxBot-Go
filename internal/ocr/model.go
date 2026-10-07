package ocr

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ModelSpec 描述单个模型文件的规格与下载源。
// 校验以 SHA256 + 大小为准：检测下载中断、文件损坏或被替换。
type ModelSpec struct {
	FileName string   // 文件名（det.onnx / rec.onnx）
	Size     int64    // 预期字节数
	SHA256   string   // 预期 SHA256（小写 hex）
	Sources  []string // 下载 URL，按序尝试（前者失败回退后者）
}

// modelSpecs PP-OCRv6 模型规格（medium_det + small_rec）。
// URL 固定到具体 revision/commit：上游 master/main 更新后内容漂移会破坏
// SHA256 校验，固定后下载内容与哈希恒定；升级模型需同步更新 URL 与哈希。
var modelSpecs = []ModelSpec{
	{
		FileName: "det.onnx",
		Size:     62032837,
		SHA256:   "eb13b44b25bb36f89528b68720af8a61d9cf381176107f465db1757b65d086e1",
		Sources: []string{
			"https://www.modelscope.cn/models/PaddlePaddle/PP-OCRv6_medium_det_onnx/resolve/8cb026ecab7a28b7f7e479dccd7f93ebe3ff47c1/inference.onnx",
			"https://huggingface.co/PaddlePaddle/PP-OCRv6_medium_det_onnx/resolve/61323801669c338b7891481ec7bac61ce31b576a/inference.onnx",
		},
	},
	{
		FileName: "rec.onnx",
		Size:     21159378,
		SHA256:   "5435fd747c9e0efe15a96d0b378d5bd157e9492ed8fd80edf08f30d02fa24634",
		Sources: []string{
			"https://www.modelscope.cn/models/PaddlePaddle/PP-OCRv6_small_rec_onnx/resolve/ba215b1cc49d9ed4459d161b96778e8643fe0c1f/inference.onnx",
			"https://huggingface.co/PaddlePaddle/PP-OCRv6_small_rec_onnx/resolve/b8f84f0b80c529de40b4fbb3544b84fa7233a513/inference.onnx",
		},
	},
}

// SourcesFor 按下载源选项过滤模型下载源列表。
// source 为 ""/"auto" 返回全部（ModelScope 优先、HuggingFace 回退）；
// "modelscope"/"huggingface" 仅保留对应源；未知取值报错。
func SourcesFor(spec ModelSpec, source string) ([]string, error) {
	switch source {
	case "", "auto":
		return spec.Sources, nil
	case "modelscope":
		return spec.Sources[:1], nil
	case "huggingface":
		return spec.Sources[1:], nil
	default:
		return nil, fmt.Errorf("未知下载源 %q（可选 auto/modelscope/huggingface）", source)
	}
}

// VerifyModel 校验模型文件是否完整：存在、大小与 SHA256 均匹配时返回 nil。
func VerifyModel(dir string, spec ModelSpec) error {
	path := filepath.Join(dir, spec.FileName)

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("模型 %s 不可用: %w", spec.FileName, err)
	}
	if info.Size() != spec.Size {
		return fmt.Errorf("模型 %s 大小不符：期望 %d 字节，实际 %d", spec.FileName, spec.Size, info.Size())
	}

	sum, err := fileSHA256(path)
	if err != nil {
		return fmt.Errorf("计算 %s 哈希: %w", spec.FileName, err)
	}
	if sum != spec.SHA256 {
		return fmt.Errorf("模型 %s 哈希不符：期望 %s，实际 %s", spec.FileName, spec.SHA256, sum)
	}
	return nil
}

// fileSHA256 计算文件内容的 SHA256（小写 hex）。
func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
