//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

const (
	MOD_CONTROL = 0x0002
	MOD_SHIFT   = 0x0004
	VK_L        = 0x4C
	WM_HOTKEY   = 0x0312
)

var (
	user32                = syscall.NewLazyDLL("user32.dll")
	procRegisterHotKey    = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey  = user32.NewProc("UnregisterHotKey")
	procGetMessage        = user32.NewProc("GetMessageW")
	procTranslateMessage  = user32.NewProc("TranslateMessage")
	procDispatchMessage   = user32.NewProc("DispatchMessageW")
	procPostQuitMessage   = user32.NewProc("PostQuitMessage")
	procGetCurrentThreadId = user32.NewProc("GetCurrentThreadId")
)

type MSG struct {
	HWND   uintptr
	UINT   uint32
	WPARAM uintptr
	LPARAM uintptr
	DWORD  uint32
	POINT  struct {
		X, Y int32
	}
}

// startHotkeyDetection registers a global hotkey and starts listening for it
func (a *App) startHotkeyDetection() {
	// Register Ctrl+Shift+L hotkey
	ret, _, err := procRegisterHotKey.Call(
		0,                    // HWND (0 for global hotkey)
		a.hotkeyId,           // ID
		MOD_CONTROL|MOD_SHIFT, // Modifiers (Ctrl+Shift)
		VK_L,                 // Virtual key code for 'L'
	)
	
	if ret == 0 {
		fmt.Printf("Failed to register hotkey: %v\n", err)
		return
	}
	
	fmt.Println("Hotkey registered: Ctrl+Shift+L")
	
	// Message loop to listen for hotkey events
	var msg MSG
	for {
		ret, _, _ := procGetMessage.Call(
			uintptr(unsafe.Pointer(&msg)),
			0, 0, 0,
		)
		
		if ret == 0 { // WM_QUIT
			break
		}
		
		if ret == ^uintptr(0) { // Error
			fmt.Println("GetMessage error")
			break
		}
		
		if msg.UINT == WM_HOTKEY && msg.WPARAM == a.hotkeyId {
			// Hotkey pressed! Show the window
			fmt.Println("Hotkey detected! Showing window...")
			a.ShowWindow()
		}
		
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
	}
	
	// Cleanup: unregister hotkey
	procUnregisterHotKey.Call(0, a.hotkeyId)
	fmt.Println("Hotkey unregistered")
}
