#!/usr/bin/env bash
set -euo pipefail

APP_NAME="snaplog"
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}" )/.." && pwd)"
BIN_DIR="$PROJECT_ROOT/build/bin"

if [[ ! -d "$BIN_DIR" ]]; then
  echo "Error: build/bin directory not found. Run 'wails build' first." >&2
  exit 1
fi

APP_PATH="$(find "$BIN_DIR" -maxdepth 3 -type d -name "${APP_NAME}.app" | head -n 1)"

if [[ -z "$APP_PATH" ]]; then
  echo "Error: Could not locate ${APP_NAME}.app inside build/bin." >&2
  exit 1
fi

PLIST="$APP_PATH/Contents/Info.plist"

if [[ ! -f "$PLIST" ]]; then
  echo "Error: Info.plist not found at $PLIST" >&2
  exit 1
fi

if /usr/bin/plutil -replace LSUIElement -bool true "$PLIST"; then
  echo "Set LSUIElement=true using plutil"
else
  echo "plutil failed, applying awk fallback" >&2
  TMP_FILE="${PLIST}.tmp"
  awk '
    { lines[++n] = $0 }
    END {
      found = 0
      for (i = 1; i <= n; i++) {
        line = lines[i]
        if (!found && line ~ /<key>LSUIElement<\/key>/) {
          found = 1
          print line
          if (i + 1 <= n && lines[i + 1] ~ /<true\/>|<false\/>/) {
            print "    <true/>"
            i++
          } else {
            print "    <true/>"
          }
          for (j = i + 1; j <= n; j++) {
            print lines[j]
          }
          exit
        }
      }
      inserted = 0
      for (i = 1; i <= n; i++) {
        line = lines[i]
        if (!inserted && line ~ /<\/dict>/) {
          print "    <key>LSUIElement</key>"
          print "    <true/>"
          inserted = 1
        }
        print line
      }
      if (!inserted) {
        print "    <key>LSUIElement</key>"
        print "    <true/>"
      }
    }
  ' "$PLIST" > "$TMP_FILE"
  mv "$TMP_FILE" "$PLIST"
fi
