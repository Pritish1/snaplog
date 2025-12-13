# SnapLog

A lightweight journaling utility that captures notes from anywhere with a single hotkey. Built with Go (Wails) and React, stores everything locally in SQLite.

## Features

- **Global hotkey**: Default `Ctrl+Shift+L` (configurable)
- **Quick capture**: Type → Enter → done
- **Markdown support**: Full markdown rendering in entries
- **Commands**: `/dash` (dashboard), `/settings`, `/edit <id>`, `/editprev`, `/delprev`
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

**Important**: Before building, ensure custom icons are in the build directory:

```bash
# Windows
mkdir -p build/windows
cp assets/icons/icon.ico build/windows/icon.ico
cp assets/icons/appicon.png build/appicon.png

# macOS/Linux
mkdir -p build/darwin
cp assets/icons/appicon.png build/appicon.png
```

Then build:
```bash
wails build
```

The GitHub Actions workflow automatically copies icons during CI builds.

## Usage

1. Launch SnapLog
2. Press hotkey (`Ctrl+Shift+L` by default) to open entry window
3. Type your note and press **Enter** to save
4. Use `/dash` to view all entries in a web dashboard
5. Use `/settings` to configure hotkey and theme

### Keyboard Shortcuts

- **Enter**: Save and hide window
- **Shift+Enter**: New line
- **Esc**: Hide window without saving

### Commands

- `/dash` - Open dashboard
- `/settings` - Open settings
- `/edit <id>` - Edit entry by ID
- `/editprev` - Edit most recent entry
- `/delprev` - Delete most recent entry

### Managing Entries in the Dashboard

- **Delete entries**: Click 🗑️ to delete the entry
- **Edit entries**: Click ✏️ → Paste the copied command in the CLI → Edit the entry → Press Enter

## Data Locations

- **Database**: `%APPDATA%/snaplog/snaplog.db` (Windows), `~/Library/Application Support/snaplog/snaplog.db` (macOS), `$XDG_CONFIG_HOME/snaplog/snaplog.db` (Linux)
- **Settings**: `settings.json` in same directory
- **Logs**: `snaplog-YYYY-MM-DD.log` in same directory
- **Dashboards**: System temp directory under `snaplog-dashboards/`

## Platform Notes

**macOS**
- App is notarized by Apple for security
- No additional setup required

**Windows**
- No additional setup required
- App is not code-signed

**Linux**
- Requires X11 for global hotkeys (may not work on Wayland)

## License

MIT License
