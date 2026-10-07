//go:build windows

package ocr

import _ "embed"

// ortLibBytes 内嵌的 Windows amd64 onnxruntime 动态库（go-ocr 经 purego 从文件路径加载）。
//
//go:embed assets/lib/onnxruntime.dll
var ortLibBytes []byte

// ortLibName 是释放到磁盘后的库文件名。
const ortLibName = "onnxruntime.dll"
