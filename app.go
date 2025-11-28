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
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
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

// DisplayEntry represents a log entry formatted for display
type DisplayEntry struct {
	ID           int             `json:"id"`
	Content      string          `json:"content"`
	RenderedHTML template.HTML   `json:"rendered_html"`
	LocalTime    string          `json:"local_time"`    // Format: "15:04"
	LocalTimeFull string         `json:"local_time_full"` // Format: "15:04:05" for hover
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
	httpServer   *http.Server
	dashboardPort int
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
		dashboardPort: 37564, // Rarely used port
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.hotkeyId = uintptr(1)
	
	if err := a.initLogging(); err != nil {
		fmt.Printf("Warning: Failed to initialize logging: %v\n", err)
	}
	
	a.loadSettings()
	
	if err := a.initDatabase(); err != nil {
		a.logf("Failed to initialize database: %v\n", err)
		return
	}
	
	// Start HTTP server for dashboard
	go a.startDashboardServer()
	
	if a.settings.FirstRun {
		a.logf("First run detected - showing setup window\n")
		go func() {
			time.Sleep(500 * time.Millisecond)
			a.ShowWindow()
			wailsRuntime.EventsEmit(a.ctx, "show-first-run-setup")
		}()
	} else {
		go a.startHotkeyDetection()
	}
}

func (a *App) shutdown(ctx context.Context) {
	a.logf("Shutting down SnapLog...\n")
	a.stopHotkeyDetection()
	
	// Shutdown HTTP server
	if a.httpServer != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		a.httpServer.Shutdown(shutdownCtx)
		a.logf("Dashboard server stopped\n")
	}
	
	if a.db != nil {
		a.db.Close()
		a.logf("Database connection closed\n")
	}
	
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

func (a *App) logf(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logMessage := fmt.Sprintf("[%s] %s", timestamp, message)
	
	if a.logFile != nil {
		a.logFile.WriteString(logMessage)
		a.logFile.Sync()
	}
	
	fmt.Print(logMessage)
}


func (a *App) initDatabase() error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get config directory: %v", err)
	}
	
	snaplogDir := filepath.Join(configDir, "snaplog")
	if err := os.MkdirAll(snaplogDir, 0755); err != nil {
		return fmt.Errorf("failed to create snaplog directory: %v", err)
	}
	
	dbPath := filepath.Join(snaplogDir, "snaplog.db")
	
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %v", err)
	}
	
	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %v", err)
	}
	
	a.db = db
	
	if err := a.createTables(); err != nil {
		return fmt.Errorf("failed to create tables: %v", err)
	}
	
	a.logf("Database initialized at: %s\n", dbPath)
	return nil
}

func (a *App) createTables() error {
	createEntriesTableSQL := `
	CREATE TABLE IF NOT EXISTS log_entries (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		content TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	
	if _, err := a.db.Exec(createEntriesTableSQL); err != nil {
		return fmt.Errorf("failed to create log_entries table: %v", err)
	}
	
	createTagsTableSQL := `
	CREATE TABLE IF NOT EXISTS tags (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	
	if _, err := a.db.Exec(createTagsTableSQL); err != nil {
		return fmt.Errorf("failed to create tags table: %v", err)
	}
	
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
	
	createIndexSQL := `
	CREATE INDEX IF NOT EXISTS idx_log_entries_tags_entry ON log_entries_tags(log_entry_id);
	CREATE INDEX IF NOT EXISTS idx_log_entries_tags_tag ON log_entries_tags(tag_id);
	CREATE INDEX IF NOT EXISTS idx_tags_name ON tags(name);`
	
	if _, err := a.db.Exec(createIndexSQL); err != nil {
		return fmt.Errorf("failed to create indexes: %v", err)
	}
	
	return nil
}

func (a *App) GetEntryForEdit(id int) (string, error) {
	entry, err := a.GetEntryByID(id)
	if err != nil {
		return "", err
	}
	return entry.Content, nil
}

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

func (a *App) ProcessCommand(command string) error {
	a.logf("Processing command: %s\n", command)
	
	command = strings.TrimSpace(command)
	
	if strings.HasPrefix(command, "/edit ") {
		parts := strings.Fields(command)
		if len(parts) != 2 {
			return fmt.Errorf("invalid edit command. Usage: /edit <entry-id>")
		}
		
		var entryID int
		if _, err := fmt.Sscanf(parts[1], "%d", &entryID); err != nil {
			return fmt.Errorf("invalid entry ID: %s", parts[1])
		}
		
		content, err := a.GetEntryForEdit(entryID)
		if err != nil {
			return err
		}
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
		
		preview, err := a.GetEntryPreview(entryID)
		if err != nil {
			return err
		}
		return fmt.Errorf("DELETE_CONFIRM:%d:%s", entryID, preview)
	}
	
	switch command {
	case "/dash":
		return a.generateDashboard()
	case "/settings":
		a.OpenSettings()
		return nil
	case "/editprev":
		entry, err := a.GetMostRecentEntry()
		if err != nil {
			return err
		}
		return fmt.Errorf("EDIT_MODE:%d:%s", entry.ID, entry.Content)
	case "/delprev":
		entry, err := a.GetMostRecentEntry()
		if err != nil {
			return err
		}
		preview := entry.Content
		if len(preview) > 100 {
			preview = preview[:100] + "..."
		}
		return fmt.Errorf("DELETE_CONFIRM:%d:%s", entry.ID, preview)
	default:
		return fmt.Errorf("unknown command: %s. Available commands: /dash, /settings, /edit <id>, /delete <id>, /editprev, /delprev", command)
	}
}
func (a *App) LogText(text string) error {
	if text == "" {
		return nil
	}

	a.logf("LogText called with: '%s'\n", text)

	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	query := `INSERT INTO log_entries (content) VALUES (?)`
	result, err := a.db.Exec(query, text)
	if err != nil {
		return fmt.Errorf("failed to insert log entry: %v", err)
	}

	entryID, err := result.LastInsertId()
	if err != nil {
		a.logf("Warning: failed to get last insert ID: %v\n", err)
	} else {
		if err := a.processTags(entryID, text); err != nil {
			a.logf("Warning: failed to process tags: %v\n", err)
		}
	}

	a.logf("Logged text: %s\n", text)
	return nil
}

func (a *App) processTags(entryID int64, text string) error {
	tagPattern := regexp.MustCompile(`#([a-zA-Z0-9_-]+)`)
	matches := tagPattern.FindAllStringSubmatch(text, -1)
	
	if len(matches) == 0 {
		return nil
	}

	for _, match := range matches {
		tagName := match[1]
		tagID, err := a.getOrCreateTag(tagName)
		if err != nil {
			a.logf("Warning: failed to get or create tag '%s': %v\n", tagName, err)
			continue
		}
		
		insertJunctionSQL := `INSERT OR IGNORE INTO log_entries_tags (log_entry_id, tag_id) VALUES (?, ?)`
		if _, err := a.db.Exec(insertJunctionSQL, entryID, tagID); err != nil {
			a.logf("Warning: failed to create tag association: %v\n", err)
		}
	}
	
	return nil
}

func (a *App) getOrCreateTag(tagName string) (int64, error) {
	var tagID int64
	query := `SELECT id FROM tags WHERE name = ?`
	err := a.db.QueryRow(query, tagName).Scan(&tagID)
	
	if err == nil {
		return tagID, nil
	}
	
	if err != sql.ErrNoRows {
		return 0, fmt.Errorf("failed to query tag: %v", err)
	}
	
	insertSQL := `INSERT INTO tags (name) VALUES (?)`
	result, err := a.db.Exec(insertSQL, tagName)
	if err != nil {
		return 0, fmt.Errorf("failed to create tag: %v", err)
	}
	
	tagID, err = result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get tag ID: %v", err)
	}
	
	return tagID, nil
}

func (a *App) ClearAllData() error {
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	query := `DELETE FROM log_entries`
	_, err := a.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to delete log entries: %v", err)
	}

	a.logf("All log entries deleted successfully\n")
	return nil
}

func (a *App) generateDashboard() error {
	a.logf("Opening dashboard...\n")
	
	dashboardURL := fmt.Sprintf("http://localhost:%d/dash", a.dashboardPort)
	
	if err := a.openInBrowser(dashboardURL); err != nil {
		return fmt.Errorf("failed to open dashboard: %v", err)
	}

	a.logf("Dashboard opened at %s\n", dashboardURL)
	return nil
}

func (a *App) getDashboardData() (*DisplayDashboardData, error) {
	entries, err := a.GetLogEntries(1000)
	if err != nil {
		return nil, fmt.Errorf("failed to get log entries: %v", err)
	}
	
	totalCount, err := a.GetLogEntriesCount()
	if err != nil {
		return nil, fmt.Errorf("failed to get log count: %v", err)
	}
	
	displayEntries := make([]DisplayEntry, len(entries))
	for i, entry := range entries {
		localTime := entry.CreatedAt.Local()
		renderedHTML, err := a.RenderMarkdown(entry.Content)
		if err != nil {
			renderedHTML = fmt.Sprintf("<p>%s</p>", strings.ReplaceAll(entry.Content, "\n", "<br>"))
		}
		
		displayEntries[i] = DisplayEntry{
			ID:            entry.ID,
			Content:       entry.Content,
			RenderedHTML:  template.HTML(renderedHTML),
			LocalTime:     localTime.Format("15:04"),
			LocalTimeFull: localTime.Format("15:04:05"),
			CreatedAt:     entry.CreatedAt,
			DateString:    localTime.Format("2006-01-02"),
		}
	}
	
	dayGroups := a.groupDisplayEntriesByDay(displayEntries)
	thisWeek := a.calculateThisWeekCount(entries)
	
	tags, err := a.GetTags()
	if err != nil {
		a.logf("Warning: failed to get tags: %v\n", err)
		tags = []Tag{}
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
                "id":           entry.ID,
                "content":      entry.RenderedHTML,
                "rawContent":   entry.Content,
                "localTime":    entry.LocalTime,
                "localTimeFull": entry.LocalTimeFull,
                "date":         entry.DateString,
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

func (a *App) groupDisplayEntriesByDay(entries []DisplayEntry) []DisplayDayGroup {
	dayMap := make(map[string][]DisplayEntry)
	
	for _, entry := range entries {
		localTime := entry.CreatedAt.Local()
		dayKey := localTime.Format("2006-01-02")
		dayMap[dayKey] = append(dayMap[dayKey], entry)
	}
	
	var dayGroups []DisplayDayGroup
	for _, dayEntries := range dayMap {
		localTime := dayEntries[0].CreatedAt.Local()
		dayGroup := DisplayDayGroup{
			DayName: localTime.Format("Monday"),
			Date:    localTime.Format("2006-01-02"),
			Count:   len(dayEntries),
			Entries: dayEntries,
		}
		dayGroups = append(dayGroups, dayGroup)
	}
	
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

func (a *App) generateHTMLFromTemplate(data *DisplayDashboardData) (string, error) {
	templateContent, err := templates.ReadFile("templates/dashboard.html")
	if err != nil {
		templatePath := filepath.Join(".", "templates", "dashboard.html")
		templateContent, err = os.ReadFile(templatePath)
		if err != nil {
			return "", fmt.Errorf("failed to read template file: %v", err)
		}
	}
	
	tmpl, err := template.New("dashboard").Parse(string(templateContent))
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %v", err)
	}
	
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %v", err)
	}
	
	return buf.String(), nil
}

// startDashboardServer starts the HTTP server for serving the dashboard
func (a *App) startDashboardServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/dash", a.serveDashboard)
	mux.HandleFunc("/api/entries/", a.handleEntryAPI)
	
	server := &http.Server{
		Addr:    fmt.Sprintf("localhost:%d", a.dashboardPort),
		Handler: mux,
	}
	
	a.httpServer = server
	
	a.logf("Dashboard server starting on http://localhost:%d/dash\n", a.dashboardPort)
	
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		a.logf("Failed to start dashboard server: %v\n", err)
	}
}

// handleEntryAPI handles API requests for entries (DELETE)
func (a *App) handleEntryAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Extract entry ID from path: /api/entries/123
	path := strings.TrimPrefix(r.URL.Path, "/api/entries/")
	entryID, err := strconv.Atoi(path)
	if err != nil {
		http.Error(w, "Invalid entry ID", http.StatusBadRequest)
		return
	}
	
	// Delete the entry
	if err := a.DeleteEntry(entryID); err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete entry: %v", err), http.StatusInternalServerError)
		a.logf("Error deleting entry %d: %v\n", entryID, err)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Entry deleted successfully",
	})
	
	a.logf("Entry %d deleted via dashboard\n", entryID)
}

// serveDashboard generates and serves the dashboard HTML
func (a *App) serveDashboard(w http.ResponseWriter, r *http.Request) {
	data, err := a.getDashboardData()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get dashboard data: %v", err), http.StatusInternalServerError)
		a.logf("Error getting dashboard data: %v\n", err)
		return
	}

	htmlContent, err := a.generateHTMLFromTemplate(data)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to generate HTML: %v", err), http.StatusInternalServerError)
		a.logf("Error generating HTML: %v\n", err)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(htmlContent))
}

func (a *App) openInBrowser(urlOrPath string) error {
	var cmd *exec.Cmd
	
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", urlOrPath)
	case "darwin":
		cmd = exec.Command("open", urlOrPath)
	case "linux":
		cmd = exec.Command("xdg-open", urlOrPath)
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

func (a *App) UpdateEntry(id int, newContent string) error {
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	if newContent == "" {
		return fmt.Errorf("content cannot be empty")
	}

	_, err := a.GetEntryByID(id)
	if err != nil {
		return fmt.Errorf("entry not found: %v", err)
	}

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

	if err := a.processTags(int64(id), newContent); err != nil {
		a.logf("Warning: failed to update tags for entry %d: %v\n", id, err)
	}

	return nil
}

func (a *App) DeleteEntry(id int) error {
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	_, err := a.GetEntryByID(id)
	if err != nil {
		return fmt.Errorf("entry not found: %v", err)
	}

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


func (a *App) RenderMarkdown(markdown string) (string, error) {
	var buf bytes.Buffer
	md := goldmark.New()
	if err := md.Convert([]byte(markdown), &buf); err != nil {
		return "", fmt.Errorf("failed to render markdown: %v", err)
	}
	return buf.String(), nil
}

func (a *App) ShowWindow() {
	wailsRuntime.WindowShow(a.ctx)
	wailsRuntime.WindowUnminimise(a.ctx)
}

func (a *App) HideWindow() {
	wailsRuntime.WindowMinimise(a.ctx)
}

func (a *App) GetDatabasePath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "unknown"
	}
	snaplogDir := filepath.Join(configDir, "snaplog")
	return filepath.Join(snaplogDir, "snaplog.db")
}


func (a *App) Quit() {
	a.logf("Quitting SnapLog...\n")
	wailsRuntime.Quit(a.ctx)
}

func (a *App) startHotkeyDetection() {
	a.logf("Starting hotkey detection...\n")
	
	modifiers := parseModifiers(a.settings.HotkeyModifiers)
	
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
		key = hotkey.KeyL
	}
	
	hk := hotkey.New(modifiers, key)
	
	if err := hk.Register(); err != nil {
		a.logf("Failed to register hotkey: %v\n", err)
		a.logf("Note: On macOS, this requires accessibility permissions.\n")
		return
	}
	
	a.logf("Hotkey registered: %v+%v\n", a.settings.HotkeyModifiers, a.settings.HotkeyKey)
	a.packageHotkey = hk
	
	go func() {
		for {
			select {
			case <-hk.Keydown():
				a.logf("Hotkey detected! Showing window...\n")
				a.ShowWindow()
			}
		}
	}()
}

func (a *App) stopHotkeyDetection() {
	if a.packageHotkey != nil {
		a.logf("Unregistering hotkey...\n")
		a.packageHotkey.Unregister()
		a.packageHotkey = nil
	}
}

func (a *App) OpenSettings() {
	a.ShowWindow()
	wailsRuntime.EventsEmit(a.ctx, "open-settings")
}

func (a *App) IsFirstRun() bool {
	return a.settings.FirstRun
}

func (a *App) GetSettings() *Settings {
	return a.settings
}

func (a *App) SetSettings(settings *Settings) error {
	a.settings = settings
	a.settings.FirstRun = false
	
	if err := a.saveSettings(); err != nil {
		return fmt.Errorf("failed to save settings: %v", err)
	}
	
	a.stopHotkeyDetection()
	go a.startHotkeyDetection()
	
	return nil
}

func (a *App) loadSettings() {
	configDir, err := os.UserConfigDir()
	if err != nil {
		a.logf("Failed to get config directory: %v\n", err)
		return
	}
	
	settingsFile := filepath.Join(configDir, "snaplog", "settings.json")
	
	data, err := os.ReadFile(settingsFile)
	if err != nil {
		a.logf("Using default settings (first run)\n")
		a.settings.FirstRun = true
		return
	}
	
	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		a.logf("Failed to parse settings: %v\n", err)
		return
	}
	
	a.settings = &settings
	if !strings.Contains(string(data), "first_run") {
		a.logf("Old settings detected - forcing first run\n")
		a.settings.FirstRun = true
	}
	a.logf("Settings loaded successfully\n")
}

func (a *App) saveSettings() error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get config directory: %v", err)
	}
	
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
	
	a.logf("Settings saved successfully\n")
	return nil
}