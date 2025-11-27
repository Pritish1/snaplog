# SnapLog

A lightweight journaling utility that captures notes from anywhere with a single hotkey. Built with Go (Wails) and React, stores everything locally in SQLite.

## Features

- **Global hotkey**: Default `Ctrl+Shift+L` (configurable)
- **Quick capture**: Type → Enter → done
- **Markdown support**: Live preview with `Ctrl/Cmd+Tab`
- **Commands**: `/dash` (dashboard), `/settings`, `/edit <id>`, `/delete <id>`, `/editprev`, `/delprev`
- **Tags**: Use `#tag` in entries for organization
- **Dashboard**: HTML view with filtering by date and tags

## Installation

### Prerequisites

- Go 1.23+ (CGO enabled)
- Node.js 18+ and npm
- Wails CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- C compiler (for SQLite/CGO)

### Development

```bash
npm --prefix frontend install
wails dev
```

### Build

```bash
# macOS
bash ./build-macos.sh

# Other platforms
wails build
```

## Usage

1. Launch SnapLog
2. Press hotkey (`Ctrl+Shift+L` by default) to open entry window
3. Type your note and press **Enter** to save
4. Use `/dash` to view all entries in a web dashboard
5. Use `/settings` to configure hotkey and theme

### Keyboard Shortcuts

- **Enter**: Save and hide window
- **Shift+Enter**: New line
- **Ctrl/Cmd+Tab**: Toggle markdown preview
- **Esc**: Hide window without saving

### Commands

- `/dash` - Open dashboard
- `/settings` - Open settings
- `/edit <id>` - Edit entry by ID
- `/editprev` - Edit most recent entry
- `/delete <id>` - Delete entry by ID
- `/delprev` - Delete most recent entry

## Data Locations

- **Database**: `%APPDATA%/snaplog/snaplog.db` (Windows), `~/Library/Application Support/snaplog/snaplog.db` (macOS), `$XDG_CONFIG_HOME/snaplog/snaplog.db` (Linux)
- **Settings**: `settings.json` in same directory
- **Logs**: `snaplog-YYYY-MM-DD.log` in same directory
- **Dashboards**: System temp directory under `snaplog-dashboards/`

## Platform Notes

**macOS**
- Requires Accessibility permission for global hotkeys
- Grant permission in System Settings → Privacy & Security → Accessibility

**Windows**
- No additional setup required

**Linux**
- Requires X11 for global hotkeys (may not work on Wayland)

## License

MIT License
