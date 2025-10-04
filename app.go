package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/getlantern/systray"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.design/x/hotkey"
	"github.com/yuin/goldmark"
	_ "modernc.org/sqlite"
)

// Settings represents the application configuration
type Settings struct {
	HotkeyModifiers []string `json:"hotkey_modifiers"`
	HotkeyKey       string   `json:"hotkey_key"`
}

// LogEntry represents a log entry in the database
type LogEntry struct {
	ID        int       `json:"id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// App struct
type App struct {
	ctx          context.Context
	hotkeyId     uintptr
	packageHotkey *hotkey.Hotkey
	settings     *Settings
	systrayReady chan bool
	db           *sql.DB
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		settings: &Settings{
			HotkeyModifiers: []string{"ctrl", "shift"},
			HotkeyKey:       "l",
		},
		systrayReady: make(chan bool),
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.hotkeyId = uintptr(1) // Initialize hotkey ID
	
	// Load settings from file
	a.loadSettings()
	
	// Initialize database
	if err := a.initDatabase(); err != nil {
		fmt.Printf("Failed to initialize database: %v\n", err)
		return
	}
	
	// Start system tray in a goroutine
	go a.runSystemTray()
	
	// Wait for system tray to be ready
	<-a.systrayReady
	
	// Start hotkey detection in a goroutine
	go a.startHotkeyDetection()
}

// shutdown is called when the app is shutting down
func (a *App) shutdown(ctx context.Context) {
	fmt.Println("Shutting down SnapLog...")
	// Stop hotkey detection if running
	a.stopHotkeyDetection()
	
	// Close database connection
	if a.db != nil {
		a.db.Close()
		fmt.Println("Database connection closed")
	}
}

// initDatabase initializes the SQLite database
func (a *App) initDatabase() error {
	// Get user config directory
	configDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get config directory: %v", err)
	}
	
	// Create snaplog directory if it doesn't exist
	snaplogDir := filepath.Join(configDir, "snaplog")
	if err := os.MkdirAll(snaplogDir, 0755); err != nil {
		return fmt.Errorf("failed to create snaplog directory: %v", err)
	}
	
	// Database file path
	dbPath := filepath.Join(snaplogDir, "snaplog.db")
	
	// Open database connection
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %v", err)
	}
	
	// Test connection
	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %v", err)
	}
	
	a.db = db
	
	// Create tables
	if err := a.createTables(); err != nil {
		return fmt.Errorf("failed to create tables: %v", err)
	}
	
	fmt.Printf("Database initialized at: %s\n", dbPath)
	return nil
}

// createTables creates the necessary database tables
func (a *App) createTables() error {
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS log_entries (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		content TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	
	_, err := a.db.Exec(createTableSQL)
	return err
}

// LogText saves text to the SQLite database with timestamp
func (a *App) LogText(text string) error {
	if text == "" {
		return nil
	}

	fmt.Printf("LogText called with: '%s'\n", text)

	// Check for special commands
	if strings.TrimSpace(text) == "/dash" {
		fmt.Println("Detected /dash command, generating dashboard...")
		return a.generateDashboard()
	}

	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	// Insert text into database
	query := `INSERT INTO log_entries (content) VALUES (?)`
	_, err := a.db.Exec(query, text)
	if err != nil {
		return fmt.Errorf("failed to insert log entry: %v", err)
	}

	fmt.Printf("Logged text: %s\n", text)
	return nil
}

// generateDashboard creates an HTML dashboard with all log entries
func (a *App) generateDashboard() error {
	fmt.Println("Starting dashboard generation...")
	
	// Get all log entries
	entries, err := a.GetLogEntries(1000) // Get up to 1000 entries
	if err != nil {
		fmt.Printf("Error getting log entries: %v\n", err)
		return fmt.Errorf("failed to get log entries: %v", err)
	}
	fmt.Printf("Retrieved %d log entries\n", len(entries))

	// Get log count
	count, err := a.GetLogEntriesCount()
	if err != nil {
		fmt.Printf("Error getting log count: %v\n", err)
		return fmt.Errorf("failed to get log count: %v", err)
	}
	fmt.Printf("Total log count: %d\n", count)

	// Generate HTML
	htmlContent, err := a.generateHTML(entries, count)
	if err != nil {
		fmt.Printf("Error generating HTML: %v\n", err)
		return fmt.Errorf("failed to generate HTML: %v", err)
	}
	fmt.Println("HTML content generated successfully")

	// Get temp directory path
	tempPath, err := a.getTempPath()
	if err != nil {
		fmt.Printf("Error getting temp path: %v\n", err)
		return fmt.Errorf("failed to get temp path: %v", err)
	}
	fmt.Printf("Temp path: %s\n", tempPath)

	// Write HTML file
	htmlFile := filepath.Join(tempPath, "snaplog-dashboard.html")
	err = os.WriteFile(htmlFile, []byte(htmlContent), 0644)
	if err != nil {
		fmt.Printf("Error writing HTML file: %v\n", err)
		return fmt.Errorf("failed to write HTML file: %v", err)
	}
	fmt.Printf("HTML file written to: %s\n", htmlFile)

	// Open in browser
	err = a.openInBrowser(htmlFile)
	if err != nil {
		fmt.Printf("Error opening browser: %v\n", err)
		return fmt.Errorf("failed to open browser: %v", err)
	}
	fmt.Println("Browser opened successfully")

	fmt.Printf("Dashboard generated: %s\n", htmlFile)
	return nil
}

// generateHTML creates the HTML content for the dashboard
func (a *App) generateHTML(entries []LogEntry, count int) (string, error) {
	const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>SnapLog Dashboard</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Roboto', sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
            padding: 20px;
        }
        
        .container {
            max-width: 1200px;
            margin: 0 auto;
            background: white;
            border-radius: 12px;
            box-shadow: 0 20px 40px rgba(0,0,0,0.1);
            overflow: hidden;
        }
        
        .header {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            padding: 30px;
            text-align: center;
        }
        
        .header h1 {
            font-size: 2.5rem;
            margin-bottom: 10px;
            font-weight: 300;
        }
        
        .header p {
            font-size: 1.1rem;
            opacity: 0.9;
        }
        
        .stats {
            display: flex;
            justify-content: center;
            gap: 30px;
            padding: 20px;
            background: #f8f9fa;
            border-bottom: 1px solid #e0e0e0;
        }
        
        .stat-item {
            text-align: center;
        }
        
        .stat-number {
            font-size: 2rem;
            font-weight: bold;
            color: #667eea;
        }
        
        .stat-label {
            color: #666;
            font-size: 0.9rem;
        }
        
        .entries-container {
            padding: 30px;
        }
        
        .entry {
            background: #f8f9fa;
            border-radius: 8px;
            padding: 20px;
            margin-bottom: 15px;
            border-left: 4px solid #667eea;
            transition: transform 0.2s ease;
        }
        
        .entry:hover {
            transform: translateY(-2px);
            box-shadow: 0 4px 12px rgba(0,0,0,0.1);
        }
        
        .entry-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 10px;
        }
        
        .entry-id {
            background: #667eea;
            color: white;
            padding: 4px 12px;
            border-radius: 20px;
            font-size: 0.8rem;
            font-weight: 500;
        }
        
        .entry-date {
            color: #666;
            font-size: 0.9rem;
        }
        
        .entry-content {
            color: #333;
            line-height: 1.6;
            white-space: pre-wrap;
            word-break: break-word;
        }
        
        .no-entries {
            text-align: center;
            padding: 60px;
            color: #666;
            font-size: 1.1rem;
        }
        
        .footer {
            text-align: center;
            padding: 20px;
            background: #f8f9fa;
            color: #666;
            font-size: 0.9rem;
        }
        
        @media (max-width: 768px) {
            .stats {
                flex-direction: column;
                gap: 15px;
            }
            
            .entry-header {
                flex-direction: column;
                align-items: flex-start;
                gap: 5px;
            }
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>SnapLog Dashboard</h1>
            <p>Your personal text logging dashboard</p>
        </div>
        
        <div class="stats">
            <div class="stat-item">
                <div class="stat-number">{{.Count}}</div>
                <div class="stat-label">Total Entries</div>
            </div>
            <div class="stat-item">
                <div class="stat-number">{{.Displayed}}</div>
                <div class="stat-label">Displayed</div>
            </div>
            <div class="stat-item">
                <div class="stat-number">{{.Generated}}</div>
                <div class="stat-label">Generated</div>
            </div>
        </div>
        
        <div class="entries-container">
            {{if .Entries}}
                {{range .Entries}}
                <div class="entry">
                    <div class="entry-header">
                        <span class="entry-id">#{{.ID}}</span>
                        <span class="entry-date">{{.CreatedAt}}</span>
                    </div>
                    <div class="entry-content">{{.Content}}</div>
                </div>
                {{end}}
            {{else}}
                <div class="no-entries">
                    <p>No log entries found.</p>
                </div>
            {{end}}
        </div>
        
        <div class="footer">
            <p>Generated on {{.Generated}} | SnapLog Dashboard</p>
        </div>
    </div>
</body>
</html>`

	// Prepare template data
	data := struct {
		Entries    []LogEntry
		Count      int
		Displayed  int
		Generated  string
	}{
		Entries:   entries,
		Count:     count,
		Displayed: len(entries),
		Generated: time.Now().Format("2006-01-02 15:04:05"),
	}

	// Parse and execute template
	tmpl, err := template.New("dashboard").Parse(htmlTemplate)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

// getTempPath returns a temporary directory path for the HTML file
func (a *App) getTempPath() (string, error) {
	// Use the system temp directory
	tempDir := os.TempDir()
	if tempDir == "" {
		return "", fmt.Errorf("could not get temp directory")
	}
	return tempDir, nil
}

// openInBrowser opens the HTML file in the default browser
func (a *App) openInBrowser(filePath string) error {
	var cmd *exec.Cmd
	
	// Determine the operating system and use appropriate command
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", filePath)
	case "darwin":
		cmd = exec.Command("open", filePath)
	case "linux":
		cmd = exec.Command("xdg-open", filePath)
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
	
	return cmd.Start()
}


func (a *App) GetLogEntries(limit int) ([]LogEntry, error) {
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := `SELECT id, content, created_at FROM log_entries ORDER BY created_at DESC LIMIT ?`
	rows, err := a.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query log entries: %v", err)
	}
	defer rows.Close()

	var entries []LogEntry
	for rows.Next() {
		var entry LogEntry
		err := rows.Scan(&entry.ID, &entry.Content, &entry.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan log entry: %v", err)
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// GetLogEntriesCount returns the total number of log entries
func (a *App) GetLogEntriesCount() (int, error) {
	if a.db == nil {
		return 0, fmt.Errorf("database not initialized")
	}

	var count int
	query := `SELECT COUNT(*) FROM log_entries`
	err := a.db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count log entries: %v", err)
	}

	return count, nil
}

// SearchLogEntries searches for log entries containing the given text
func (a *App) SearchLogEntries(searchText string, limit int) ([]LogEntry, error) {
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := `SELECT id, content, created_at FROM log_entries WHERE content LIKE ? ORDER BY created_at DESC LIMIT ?`
	searchPattern := "%" + searchText + "%"
	rows, err := a.db.Query(query, searchPattern, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search log entries: %v", err)
	}
	defer rows.Close()

	var entries []LogEntry
	for rows.Next() {
		var entry LogEntry
		err := rows.Scan(&entry.ID, &entry.Content, &entry.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan log entry: %v", err)
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// RenderMarkdown converts Markdown text to HTML
func (a *App) RenderMarkdown(markdown string) (string, error) {
	var buf bytes.Buffer
	md := goldmark.New()
	if err := md.Convert([]byte(markdown), &buf); err != nil {
		return "", fmt.Errorf("failed to render markdown: %v", err)
	}
	return buf.String(), nil
}

// ShowWindow brings the app window to the front
func (a *App) ShowWindow() {
	wailsRuntime.WindowShow(a.ctx)
	wailsRuntime.WindowUnminimise(a.ctx)
}

// HideWindow hides the app window
func (a *App) HideWindow() {
	wailsRuntime.WindowHide(a.ctx)
}

// GetDatabasePath returns the current database file path
func (a *App) GetDatabasePath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "unknown"
	}
	snaplogDir := filepath.Join(configDir, "snaplog")
	return filepath.Join(snaplogDir, "snaplog.db")
}

// Quit closes the application
func (a *App) Quit() {
	fmt.Println("Quitting SnapLog...")
	wailsRuntime.Quit(a.ctx)
}

// startHotkeyDetection registers a global hotkey using golang.design/x/hotkey
func (a *App) startHotkeyDetection() {
	fmt.Println("Starting hotkey detection...")
	
	// Parse modifiers from settings
	var modifiers []hotkey.Modifier
	for _, mod := range a.settings.HotkeyModifiers {
		switch mod {
		case "ctrl":
			modifiers = append(modifiers, hotkey.ModCtrl)
		case "cmd", "meta":
			modifiers = append(modifiers, hotkey.ModCtrl) // Use Ctrl as fallback for Cmd
		case "alt":
			modifiers = append(modifiers, hotkey.ModAlt)
		case "shift":
			modifiers = append(modifiers, hotkey.ModShift)
		}
	}
	
	// Parse key from settings
	var key hotkey.Key
	switch a.settings.HotkeyKey {
	case "l":
		key = hotkey.KeyL
	case "s":
		key = hotkey.KeyS
	case "t":
		key = hotkey.KeyT
	case "n":
		key = hotkey.KeyN
	case "space":
		key = hotkey.KeySpace
	default:
		key = hotkey.KeyL // Default to L
	}
	
	// Register hotkey
	hk := hotkey.New(modifiers, key)
	
	err := hk.Register()
	if err != nil {
		fmt.Printf("Failed to register hotkey: %v\n", err)
		fmt.Println("Note: On macOS, this requires accessibility permissions.")
		fmt.Println("Go to System Preferences > Security & Privacy > Privacy > Accessibility")
		fmt.Println("On Linux, you may need to install additional packages or configure X11.")
		return
	}
	
	fmt.Printf("Hotkey registered: %v+%v\n", a.settings.HotkeyModifiers, a.settings.HotkeyKey)
	
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

// setupSystemMenu creates a simple menu (placeholder for now)
func (a *App) setupSystemMenu() {
	// For now, we'll use a simple approach
	// In Wails v2, system tray might not be fully supported
	// We'll implement a settings button in the main UI instead
	fmt.Println("System menu setup (placeholder)")
}

// runSystemTray initializes and runs the system tray
func (a *App) runSystemTray() {
	systray.Run(a.onSystemTrayReady, a.onSystemTrayExit)
}

// onSystemTrayReady is called when the system tray is ready
func (a *App) onSystemTrayReady() {
	// Try to read the appicon.png file
	iconData, err := os.ReadFile("build/appicon.png")
	if err != nil {
		fmt.Printf("Warning: Could not read appicon.png: %v\n", err)
		// Fallback to empty icon if file not found
		iconData = []byte{}
	}
	
	systray.SetIcon(iconData)
	systray.SetTitle("SnapLog")
	systray.SetTooltip("SnapLog - Hotkey Text Logger")
	
	// Create menu items
	showWindow := systray.AddMenuItem("Show Window", "Show the main window")
	systray.AddSeparator()
	settings := systray.AddMenuItem("Settings...", "Open settings")
	systray.AddSeparator()
	instructions := systray.AddMenuItem("Instructions", "Show keyboard shortcuts")
	systray.AddSeparator()
	quit := systray.AddMenuItem("Quit", "Quit SnapLog")
	
	// Signal that system tray is ready
	a.systrayReady <- true
	
	// Handle menu clicks
	go func() {
		for {
			select {
			case <-showWindow.ClickedCh:
				a.ShowWindow()
			case <-settings.ClickedCh:
				a.OpenSettings()
			case <-instructions.ClickedCh:
				a.ShowInstructions()
			case <-quit.ClickedCh:
				a.Quit()
			}
		}
	}()
}

// onSystemTrayExit is called when the system tray exits
func (a *App) onSystemTrayExit() {
	fmt.Println("System tray exiting...")
}

// OpenSettings opens the settings modal
func (a *App) OpenSettings() {
	// Show the main window first
	a.ShowWindow()
	// Then emit the event to open settings modal
	wailsRuntime.EventsEmit(a.ctx, "open-settings")
}

// ShowInstructions shows keyboard shortcuts and usage instructions
func (a *App) ShowInstructions() {
	// Show the main window first
	a.ShowWindow()
	// Then emit the event to show instructions
	wailsRuntime.EventsEmit(a.ctx, "show-instructions")
}

// GetSettings returns the current settings
func (a *App) GetSettings() *Settings {
	return a.settings
}

// SetSettings updates the settings and saves them
func (a *App) SetSettings(settings *Settings) error {
	a.settings = settings
	
	// Save settings to file
	if err := a.saveSettings(); err != nil {
		return fmt.Errorf("failed to save settings: %v", err)
	}
	
	// Restart hotkey detection with new settings
	a.stopHotkeyDetection()
	go a.startHotkeyDetection()
	
	return nil
}

// loadSettings loads settings from file
func (a *App) loadSettings() {
	configDir, err := os.UserConfigDir()
	if err != nil {
		fmt.Printf("Failed to get config directory: %v\n", err)
		return
	}
	
	settingsFile := filepath.Join(configDir, "snaplog", "settings.json")
	
	data, err := os.ReadFile(settingsFile)
	if err != nil {
		// File doesn't exist or can't be read, use defaults
		fmt.Println("Using default settings")
		return
	}
	
	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		fmt.Printf("Failed to parse settings: %v\n", err)
		return
	}
	
	a.settings = &settings
	fmt.Println("Settings loaded successfully")
}

// saveSettings saves settings to file
func (a *App) saveSettings() error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get config directory: %v", err)
	}
	
	// Create snaplog directory if it doesn't exist
	snaplogDir := filepath.Join(configDir, "snaplog")
	if err := os.MkdirAll(snaplogDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}
	
	settingsFile := filepath.Join(snaplogDir, "settings.json")
	
	data, err := json.MarshalIndent(a.settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %v", err)
	}
	
	if err := os.WriteFile(settingsFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write settings file: %v", err)
	}
	
	fmt.Println("Settings saved successfully")
	return nil
}
