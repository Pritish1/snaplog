package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.design/x/hotkey"
)

// App struct
type App struct {
	ctx          context.Context
	hotkeyId     uintptr
	packageHotkey *hotkey.Hotkey
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.hotkeyId = uintptr(1) // Initialize hotkey ID
	// Start hotkey detection in a goroutine
	go a.startHotkeyDetection()
}

// shutdown is called when the app is shutting down
func (a *App) shutdown(ctx context.Context) {
	fmt.Println("Shutting down SnapLog...")
	// Stop hotkey detection if running
	a.stopHotkeyDetection()
}

// LogText appends text to the log file with timestamp
func (a *App) LogText(text string) error {
	if text == "" {
		return nil
	}

	// Create logs directory if it doesn't exist
	logsDir := "logs"
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %v", err)
	}

	// Create log file path
	logFile := filepath.Join(logsDir, "snaplog.txt")

	// Prepare log entry with timestamp
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logEntry := fmt.Sprintf("[%s] %s\n", timestamp, text)

	// Append to file
	file, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %v", err)
	}
	defer file.Close()

	if _, err := file.WriteString(logEntry); err != nil {
		return fmt.Errorf("failed to write to log file: %v", err)
	}

	return nil
}

// ShowWindow brings the app window to the front
func (a *App) ShowWindow() {
	runtime.WindowShow(a.ctx)
	runtime.WindowUnminimise(a.ctx)
}

// HideWindow hides the app window
func (a *App) HideWindow() {
	runtime.WindowHide(a.ctx)
}

// GetLogFilePath returns the current log file path
func (a *App) GetLogFilePath() string {
	return filepath.Join("logs", "snaplog.txt")
}

// Quit closes the application
func (a *App) Quit() {
	fmt.Println("Quitting SnapLog...")
	runtime.Quit(a.ctx)
}

// startHotkeyDetection registers a global hotkey using golang.design/x/hotkey
func (a *App) startHotkeyDetection() {
	fmt.Println("Starting hotkey detection...")
	
	// Register Ctrl+Shift+L (Windows/Linux) or Cmd+Shift+L (macOS)
	hk := hotkey.New([]hotkey.Modifier{hotkey.ModCtrl, hotkey.ModShift}, hotkey.KeyL)
	
	err := hk.Register()
	if err != nil {
		fmt.Printf("Failed to register hotkey: %v\n", err)
		fmt.Println("Note: On macOS, this requires accessibility permissions.")
		fmt.Println("Go to System Preferences > Security & Privacy > Privacy > Accessibility")
		fmt.Println("On Linux, you may need to install additional packages or configure X11.")
		return
	}
	
	fmt.Println("Hotkey registered: Ctrl+Shift+L (or Cmd+Shift+L on macOS)")
	
	// Store the hotkey for cleanup
	a.packageHotkey = hk
	
	// Listen for hotkey events in a goroutine
	go func() {
		for {
			select {
			case <-hk.Keydown():
				fmt.Println("Hotkey detected! Showing window...")
				a.ShowWindow()
			}
		}
	}()
}

// stopHotkeyDetection stops the hotkey detection
func (a *App) stopHotkeyDetection() {
	if a.packageHotkey != nil {
		fmt.Println("Unregistering hotkey...")
		a.packageHotkey.Unregister()
		a.packageHotkey = nil
	}
}

// CheckAccessibilityPermissions returns true if accessibility permissions are granted (macOS)
func (a *App) CheckAccessibilityPermissions() bool {
	// This is a simple check - if we can register a hotkey, permissions are granted
	if a.packageHotkey != nil {
		return true
	}
	return false
}

// RequestAccessibilityPermissions prompts the user to grant accessibility permissions (macOS)
func (a *App) RequestAccessibilityPermissions() {
	fmt.Println("SnapLog requires accessibility permissions to register global hotkeys.")
	fmt.Println("Please grant accessibility permissions in System Preferences > Security & Privacy > Privacy > Accessibility")
	fmt.Println("After granting permissions, restart SnapLog.")
}
