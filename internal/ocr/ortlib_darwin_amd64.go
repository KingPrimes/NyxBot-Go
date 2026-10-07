//go:build darwin && amd64

package ocr

import _ "embed"

// ortLibBytes 内嵌的 macOS amd64 onnxruntime 动态库（go-ocr 经 purego 从文件路径加载）。
//
//go:embed assets/lib/onnxruntime_amd64.dylib
var ortLibBytes []byte

// ortLibName 是释放到磁盘后的库文件名。
const ortLibName = "onnxruntime_amd64.dylib"
