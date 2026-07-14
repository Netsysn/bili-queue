//go:build windows

package main

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	dwmapi              = windows.NewLazySystemDLL("dwmapi.dll")
	procDwmExtendFrame  = dwmapi.NewProc("DwmExtendFrameIntoClientArea")
	procFindWindow      = windows.NewLazySystemDLL("user32.dll").NewProc("FindWindowW")
)

func removeShadow() {
	hwnd := findBiliQueueWindow()
	if hwnd == 0 {
		return
	}
	margins := [4]int32{-1, -1, -1, -1}
	procDwmExtendFrame.Call(hwnd, uintptr(unsafe.Pointer(&margins[0])))
}

func findBiliQueueWindow() uintptr {
	name, _ := windows.UTF16PtrFromString("B站排队助手")
	hwnd, _, _ := procFindWindow.Call(0, uintptr(unsafe.Pointer(name)))
	return hwnd
}
