#!/bin/bash
# Post-build script to add LSUIElement to Info.plist (hides app from Dock)
#
# This script modifies the built macOS app to hide it from the Dock.
# Note: Modifying Info.plist invalidates code signing. For development this is fine.
# For distribution, you may need to re-sign the app.


APP_BUNDLE="build/bin/snaplog.app"
INFO_PLIST="${APP_BUNDLE}/Contents/Info.plist"


if [ ! -f "$INFO_PLIST" ]; then
   echo "Error: Info.plist not found at $INFO_PLIST"
   echo "Make sure you've run 'wails build' first"
   exit 1
fi


# Check if LSUIElement already exists
if grep -q "LSUIElement" "$INFO_PLIST"; then
   echo "LSUIElement already exists in Info.plist"
   exit 0
fi


# Use plutil (macOS's native plist tool) if available - more reliable
if command -v plutil &> /dev/null; then
   # Convert to binary plist, add key, convert back
   plutil -convert binary1 "$INFO_PLIST" 2>/dev/null
   plutil -insert LSUIElement -bool true "$INFO_PLIST" 2>/dev/null
   plutil -convert xml1 "$INFO_PLIST" 2>/dev/null
  
   if grep -q "LSUIElement" "$INFO_PLIST"; then
       echo "✓ Added LSUIElement using plutil (app will be hidden from Dock)"
       exit 0
   fi
fi


# Fallback: Use awk if plutil didn't work
echo "Using fallback method (awk)..."
TEMP_FILE=$(mktemp)


# Use awk to insert LSUIElement before NSHumanReadableCopyright or before closing </dict>
if grep -q "NSHumanReadableCopyright" "$INFO_PLIST"; then
   awk '/<key>NSHumanReadableCopyright<\/key>/ {print "        <key>LSUIElement</key>"; print "        <true/>"} {print}' "$INFO_PLIST" > "$TEMP_FILE"
else
   awk '/<\/dict>/ {print "        <key>LSUIElement</key>"; print "        <true/>"} {print}' "$INFO_PLIST" > "$TEMP_FILE"
fi


# Replace the original file
mv "$TEMP_FILE" "$INFO_PLIST"


if grep -q "LSUIElement" "$INFO_PLIST"; then
   echo "✓ Added LSUIElement to Info.plist (app will be hidden from Dock)"
else
   echo "✗ Failed to add LSUIElement"
   exit 1
fi