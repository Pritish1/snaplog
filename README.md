# SnapLog - Hotkey Text Logger

A simple, fast text logging utility that can be triggered with a global hotkey. Built with Go (Wails) and React.

## Features

- **Global Hotkey**: Press `Ctrl+Shift+L` from anywhere to popup the logging window
- **Quick Text Entry**: Simple interface focused on fast text logging
- **Automatic Timestamps**: All entries are timestamped automatically
- **File Logging**: Text is appended to `logs/snaplog.txt`
- **Cross-Platform Ready**: Currently supports Windows (macOS/Linux support planned)

## How to Use

1. **Start the Application**: Run `wails dev` for development or `wails build` for production
2. **Register Hotkey**: The app starts hidden and registers `Ctrl+Shift+L` globally
3. **Log Text**: Press the hotkey to show the window, type your text, and press Enter or click "Log Text"
4. **View Logs**: Check `logs/snaplog.txt` for your logged entries

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
├── app.go                 # Main application logic
├── hotkey_windows.go      # Windows hotkey implementation
├── hotkey_stub.go         # Non-Windows stub
├── main.go               # Application entry point
├── frontend/             # React frontend
│   ├── src/
│   │   ├── App.jsx       # Main React component
│   │   └── App.css       # Styling
└── logs/                 # Log files directory (created automatically)
    └── snaplog.txt       # Text log file
```

## Security Notes

- Uses native Windows `RegisterHotKey` API (no third-party keylogging libraries)
- Only captures the specific hotkey combination (`Ctrl+Shift+L`)
- Does not capture or store any other keyboard input
- All data stays local on your machine

## Future Enhancements

- macOS and Linux support
- Configurable hotkey combinations
- Multiple log file formats
- Search and filter functionality
- Auto-hide after logging option
