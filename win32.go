//go:build windows

package main

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

const DWMWA_NCRENDERING_POLICY = 2
const DWMNCRP_DISABLED = 1

var (
	dwmapi                    = windows.NewLazySystemDLL("dwmapi.dll")
	procDwmSetWindowAttribute = dwmapi.NewProc("DwmSetWindowAttribute")
	modUser32                 = windows.NewLazySystemDLL("user32.dll")
	procFindWindow            = modUser32.NewProc("FindWindowW")
	procSetWindowLong         = modUser32.NewProc("SetWindowLongPtrW")
	procGetWindowLong         = modUser32.NewProc("GetWindowLongPtrW")
)

func setNoShadow() {
	hwnd := findBiliQueueWindow()
	if hwnd == 0 {
		return
	}
	// 禁用 DWM 非客户区渲染 = 无阴影
	policy := uint32(DWMNCRP_DISABLED)
	procDwmSetWindowAttribute.Call(hwnd, uintptr(DWMWA_NCRENDERING_POLICY),
		uintptr(unsafe.Pointer(&policy)), uintptr(unsafe.Sizeof(policy)))
	// 去掉 WS_EX_WINDOWEDGE 等边框
	const GWL_EXSTYLE = ^uintptr(19) // -20
	style, _, _ := procGetWindowLong.Call(hwnd, GWL_EXSTYLE)
	const WS_EX_WINDOWEDGE = 0x00000100
	style &^= WS_EX_WINDOWEDGE
	procSetWindowLong.Call(hwnd, GWL_EXSTYLE, style)
}

func restoreShadow() {
	hwnd := findBiliQueueWindow()
	if hwnd == 0 {
		return
	}
	policy := uint32(0) // 恢复默认
	procDwmSetWindowAttribute.Call(hwnd, uintptr(DWMWA_NCRENDERING_POLICY),
		uintptr(unsafe.Pointer(&policy)), uintptr(unsafe.Sizeof(policy)))
}

func findBiliQueueWindow() uintptr {
	name, _ := windows.UTF16PtrFromString("B站排队助手")
	hwnd, _, _ := procFindWindow.Call(0, uintptr(unsafe.Pointer(name)))
	return hwnd
}
