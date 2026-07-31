#!/bin/bash
# ADBCast Android TV Server - macOS Launchd Service Uninstaller

PLIST_LABEL="com.adbyc.tvbridge"
PLIST_PATH="$HOME/Library/LaunchAgents/${PLIST_LABEL}.plist"

echo "[*] Stopping and unloading macOS service..."
launchctl unload "$PLIST_PATH" 2>/dev/null || true
rm -f "$PLIST_PATH"

echo "[+] ADBCast Service successfully removed from macOS startup."
