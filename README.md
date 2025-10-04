# SnapLog - Hotkey Text Logger

A simple, fast text logging utility that can be triggered with a global hotkey. Built with Go (Wails) and React.

## Features

- **Global Hotkey**: Press `Ctrl+Shift+L` (Windows) or `Cmd+Shift+L` (macOS) from anywhere to popup the logging window
- **Quick Text Entry**: Simple interface focused on fast text logging
- **Markdown Support**: Full Markdown rendering with live preview
- **SQLite Database**: Robust data storage with automatic timestamps and search capabilities
- **System Tray Integration**: Runs in background with configurable settings menu
- **Cross-Platform**: Supports Windows, macOS, and Linux
- **Data Durability**: All entries stored in SQLite database with full query support

## How to Use

1. **Start the Application**: Run `wails dev` for development or `wails build` for production
2. **Register Hotkey**: The app starts hidden and registers the global hotkey
   - **Windows**: `Ctrl+Shift+L` (no additional setup required)
   - **macOS**: `Cmd+Shift+L` (requires accessibility permissions - see Setup below)
   - **Linux**: `Ctrl+Shift+L` (may require X11 configuration)
3. **Log Text**: Press the hotkey to show the window, type your text, and press Enter
4. **Markdown Preview**: Press Ctrl+Tab to toggle between edit and preview modes
5. **Access Settings**: Right-click system tray icon to configure hotkey and view instructions
6. **Database Storage**: All entries are stored in SQLite database with timestamps

### Platform Setup

#### macOS Setup
On macOS, SnapLog requires accessibility permissions to register global hotkeys:

1. When you first run SnapLog, it will prompt you to grant accessibility permissions
2. Go to **System Preferences** > **Security & Privacy** > **Privacy** > **Accessibility**
3. Click the lock icon and enter your password
4. Find "SnapLog" in the list and check the box to enable it
5. If SnapLog isn't in the list, click the "+" button and add it manually
6. Restart SnapLog after granting permissions

**Note**: Without accessibility permissions, the global hotkey won't work, but you can still use SnapLog by clicking on the app icon in the Dock.

#### Linux Setup
On Linux, SnapLog uses X11 for global hotkey detection:

1. Ensure you're running an X11 session (not Wayland)
2. Install required packages: `sudo apt install libx11-dev` (Ubuntu/Debian) or equivalent
3. The hotkey should work automatically after installation

**Note**: Wayland users may need to use alternative methods or switch to X11 for global hotkey support.

## Development

### Prerequisites
- Go 1.23+ (with CGO enabled)
- Node.js
- Wails v2
- C Compiler (for SQLite support)

### Running in Development
```bash
wails dev
```

### Building for Production
```bash
wails build
```

**Note**: The application requires CGO to be enabled for SQLite support. This is configured in `wails.json` with `"cgo": true`.

## File Structure

```
snaplog/
├── app.go                 # Main application logic with SQLite integration
├── main.go               # Application entry point
├── frontend/             # React frontend
│   ├── src/
│   │   ├── App.jsx       # Main React component
│   │   └── App.css       # Styling
└── build/                # Built application
    ├── appicon.png       # Application icon
    └── bin/
        └── snaplog.exe   # Windows executable
```

**Data Storage**: SQLite database stored in user config directory:
- **Windows**: `%APPDATA%\snaplog\snaplog.db`
- **macOS**: `~/Library/Application Support/snaplog/snaplog.db`
- **Linux**: `~/.config/snaplog/snaplog.db`

## Security Notes

- **Cross-Platform**: Uses `golang.design/x/hotkey` package for secure, native hotkey registration
- **Minimal Permissions**: Only requests necessary system permissions for hotkey functionality
- **Platform-Specific Security**:
  - **Windows**: Uses native `RegisterHotKey` API
  - **macOS**: Uses Core Graphics Event Taps (requires accessibility permissions)
  - **Linux**: Uses X11 event handling
- Only captures the specific hotkey combination (`Ctrl+Shift+L` on Windows/Linux, `Cmd+Shift+L` on macOS)
- Does not capture or store any other keyboard input
- All data stays local on your machine
- No network connections or data transmission
- Open source implementation with transparent security model

## Database Features

SnapLog now includes a SQLite database with the following capabilities:

- **Structured Storage**: All log entries stored with ID, content, and timestamp
- **Search Functionality**: Built-in search methods for finding specific entries
- **Query Support**: Methods to retrieve entries by count, date range, or content
- **Data Integrity**: ACID compliance and transaction support
- **Performance**: Fast insertion and retrieval even with thousands of entries

### Available Database Methods

- `GetLogEntries(limit)` - Retrieve recent entries
- `GetLogEntriesCount()` - Get total entry count
- `SearchLogEntries(searchText, limit)` - Search entries by content
- `GetDatabasePath()` - Get database file location
- `RenderMarkdown(markdown)` - Convert Markdown to HTML

## Markdown Features

SnapLog includes comprehensive Markdown support:

- **Live Preview**: Toggle between edit and preview modes with Ctrl+Tab
- **Backend Rendering**: Uses Goldmark parser for reliable Markdown processing
- **Full Syntax Support**: Headers, lists, links, bold, italic, code blocks, tables
- **GitHub Flavored Markdown**: Strikethrough, task lists, and more
- **Safe Rendering**: HTML is sanitized for security

### Supported Markdown Elements

- **Headers**: `# H1`, `## H2`, `### H3`
- **Text Formatting**: `**bold**`, `*italic*`, `~~strikethrough~~`
- **Lists**: `- Bullet`, `1. Numbered`, `- [ ] Task`
- **Links**: `[text](url)`, `![alt](image)`
- **Code**: `` `inline` ``, ``` code blocks ```
- **Tables**: Pipe-separated columns
- **Blockquotes**: `> quoted text`
- **Horizontal Rules**: `---`
