//go:build !windows

package main

import "fmt"

// startHotkeyDetection is a stub for non-Windows platforms
func (a *App) startHotkeyDetection() {
	fmt.Println("SnapLog is running in background mode")
	fmt.Println("Note: Global hotkey detection (Ctrl+Shift+L) is currently only supported on Windows")
	fmt.Println("On macOS/Linux, you can:")
	fmt.Println("1. Use the Dock/System Tray to show the window")
	fmt.Println("2. Use Spotlight (Cmd+Space) to search for 'SnapLog'")
	fmt.Println("3. The app will start hidden - look for it in your applications")
}
