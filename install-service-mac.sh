#!/bin/bash
# ADBCast Android TV Server - macOS Launchd Daemon Service Installer

set -e

INSTALL_DIR="$HOME/Library/Application Support/ADBCast"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PLIST_LABEL="com.adbyc.tvbridge"
PLIST_PATH="$HOME/Library/LaunchAgents/${PLIST_LABEL}.plist"

echo "=========================================="
echo "  ADBCast Android TV Server - macOS       "
echo "=========================================="

echo "[*] Creating installation directory: $INSTALL_DIR"
mkdir -p "$INSTALL_DIR"

# Build Go binary if missing
if [ ! -f "$SCRIPT_DIR/adb-connector-server" ]; then
  echo "[*] Building Go server binary..."
  (cd "$SCRIPT_DIR" && go build -o adb-connector-server main.go)
fi

echo "[*] Copying binary to $INSTALL_DIR..."
cp -r "$SCRIPT_DIR/adb-connector-server" "$INSTALL_DIR/"
cp -r "$SCRIPT_DIR/config.json" "$INSTALL_DIR/" 2>/dev/null || true

echo "[*] Creating launchd plist file at $PLIST_PATH..."
mkdir -p "$HOME/Library/LaunchAgents"

cat << EOF > "$PLIST_PATH"
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>${PLIST_LABEL}</string>
    <key>ProgramArguments</key>
    <array>
        <string>${INSTALL_DIR}/adb-connector-server</string>
    </array>
    <key>WorkingDirectory</key>
    <string>${INSTALL_DIR}</string>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>${INSTALL_DIR}/server.log</string>
    <key>StandardErrorPath</key>
    <string>${INSTALL_DIR}/server_err.log</string>
</dict>
</plist>
EOF

echo "[*] Loading and starting $PLIST_LABEL..."
launchctl unload "$PLIST_PATH" 2>/dev/null || true
launchctl load -w "$PLIST_PATH"

echo "[+] Service successfully installed and started on macOS startup!"
