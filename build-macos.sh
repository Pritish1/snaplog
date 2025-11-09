#!/bin/bash
# Build script for macOS that hides app from Dock

echo "Building SnapLog..."

# Check if wails is available
if ! command -v wails &> /dev/null; then
   echo "Error: wails command not found. Make sure Wails is installed and in your PATH."
   exit 1
fi

wails build

if [ $? -eq 0 ]; then
   echo ""
   echo "Adding LSUIElement to hide from Dock..."
   ./scripts/postbuild-macos.sh
   echo ""
   echo "✓ Build complete! App is hidden from Dock (menu bar only)"
else
   echo "Build failed!"
   exit 1
fi
