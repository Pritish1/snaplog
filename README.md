# SnapLog

SnapLog is a lightweight journaling utility that lets you capture notes from anywhere with a single hotkey. It is built with Go (Wails) on the backend and React on the frontend, and persists everything locally in SQLite.

## Highlights

- **Global hotkey**: Default `Ctrl+Shift+L` (all platforms) with in-app configuration.
- **Quick input**: Minimal UI focused on type → press Enter → hide.
- **Markdown**: Toggle live preview with `Ctrl/Cmd+Tab`; rendering powered by Goldmark.
- **Slash commands**: `/dash` to generate the dashboard, `/settings` to open preferences.
- **Tray / menu-bar integration**: Background service with “Show App” and “Quit” options.
- **Local-first storage**: SQLite database under the user config directory; no network access.

## Prerequisites

- Go 1.23 or newer (CGO enabled)
- Node.js 18+ and npm
- Wails CLI (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)
- A C compiler toolchain (required by SQLite / CGO)

## Development Workflow

```bash
# install frontend dependencies
npm --prefix frontend install

# run in development
wails dev
```

The development build launches the React dev server and the Wails runtime with live reload. On first launch you may need to grant OS permissions (see below).

### Platform Notes

**macOS**
- The app requires Accessibility permission for global hotkeys. When prompted, enable SnapLog under *System Settings → Privacy & Security → Accessibility* and restart the app.
- The tray icon is a monochrome template asset; right-click (two-finger tap) to show the menu. Left-click brings the window forward.

**Windows**
- No additional setup is required. The tray icon uses the embedded ICO asset. Right-click to open the menu.

**Linux**
- X11 is required for global hotkeys (`golang.design/x/hotkey`). On Wayland, hotkeys may not register.

## Building Releases

```bash
wails build            # builds for current platform
wails build -platform darwin/amd64   # example: cross-build (requires macOS)
```

Artifacts are written to `build/bin/`. When packaging, ensure the icons in `assets/icons/` are up to date—Wails consumes them during the build step.

## Usage

1. Launch SnapLog (dev or packaged build).
2. Hit the hotkey (`Ctrl+Shift+L` by default) to toggle the entry window.
3. Type a note:
   - Press **Enter** to save and hide.
   - Press **Shift+Enter** for a new line.
   - Use `/dash` or `/settings` for quick commands.
4. Toggle preview with **Ctrl/Cmd + Tab**.
5. Tray/menu actions:
   - **Show App**: bring window to front.
   - **Quit App**: exit SnapLog while keeping data.

### Data Locations

- **Database**: `%APPDATA%/snaplog/snaplog.db` (Windows), `~/Library/Application Support/snaplog/snaplog.db` (macOS), `$XDG_CONFIG_HOME/snaplog/snaplog.db` (Linux)
- **Settings**: Stored alongside the database as `settings.json`.
- **Temporary dashboards**: Inside the system temp directory under `snaplog-dashboards/`.

## Project Layout

```
root/
├── app.go            # backend application logic
├── frontend/         # React UI
│   └── src/
│       ├── App.jsx
│       └── assets/
├── assets/icons/     # source icon assets (checked into git)
├── templates/        # HTML templates for reports
└── wails.json        # Wails project configuration
```

## Updating Assets

- **Tray icons**: replace files in `assets/icons/`. Supply 16×16/32×32 (ICO) for Windows and 18×18 / 36×36 template PNGs for macOS if you need alternate styling.
- **Dock / app icon**: provide high-resolution PNG or ICNS variants and update `build/darwin/` before running `wails build` on macOS.

## Troubleshooting

- **Hotkey not responding (macOS)**: confirm Accessibility permission and restart the app.
- **Tray icon missing (Windows)**: verify `assets/icons/icon.ico` includes 16×16 and 32×32 entries.
- **Slash command logged as text**: only `/dash` and `/settings` are treated as commands; other `/something` inputs are stored as normal notes.

## License

This project is released under the [MIT License](./LICENSE).
