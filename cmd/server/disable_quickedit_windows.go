//go:build windows

// Package main Windows 平台专用：启动时禁用控制台的 QuickEdit 模式，防止误点击阻塞进程。
package main

import "golang.org/x/sys/windows"

// init 禁用 Windows 控制台 QuickEdit 模式，避免程序因鼠标点击暂停。
func init() {
	hStdin, err := windows.GetStdHandle(windows.STD_INPUT_HANDLE)
	if err != nil {
		return
	}
	var mode uint32
	if err := windows.GetConsoleMode(hStdin, &mode); err != nil {
		return
	}
	mode &^= 0x0040
	mode |= 0x0080
	windows.SetConsoleMode(hStdin, mode)
}
