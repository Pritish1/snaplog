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

// DayGroup represents a group of entries for a specific day
type DayGroup struct {
	DayName string    `json:"day_name"`
	Date    string    `json:"date"`
	Count   int       `json:"count"`
	Entries []LogEntry `json:"entries"`
}

// DashboardData represents the data structure for the dashboard
type DashboardData struct {
	TotalEntries int        `json:"total_entries"`
	TotalDays    int        `json:"total_days"`
	ThisWeek     int        `json:"this_week"`
	Generated    string     `json:"generated"`
	DayGroups    []DayGroup `json:"day_groups"`
}

// DisplayEntry represents a log entry formatted for display
type DisplayEntry struct {
	ID           int             `json:"id"`
	Content      string          `json:"content"`
	RenderedHTML template.HTML   `json:"rendered_html"`
	LocalTime    string          `json:"local_time"`
	CreatedAt    time.Time       `json:"created_at"`
}

// DisplayDayGroup represents a group of display entries for a specific day
type DisplayDayGroup struct {
	DayName string         `json:"day_name"`
	Date    string         `json:"date"`
	Count   int            `json:"count"`
	Entries []DisplayEntry `json:"entries"`
}

// DisplayDashboardData represents the data structure for the dashboard with display formatting
type DisplayDashboardData struct {
	TotalEntries int              `json:"total_entries"`
	TotalDays    int              `json:"total_days"`
	ThisWeek     int              `json:"this_week"`
	Generated    string           `json:"generated"`
	DayGroups    []DisplayDayGroup `json:"day_groups"`
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

// ProcessCommand handles slash commands
func (a *App) ProcessCommand(command string) error {
	fmt.Printf("Processing command: %s\n", command)
	
	switch command {
	case "/dash":
		return a.generateDashboard()
	case "/help":
		return a.showHelp()
	case "/stats":
		return a.showStats()
	default:
		return fmt.Errorf("unknown command: %s. Type /help for available commands", command)
	}
}

// showHelp displays available commands
func (a *App) showHelp() error {
	fmt.Println("\n=== SnapLog CLI Commands ===")
	fmt.Println("\nAvailable Commands:")
	fmt.Println("  /dash  - Generate HTML dashboard and open in browser")
	fmt.Println("  /help  - Show this help message")
	fmt.Println("  /stats - Show database statistics")
	fmt.Println("\nKeyboard Shortcuts:")
	fmt.Println("  Enter        - Log text and hide window")
	fmt.Println("  Shift+Enter  - Create a new line")
	fmt.Println("  Ctrl+Tab     - Toggle Markdown preview")
	fmt.Println("  Esc          - Hide window without saving")
	fmt.Println("\n=============================")
	return nil
}

// showStats displays database statistics
func (a *App) showStats() error {
	count, err := a.GetLogEntriesCount()
	if err != nil {
		return fmt.Errorf("failed to get log count: %v", err)
	}
	
	fmt.Printf("\n=== SnapLog Statistics ===\n")
	fmt.Printf("Total entries: %d\n", count)
	fmt.Printf("Database: %s\n", a.GetDatabasePath())
	fmt.Printf("Hotkey: %v+%v\n", a.settings.HotkeyModifiers, a.settings.HotkeyKey)
	fmt.Printf("========================\n")
	return nil
}

// LogText saves text to the SQLite database with timestamp
func (a *App) LogText(text string) error {
	if text == "" {
		return nil
	}

	fmt.Printf("LogText called with: '%s'\n", text)

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
	
	data, err := a.getDashboardData()
	if err != nil {
		fmt.Printf("Error getting dashboard data: %v\n", err)
		return fmt.Errorf("failed to get dashboard data: %v", err)
	}
	fmt.Println("Dashboard data prepared successfully")

	htmlContent, err := a.generateHTMLFromTemplate(data)
	if err != nil {
		fmt.Printf("Error generating HTML from template: %v\n", err)
		return fmt.Errorf("failed to generate HTML from template: %v", err)
	}
	fmt.Println("HTML content generated from template successfully")

	err = a.saveAndOpenDashboard(htmlContent)
	if err != nil {
		fmt.Printf("Error saving and opening dashboard: %v\n", err)
		return fmt.Errorf("failed to save and open dashboard: %v", err)
	}

	fmt.Println("Dashboard generated and opened successfully!")
	return nil
}

// getDashboardData prepares data for the dashboard with proper formatting
func (a *App) getDashboardData() (*DisplayDashboardData, error) {
	// Get all log entries
	entries, err := a.GetLogEntries(1000)
	if err != nil {
		return nil, fmt.Errorf("failed to get log entries: %v", err)
	}
	
	// Get total count
	totalCount, err := a.GetLogEntriesCount()
	if err != nil {
		return nil, fmt.Errorf("failed to get log count: %v", err)
	}
	
	// Convert entries to display format with local time and rendered markdown
	displayEntries := make([]DisplayEntry, len(entries))
	for i, entry := range entries {
		// Convert UTC to local time
		localTime := entry.CreatedAt.Local()
		
		// Render markdown to HTML
		renderedHTML, err := a.RenderMarkdown(entry.Content)
		if err != nil {
			// If markdown rendering fails, use plain text
			renderedHTML = fmt.Sprintf("<p>%s</p>", strings.ReplaceAll(entry.Content, "\n", "<br>"))
		}
		
		displayEntries[i] = DisplayEntry{
			ID:           entry.ID,
			Content:      entry.Content,
			RenderedHTML: template.HTML(renderedHTML),
			LocalTime:    localTime.Format("15:04"),
			CreatedAt:    entry.CreatedAt,
		}
	}
	
	// Group entries by day using local time
	dayGroups := a.groupDisplayEntriesByDay(displayEntries)
	
	// Calculate statistics
	thisWeek := a.calculateThisWeekCount(entries)
	
	return &DisplayDashboardData{
		TotalEntries: totalCount,
		TotalDays:    len(dayGroups),
		ThisWeek:     thisWeek,
		Generated:    time.Now().Local().Format("2006-01-02 15:04:05"),
		DayGroups:    dayGroups,
	}, nil
}

// groupDisplayEntriesByDay groups display entries by day using local time
func (a *App) groupDisplayEntriesByDay(entries []DisplayEntry) []DisplayDayGroup {
	dayMap := make(map[string][]DisplayEntry)
	
	for _, entry := range entries {
		// Use local time for grouping
		localTime := entry.CreatedAt.Local()
		dayKey := localTime.Format("2006-01-02")
		dayMap[dayKey] = append(dayMap[dayKey], entry)
	}
	
	var dayGroups []DisplayDayGroup
	for _, dayEntries := range dayMap {
		// Parse the date using local time
		localTime := dayEntries[0].CreatedAt.Local()
	
		
		dayGroup := DisplayDayGroup{
			DayName: localTime.Format("Monday"),
			Date:    localTime.Format("Jan 2, 2006"),
			Count:   len(dayEntries),
			Entries: dayEntries,
		}
		dayGroups = append(dayGroups, dayGroup)
	}
	
	// Sort by date (newest first)
	for i := 0; i < len(dayGroups); i++ {
		for j := i + 1; j < len(dayGroups); j++ {
			date1, _ := time.Parse("Jan 2, 2006", dayGroups[i].Date)
			date2, _ := time.Parse("Jan 2, 2006", dayGroups[j].Date)
			if date1.Before(date2) {
				dayGroups[i], dayGroups[j] = dayGroups[j], dayGroups[i]
			}
		}
	}
	
	return dayGroups
}

// calculateThisWeekCount counts entries from this week
func (a *App) calculateThisWeekCount(entries []LogEntry) int {
	now := time.Now()
	weekStart := now.AddDate(0, 0, -int(now.Weekday()))
	weekStart = time.Date(weekStart.Year(), weekStart.Month(), weekStart.Day(), 0, 0, 0, 0, weekStart.Location())
	
	count := 0
	for _, entry := range entries {
		if entry.CreatedAt.After(weekStart) {
			count++
		}
	}
	return count
}

// generateHTMLFromTemplate generates HTML from the template file
func (a *App) generateHTMLFromTemplate(data *DisplayDashboardData) (string, error) {
	// Read template file
	templatePath := filepath.Join(".", "templates", "dashboard.html")
	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("failed to read template file: %v", err)
	}
	
	// Parse template
	tmpl, err := template.New("dashboard").Parse(string(templateContent))
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %v", err)
	}
	
	// Execute template
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		return "", fmt.Errorf("failed to execute template: %v", err)
	}
	
	return buf.String(), nil
}

// saveAndOpenDashboard saves HTML content and opens it in browser
func (a *App) saveAndOpenDashboard(htmlContent string) error {
	// Get temp directory path
	tempPath, err := a.getTempPath()
	if err != nil {
		fmt.Printf("Error getting temp path: %v\n", err)
		return fmt.Errorf("failed to get temp path: %v", err)
	}
	fmt.Printf("Temp path: %s\n", tempPath)
	
	// Write HTML file
	htmlFile := filepath.Join(tempPath, "snaplog-dashboard.html")
	fmt.Printf("Writing HTML file to: %s\n", htmlFile)
	err = os.WriteFile(htmlFile, []byte(htmlContent), 0644)
	if err != nil {
		fmt.Printf("Error writing HTML file: %v\n", err)
		return fmt.Errorf("failed to write HTML file: %v", err)
	}
	fmt.Printf("HTML file written successfully\n")
	
	// Open in browser
	fmt.Printf("Opening browser with file: %s\n", htmlFile)
	err = a.openInBrowser(htmlFile)
	if err != nil {
		fmt.Printf("Error opening browser: %v\n", err)
		return fmt.Errorf("failed to open browser: %v", err)
	}
	fmt.Printf("Browser opened successfully\n")
	
	fmt.Printf("Dashboard generated: %s\n", htmlFile)
	return nil
}

// getTempPath returns a temporary directory path for the HTML file
func (a *App) getTempPath() (string, error) {
	// Use the system temp directory
	tempDir := os.TempDir()
	if tempDir == "" {
		return "", fmt.Errorf("could not get temp directory")
	}
	
	// Create snaplog temp directory
	snaplogTempDir := filepath.Join(tempDir, "snaplog-dashboards")
	if err := os.MkdirAll(snaplogTempDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp dashboard directory: %v", err)
	}
	
	return snaplogTempDir, nil
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
	settings := systray.AddMenuItem("Settings...", "Configure hotkey")
	systray.AddSeparator()
	instructions := systray.AddMenuItem("Instructions", "Keyboard shortcuts and usage")
	systray.AddSeparator()
	quit := systray.AddMenuItem("Quit", "Exit SnapLog")
	
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
	fmt.Println("\n=== SnapLog CLI Instructions ===")
	fmt.Println("\nKeyboard Shortcuts:")
	fmt.Println("  Enter        - Log text and hide window")
	fmt.Println("  Shift+Enter  - Create a new line")
	fmt.Println("  Ctrl+Tab     - Toggle Markdown preview")
	fmt.Println("  Esc          - Hide window without saving")
	fmt.Println("\nCommands:")
	fmt.Println("  /dash        - Generate HTML dashboard and open in browser")
	fmt.Println("\nMarkdown Support:")
	fmt.Println("  # Header     - Create headers")
	fmt.Println("  **bold**     - Bold text")
	fmt.Println("  *italic*     - Italic text")
	fmt.Println("  `code`       - Inline code")
	fmt.Println("  - list       - Bullet lists")
	fmt.Println("\nSystem Tray:")
	fmt.Println("  Right-click tray icon for menu options")
	fmt.Printf("  Current hotkey: %v+%v\n", a.settings.HotkeyModifiers, a.settings.HotkeyKey)
	fmt.Println("\n===============================")
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