package main

import (
	"bytes"
	"context"
	"database/sql"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.design/x/hotkey"
	"github.com/yuin/goldmark"
	_ "modernc.org/sqlite"
)

//go:embed assets/icons/appicon.png
var appIcon []byte

//go:embed assets/icons/icon.ico
var appIconWindows []byte

// Settings represents the application configuration
type Settings struct {
	HotkeyModifiers []string `json:"hotkey_modifiers"`
	HotkeyKey       string   `json:"hotkey_key"`
	FirstRun        bool     `json:"first_run"`
	Theme           string   `json:"theme"` // "light" or "dark"
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
	DateString   string          `json:"date_string"`
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
	Tags         []Tag             `json:"tags"`
	LogoData     template.URL     `json:"logo_data"`
	OriginalJSONRaw template.JS   `json:"original_json_raw"`
}



// App struct
type App struct {
	ctx          context.Context
	hotkeyId     uintptr
	packageHotkey *hotkey.Hotkey
	settings     *Settings
	db           *sql.DB
	logFile      *os.File
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		settings: &Settings{
			HotkeyModifiers: []string{"ctrl", "shift"},
			HotkeyKey:       "l",
			FirstRun:        true,
			Theme:           "dark",
		},
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.hotkeyId = uintptr(1) // Initialize hotkey ID
	
	// Initialize logging
	if err := a.initLogging(); err != nil {
		fmt.Printf("Warning: Failed to initialize logging: %v\n", err)
	}
	
	// Load settings from file
	a.loadSettings()
	
	// Initialize database
	if err := a.initDatabase(); err != nil {
		a.logf("Failed to initialize database: %v\n", err)
		return
	}
	
	// Check if this is the first run
	if a.settings.FirstRun {
		a.logf("First run detected - showing setup window\n")
		// Show window for first-time setup and emit event to open settings
		go func() {
			time.Sleep(500 * time.Millisecond) // Wait for window to be ready
			a.ShowWindow()
			wailsRuntime.EventsEmit(a.ctx, "show-first-run-setup")
		}()
	} else {
		// For subsequent runs, just start hotkey detection
		// Window will be visible by default (StartHidden removed)
		go a.startHotkeyDetection()
	}
}

// shutdown is called when the app is shutting down
func (a *App) shutdown(ctx context.Context) {
	a.logf("Shutting down SnapLog...\n")
	// Stop hotkey detection if running
	a.stopHotkeyDetection()
	
	// Close database connection
	if a.db != nil {
		a.db.Close()
		a.logf("Database connection closed\n")
	}
	
	// Close log file
	if a.logFile != nil {
		a.logFile.Close()
		a.logFile = nil
	}
}

// initLogging initializes file-based logging
func (a *App) initLogging() error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get config directory: %v", err)
	}
	
	snaplogDir := filepath.Join(configDir, "snaplog")
	if err := os.MkdirAll(snaplogDir, 0755); err != nil {
		return fmt.Errorf("failed to create snaplog directory: %v", err)
	}
	
	// Create log file with date in name (one per day)
	logFileName := fmt.Sprintf("snaplog-%s.log", time.Now().Format("2006-01-02"))
	logFilePath := filepath.Join(snaplogDir, logFileName)
	
	// Open log file in append mode
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %v", err)
	}
	
	a.logFile = logFile
	
	// Also write to stdout for development
	fmt.Printf("Logging to: %s\n", logFilePath)
	
	return nil
}

// logf writes a formatted message to both log file and stdout
func (a *App) logf(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logMessage := fmt.Sprintf("[%s] %s", timestamp, message)
	
	// Write to log file
	if a.logFile != nil {
		a.logFile.WriteString(logMessage)
		a.logFile.Sync() // Ensure it's written immediately
	}
	
	// Also write to stdout
	fmt.Print(logMessage)
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
	// Create log_entries table
	createEntriesTableSQL := `
	CREATE TABLE IF NOT EXISTS log_entries (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		content TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	
	if _, err := a.db.Exec(createEntriesTableSQL); err != nil {
		return fmt.Errorf("failed to create log_entries table: %v", err)
	}
	
	// Create tags table
	createTagsTableSQL := `
	CREATE TABLE IF NOT EXISTS tags (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	
	if _, err := a.db.Exec(createTagsTableSQL); err != nil {
		return fmt.Errorf("failed to create tags table: %v", err)
	}
	
	// Create junction table for many-to-many relationship
	createJunctionTableSQL := `
	CREATE TABLE IF NOT EXISTS log_entries_tags (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		log_entry_id INTEGER NOT NULL,
		tag_id INTEGER NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (log_entry_id) REFERENCES log_entries(id) ON DELETE CASCADE,
		FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE,
		UNIQUE(log_entry_id, tag_id)
	);`
	
	if _, err := a.db.Exec(createJunctionTableSQL); err != nil {
		return fmt.Errorf("failed to create log_entries_tags table: %v", err)
	}
	
	// Create indexes for better query performance
	createIndexSQL := `
	CREATE INDEX IF NOT EXISTS idx_log_entries_tags_entry ON log_entries_tags(log_entry_id);
	CREATE INDEX IF NOT EXISTS idx_log_entries_tags_tag ON log_entries_tags(tag_id);
	CREATE INDEX IF NOT EXISTS idx_tags_name ON tags(name);`
	
	if _, err := a.db.Exec(createIndexSQL); err != nil {
		return fmt.Errorf("failed to create indexes: %v", err)
	}
	
	return nil
}

// GetEntryForEdit returns the entry content for editing (used by /edit command)
func (a *App) GetEntryForEdit(id int) (string, error) {
	entry, err := a.GetEntryByID(id)
	if err != nil {
		return "", err
	}
	return entry.Content, nil
}

// GetEntryPreview returns a preview of the entry (first 100 chars) for delete confirmation
func (a *App) GetEntryPreview(id int) (string, error) {
	entry, err := a.GetEntryByID(id)
	if err != nil {
		return "", err
	}
	preview := entry.Content
	if len(preview) > 100 {
		preview = preview[:100] + "..."
	}
	return preview, nil
}

// ProcessCommand handles slash commands
func (a *App) ProcessCommand(command string) error {
	fmt.Printf("Processing command: %s\n", command)
	
	command = strings.TrimSpace(command)
	
	// Handle commands with arguments
	if strings.HasPrefix(command, "/edit ") {
		parts := strings.Fields(command)
		if len(parts) != 2 {
			return fmt.Errorf("invalid edit command. Usage: /edit <entry-id>")
		}
		
		var entryID int
		if _, err := fmt.Sscanf(parts[1], "%d", &entryID); err != nil {
			return fmt.Errorf("invalid entry ID: %s", parts[1])
		}
		
		// Get entry content for editing
		content, err := a.GetEntryForEdit(entryID)
		if err != nil {
			return err
		}
		
		// Return special error format that frontend can parse
		// Format: EDIT_MODE:<id>:<content>
		return fmt.Errorf("EDIT_MODE:%d:%s", entryID, content)
	}
	
	if strings.HasPrefix(command, "/delete ") {
		parts := strings.Fields(command)
		if len(parts) != 2 {
			return fmt.Errorf("invalid delete command. Usage: /delete <entry-id>")
		}
		
		var entryID int
		if _, err := fmt.Sscanf(parts[1], "%d", &entryID); err != nil {
			return fmt.Errorf("invalid entry ID: %s", parts[1])
		}
		
		// Get entry preview for confirmation
		preview, err := a.GetEntryPreview(entryID)
		if err != nil {
			return err
		}
		
		// Return special error format for confirmation
		// Format: DELETE_CONFIRM:<id>:<preview>
		return fmt.Errorf("DELETE_CONFIRM:%d:%s", entryID, preview)
	}
	
	switch command {
	case "/dash":
		return a.generateDashboard()
	case "/settings":
		a.OpenSettings()
		return nil
	case "/editprev":
		// Get the most recent entry
		entry, err := a.GetMostRecentEntry()
		if err != nil {
			return err
		}
		
		// Return special error format that frontend can parse
		// Format: EDIT_MODE:<id>:<content>
		return fmt.Errorf("EDIT_MODE:%d:%s", entry.ID, entry.Content)
	case "/delprev":
		// Get the most recent entry
		entry, err := a.GetMostRecentEntry()
		if err != nil {
			return err
		}
		
		// Get entry preview for confirmation
		preview := entry.Content
		if len(preview) > 100 {
			preview = preview[:100] + "..."
		}
		
		// Return special error format for confirmation
		// Format: DELETE_CONFIRM:<id>:<preview>
		return fmt.Errorf("DELETE_CONFIRM:%d:%s", entry.ID, preview)
	default:
		return fmt.Errorf("unknown command: %s. Available commands: /dash, /settings, /edit <id>, /delete <id>, /editprev, /delprev", command)
	}
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
	result, err := a.db.Exec(query, text)
	if err != nil {
		return fmt.Errorf("failed to insert log entry: %v", err)
	}

	// Get the inserted entry ID
	entryID, err := result.LastInsertId()
	if err != nil {
		fmt.Printf("Warning: failed to get last insert ID: %v\n", err)
	} else {
		// Extract and save tags
		if err := a.processTags(entryID, text); err != nil {
			fmt.Printf("Warning: failed to process tags: %v\n", err)
		}
	}

	fmt.Printf("Logged text: %s\n", text)
	return nil
}

// processTags extracts hashtags from text and creates associations
func (a *App) processTags(entryID int64, text string) error {
	// Extract tags using regex - matches #word, #word123, #word-word, etc.
	// But not # at end of line or followed by special chars that can't be in tags
	tagPattern := regexp.MustCompile(`#([a-zA-Z0-9_-]+)`)
	matches := tagPattern.FindAllStringSubmatch(text, -1)
	
	if len(matches) == 0 {
		return nil
	}

	// Process each tag
	for _, match := range matches {
		tagName := match[1]
		
		// Get or create tag
		tagID, err := a.getOrCreateTag(tagName)
		if err != nil {
			fmt.Printf("Warning: failed to get or create tag '%s': %v\n", tagName, err)
			continue
		}
		
		// Create association if it doesn't exist
		insertJunctionSQL := `
		INSERT OR IGNORE INTO log_entries_tags (log_entry_id, tag_id)
		VALUES (?, ?)`
		
		if _, err := a.db.Exec(insertJunctionSQL, entryID, tagID); err != nil {
			fmt.Printf("Warning: failed to create tag association: %v\n", err)
		}
	}
	
	return nil
}

// getOrCreateTag returns the ID of a tag, creating it if necessary
func (a *App) getOrCreateTag(tagName string) (int64, error) {
	// Try to get existing tag
	var tagID int64
	query := `SELECT id FROM tags WHERE name = ?`
	err := a.db.QueryRow(query, tagName).Scan(&tagID)
	
	if err == nil {
		// Tag exists
		return tagID, nil
	}
	
	if err != sql.ErrNoRows {
		// Error other than "not found"
		return 0, fmt.Errorf("failed to query tag: %v", err)
	}
	
	// Tag doesn't exist, create it
	insertSQL := `INSERT INTO tags (name) VALUES (?)`
	result, err := a.db.Exec(insertSQL, tagName)
	if err != nil {
		return 0, fmt.Errorf("failed to create tag: %v", err)
	}
	
	// Get the new tag's ID
	tagID, err = result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get tag ID: %v", err)
	}
	
	return tagID, nil
}

// ClearAllData deletes all log entries from the database
func (a *App) ClearAllData() error {
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	// Delete all entries from database
	query := `DELETE FROM log_entries`
	_, err := a.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to delete log entries: %v", err)
	}

	fmt.Println("All log entries deleted successfully")
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
			DateString:   localTime.Format("2006-01-02"),
		}
	}
	
	// Group entries by day using local time
	dayGroups := a.groupDisplayEntriesByDay(displayEntries)
	
	// Calculate statistics
	thisWeek := a.calculateThisWeekCount(entries)
	
	// Get all tags
	tags, err := a.GetTags()
	if err != nil {
		fmt.Printf("Warning: failed to get tags: %v\n", err)
		tags = []Tag{} // Use empty slice if tags fail
	}
	
	var logoData template.URL
	if len(appIcon) > 0 {
		logoData = template.URL("data:image/png;base64," + base64.StdEncoding.EncodeToString(appIcon))
	}

    dayGroupsJSON := make([]map[string]interface{}, len(dayGroups))
    for i, dg := range dayGroups {
        entriesJSON := make([]map[string]interface{}, len(dg.Entries))
        for j, entry := range dg.Entries {
            entriesJSON[j] = map[string]interface{}{
                "id":        entry.ID,
                "content":   entry.RenderedHTML,
                "rawContent": entry.Content,
                "localTime": entry.LocalTime,
                "date":      entry.DateString,
            }
        }

        dayGroupsJSON[i] = map[string]interface{}{
            "dayName": dg.DayName,
            "date":    dg.Date,
            "count":   dg.Count,
            "entries": entriesJSON,
        }
    }

    tagsJSON := make([]map[string]interface{}, len(tags))
    for i, tag := range tags {
        tagsJSON[i] = map[string]interface{}{
            "id":   tag.ID,
            "name": tag.Name,
        }
    }

    jsonSource := map[string]interface{}{
        "totalEntries": totalCount,
        "totalDays":    len(dayGroups),
        "thisWeek":     thisWeek,
        "dayGroups":    dayGroupsJSON,
        "tags":         tagsJSON,
    }

    jsonBytes, err := json.Marshal(jsonSource)
    if err != nil {
        return nil, fmt.Errorf("failed to encode dashboard json: %w", err)
    }

    return &DisplayDashboardData{
        TotalEntries: totalCount,
        TotalDays:    len(dayGroups),
        ThisWeek:     thisWeek,
        Generated:    time.Now().Local().Format("2006-01-02 15:04:05"),
        DayGroups:    dayGroups,
        Tags:         tags,
        LogoData:     logoData,
        OriginalJSONRaw: template.JS(string(jsonBytes)),
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
			Date:    localTime.Format("2006-01-02"), // Use ISO format for JavaScript compatibility
			Count:   len(dayEntries),
			Entries: dayEntries,
		}
		dayGroups = append(dayGroups, dayGroup)
	}
	
	// Sort by date (newest first)
	for i := 0; i < len(dayGroups); i++ {
		for j := i + 1; j < len(dayGroups); j++ {
			date1, _ := time.Parse("2006-01-02", dayGroups[i].Date)
			date2, _ := time.Parse("2006-01-02", dayGroups[j].Date)
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
	// Try to read from embedded FS first (for binary builds)
	templateContent, err := templates.ReadFile("templates/dashboard.html")
	if err != nil {
		// Fallback to file system (for dev mode)
		templatePath := filepath.Join(".", "templates", "dashboard.html")
		templateContent, err = os.ReadFile(templatePath)
		if err != nil {
			return "", fmt.Errorf("failed to read template file: %v", err)
		}
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

// GetEntryByID retrieves a log entry by its ID
func (a *App) GetEntryByID(id int) (*LogEntry, error) {
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var entry LogEntry
	query := `SELECT id, content, created_at FROM log_entries WHERE id = ?`
	err := a.db.QueryRow(query, id).Scan(&entry.ID, &entry.Content, &entry.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("entry not found: %v", err)
	}

	return &entry, nil
}

// GetMostRecentEntry retrieves the most recent log entry
func (a *App) GetMostRecentEntry() (*LogEntry, error) {
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var entry LogEntry
	query := `SELECT id, content, created_at FROM log_entries ORDER BY created_at DESC LIMIT 1`
	err := a.db.QueryRow(query).Scan(&entry.ID, &entry.Content, &entry.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no entries found")
		}
		return nil, fmt.Errorf("failed to get most recent entry: %v", err)
	}

	return &entry, nil
}

// UpdateEntry updates the content of an existing log entry
func (a *App) UpdateEntry(id int, newContent string) error {
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	if newContent == "" {
		return fmt.Errorf("content cannot be empty")
	}

	// First verify the entry exists
	_, err := a.GetEntryByID(id)
	if err != nil {
		return fmt.Errorf("entry not found: %v", err)
	}

	// Update the entry
	query := `UPDATE log_entries SET content = ? WHERE id = ?`
	result, err := a.db.Exec(query, newContent, id)
	if err != nil {
		return fmt.Errorf("failed to update entry: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check update result: %v", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("entry not found or not updated")
	}

	// Update tags for the entry
	if err := a.processTags(int64(id), newContent); err != nil {
		fmt.Printf("Warning: failed to update tags for entry %d: %v\n", id, err)
	}

	return nil
}

// DeleteEntry deletes a log entry by its ID
func (a *App) DeleteEntry(id int) error {
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	// First verify the entry exists
	_, err := a.GetEntryByID(id)
	if err != nil {
		return fmt.Errorf("entry not found: %v", err)
	}

	// Delete the entry (cascade will handle tags)
	query := `DELETE FROM log_entries WHERE id = ?`
	result, err := a.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete entry: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check delete result: %v", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("entry not found or not deleted")
	}

	return nil
}

// Tag represents a tag in the database
type Tag struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// GetTags returns all available tags
func (a *App) GetTags() ([]Tag, error) {
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := `SELECT id, name, created_at FROM tags ORDER BY name ASC`
	rows, err := a.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query tags: %v", err)
	}
	defer rows.Close()

	var tags []Tag
	for rows.Next() {
		var tag Tag
		err := rows.Scan(&tag.ID, &tag.Name, &tag.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan tag: %v", err)
		}
		tags = append(tags, tag)
	}

	return tags, nil
}

// GetEntriesByTags returns log entries filtered by tags
func (a *App) GetEntriesByTags(tagNames []string, limit int) ([]LogEntry, error) {
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	if len(tagNames) == 0 {
		// No tags specified, return all entries
		return a.GetLogEntries(limit)
	}

	// Build query to find entries that have ALL specified tags
	// Using subquery to filter by tag intersection
	placeholders := strings.Repeat("?,", len(tagNames))
	placeholders = placeholders[:len(placeholders)-1] // Remove trailing comma
	
	query := fmt.Sprintf(`
	SELECT DISTINCT e.id, e.content, e.created_at
	FROM log_entries e
	WHERE e.id IN (
		SELECT let.log_entry_id
		FROM log_entries_tags let
		INNER JOIN tags t ON let.tag_id = t.id
		WHERE t.name IN (%s)
		GROUP BY let.log_entry_id
		HAVING COUNT(DISTINCT t.name) = ?
	)
	ORDER BY e.created_at DESC
	LIMIT ?`, placeholders)

	// Build args: tag names, count of tags, and limit
	args := make([]interface{}, 0, len(tagNames)+2)
	for _, tagName := range tagNames {
		args = append(args, tagName)
	}
	args = append(args, len(tagNames), limit)

	rows, err := a.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query entries by tags: %v", err)
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
	// Use Minimize to keep the app accessible from the dock/taskbar
	wailsRuntime.WindowMinimise(a.ctx)
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

// GetDashboardPath returns the dashboard directory path
func (a *App) GetDashboardPath() string {
	tempPath, err := a.getTempPath()
	if err != nil {
		return "unknown"
	}
	return tempPath
}

// ClearDashboardFiles removes generated dashboard HTML files from the temp directory
func (a *App) ClearDashboardFiles() (string, error) {
    tempPath, err := a.getTempPath()
    if err != nil {
        return "", fmt.Errorf("unable to locate dashboard directory: %w", err)
    }

    entries, err := os.ReadDir(tempPath)
    if err != nil {
        return "", fmt.Errorf("unable to inspect dashboard directory: %w", err)
    }

    pattern := regexp.MustCompile(`^snaplog-dashboard.*\.html$`)
    removed := 0

    for _, entry := range entries {
        if entry.Type().IsRegular() && pattern.MatchString(entry.Name()) {
            if err := os.Remove(filepath.Join(tempPath, entry.Name())); err != nil {
                return "", fmt.Errorf("failed to remove %s: %w", entry.Name(), err)
            }
            removed++
        }
    }

    message := "No dashboard files found to remove"
    if removed > 0 {
        message = fmt.Sprintf("Removed %d dashboard file(s)", removed)
    }

    return message, nil
}

// Quit closes the application
func (a *App) Quit() {
	fmt.Println("Quitting SnapLog...")
	wailsRuntime.Quit(a.ctx)
}

// startHotkeyDetection registers a global hotkey using golang.design/x/hotkey
func (a *App) startHotkeyDetection() {
	fmt.Println("Starting hotkey detection...")
	
	// Parse modifiers from settings (platform-specific implementation)
	modifiers := parseModifiers(a.settings.HotkeyModifiers)
	
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


// OpenSettings opens the settings modal
func (a *App) OpenSettings() {
	// Show the main window first
	a.ShowWindow()
	// Then emit the event to open settings modal
	wailsRuntime.EventsEmit(a.ctx, "open-settings")
}

// IsFirstRun checks if this is the first run of the app
func (a *App) IsFirstRun() bool {
	return a.settings.FirstRun
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
	fmt.Println("\nSettings:")
	fmt.Println("  Type /settings or click the settings button to configure hotkey")
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
	// Mark that setup is complete
	a.settings.FirstRun = false
	
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
		fmt.Println("Using default settings (first run)")
		a.settings.FirstRun = true
		return
	}
	
	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		fmt.Printf("Failed to parse settings: %v\n", err)
		return
	}
	
	a.settings = &settings
	// If FirstRun field was not in JSON (old settings), default to true to force first run
	// Check if the file actually contains the first_run field
	if !strings.Contains(string(data), "first_run") {
		fmt.Println("Old settings detected - forcing first run")
		a.settings.FirstRun = true
	}
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