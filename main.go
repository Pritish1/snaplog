package main

import (
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed templates
var templates embed.FS

var lockFile *os.File

func main() {
	// Check for existing instance
	if !acquireLock() {
		// Another instance is running - show notification and exit
		showAlreadyRunningNotification()
		os.Exit(0) // Exit gracefully
	}
	defer releaseLock()

	// Create an instance of the app structure
	app := NewApp()

	// Create application with options
	err := wails.Run(&options.App{
		Title:  "SnapLog CLI",
		Width:  800,
		Height: 300,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
		// Keep the app running in background even when window is closed
		DisableResize: true,
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

// acquireLock attempts to acquire a lock file to prevent multiple instances
func acquireLock() bool {
	configDir, err := os.UserConfigDir()
	if err != nil {
		fmt.Printf("Failed to get config directory: %v\n", err)
		return false
	}

	snaplogDir := filepath.Join(configDir, "snaplog")
	if err := os.MkdirAll(snaplogDir, 0755); err != nil {
		fmt.Printf("Failed to create snaplog directory: %v\n", err)
		return false
	}

	lockFilePath := filepath.Join(snaplogDir, "snaplog.lock")

	// Check if lock file exists
	if _, err := os.Stat(lockFilePath); err == nil {
		// Lock file exists, check if the process is still running
		pidData, err := os.ReadFile(lockFilePath)
		if err == nil {
			pidStr := strings.TrimSpace(string(pidData))
			if pid, err := strconv.Atoi(pidStr); err == nil {
				if isProcessRunning(pid) {
					// Another instance is running
					return false
				}
				// Process is not running, remove stale lock file
				os.Remove(lockFilePath)
			}
		}
	}

	// Try to create the lock file
	lockFile, err = os.OpenFile(lockFilePath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0644)
	if err != nil {
		// Could not create lock file (might be locked by another process)
		return false
	}

	// Write the current process PID to the lock file
	pid := os.Getpid()
	_, err = lockFile.WriteString(fmt.Sprintf("%d\n", pid))
	if err != nil {
		lockFile.Close()
		os.Remove(lockFilePath)
		return false
	}

	// Keep the file open to maintain the lock
	// On Windows, this prevents other processes from opening it
	// On Unix, O_EXCL already ensured atomic creation, but keeping it open is safer
	return true
}

// isProcessRunning checks if a process with the given PID is running
func isProcessRunning(pid int) bool {
	if runtime.GOOS == "windows" {
		// On Windows, try to open the process handle
		// If it fails, the process doesn't exist
		proc, err := os.FindProcess(pid)
		if err != nil {
			return false
		}
		// On Windows, we can't easily check if process exists without signaling
		// So we'll assume if we can find it, it might be running
		// The file lock mechanism will handle the actual prevention
		_ = proc
		return true // Assume running if we can find the process
	} else if runtime.GOOS == "darwin" {
		// On macOS, check if process exists using ps command
		// This is more reliable than /proc which doesn't exist on macOS
		cmd := fmt.Sprintf("ps -p %d > /dev/null 2>&1", pid)
		err := exec.Command("sh", "-c", cmd).Run()
		return err == nil
	} else {
		// On Linux/Unix, check if /proc/PID exists
		procPath := fmt.Sprintf("/proc/%d", pid)
		_, err := os.Stat(procPath)
		return err == nil
	}
}

// showAlreadyRunningNotification shows a system notification that the app is already running
func showAlreadyRunningNotification() {
	message := "SnapLog is already running"
	title := "SnapLog"
	
	switch runtime.GOOS {
	case "windows":
		// Use Windows msg.exe (built-in) or PowerShell as fallback
		// msg.exe requires a session name, so we'll use PowerShell which is more reliable
		escapedMsg := strings.ReplaceAll(message, "'", "''")
		cmd := exec.Command("powershell", "-Command", 
			fmt.Sprintf("Add-Type -AssemblyName PresentationFramework; [System.Windows.MessageBox]::Show('%s', '%s', 'OK', 'Information')", 
				escapedMsg, title))
		if err := cmd.Run(); err != nil {
			// Fallback: just print to console
			fmt.Println(message)
		}
	case "darwin":
		// Use macOS notification
		// Escape quotes in the message
		escapedMsg := strings.ReplaceAll(message, "\"", "\\\"")
		cmd := exec.Command("osascript", "-e", 
			fmt.Sprintf(`display notification "%s" with title "%s"`, escapedMsg, title))
		cmd.Run()
	case "linux":
		// Try to use notify-send (common on Linux)
		cmd := exec.Command("notify-send", title, message)
		if cmd.Run() != nil {
			// Fallback to zenity if notify-send is not available
			cmd = exec.Command("zenity", "--info", "--text", message, "--title", title)
			cmd.Run()
		}
	default:
		// Fallback: just print to console
		fmt.Println(message)
	}
}

// releaseLock releases the lock file
func releaseLock() {
	if lockFile != nil {
		lockFile.Close()
		lockFile = nil
	}
	configDir, err := os.UserConfigDir()
	if err == nil {
		lockFilePath := filepath.Join(configDir, "snaplog", "snaplog.lock")
		os.Remove(lockFilePath)
	}
}
