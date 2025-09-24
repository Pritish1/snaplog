package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx     context.Context
	hotkeyId uintptr
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
	// Hotkey cleanup is handled in the hotkey detection goroutine
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
