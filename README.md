# SnapLog - Hotkey Text Logger

A simple, fast text logging utility that can be triggered with a global hotkey. Built with Go (Wails) and React.

## Features

- **Global Hotkey**: Press `Ctrl+Shift+L` (Windows) or `Cmd+Shift+L` (macOS) from anywhere to popup the logging window
- **Quick Text Entry**: Simple interface focused on fast text logging
- **Automatic Timestamps**: All entries are timestamped automatically
- **File Logging**: Text is appended to `logs/snaplog.txt`
- **Cross-Platform**: Supports Windows, macOS, and Linux

## How to Use

1. **Start the Application**: Run `wails dev` for development or `wails build` for production
2. **Register Hotkey**: The app starts hidden and registers the global hotkey
   - **Windows**: `Ctrl+Shift+L` (no additional setup required)
   - **macOS**: `Cmd+Shift+L` (requires accessibility permissions - see Setup below)
   - **Linux**: `Ctrl+Shift+L` (may require X11 configuration)
3. **Log Text**: Press the hotkey to show the window, type your text, and press Enter or click "Log Text"
4. **View Logs**: Check `logs/snaplog.txt` for your logged entries

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
- Go 1.23+
- Node.js
- Wails v2

### Running in Development
```bash
wails dev
```

### Building for Production
```bash
wails build
```

## File Structure

```
snaplog/
├── app.go                 # Main application logic (includes unified hotkey implementation)
├── main.go               # Application entry point
├── frontend/             # React frontend
│   ├── src/
│   │   ├── App.jsx       # Main React component
│   │   └── App.css       # Styling
└── logs/                 # Log files directory (created automatically)
    └── snaplog.txt       # Text log file
```

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

## Future Enhancements

- Configurable hotkey combinations
- Multiple log file formats
- Search and filter functionality
- Auto-hide after logging option
- System tray integration
- Wayland support for Linux
- Customizable themes and UI
