//go:build linux && arm64

package ocr

import _ "embed"

// ortLibBytes 内嵌的 Linux arm64 onnxruntime 动态库（go-ocr 经 purego 从文件路径加载）。
//
//go:embed assets/lib/onnxruntime_arm64.so
var ortLibBytes []byte

// ortLibName 是释放到磁盘后的库文件名。
const ortLibName = "onnxruntime_arm64.so"
