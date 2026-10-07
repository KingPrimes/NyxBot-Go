package ocr

import _ "embed"

// dictBytes 内嵌的 PP-OCRv6 字符字典（18709 行：18708 个字符 + 末尾空格类）。
// 末尾的空格类对应 PaddleOCR use_space_char 的最后一类，缺失会导致空格被解码为 "?"。
// 来源：PaddlePaddle/PP-OCRv6（Apache 2.0）。
//
//go:embed assets/ppocrv6_dict.txt
var dictBytes []byte
