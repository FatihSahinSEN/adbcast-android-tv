#!/bin/bash
# ADB TV Bridge Connector Server - Linux Systemd Service Uninstaller

set -e

if [ "$EUID" -ne 0 ]; then
  echo "[-] Please run as root (e.g. sudo ./uninstall-service-linux.sh)"
  exit 1
fi

SERVICE_NAME="tvbridge.service"

echo "[*] Stopping and disabling $SERVICE_NAME..."
systemctl stop $SERVICE_NAME 2>/dev/null || true
systemctl disable $SERVICE_NAME 2>/dev/null || true

if [ -f "/etc/systemd/system/$SERVICE_NAME" ]; then
  rm -f "/etc/systemd/system/$SERVICE_NAME"
  systemctl daemon-reload
  echo "[+] Service $SERVICE_NAME uninstalled successfully."
else
  echo "[!] Service $SERVICE_NAME was not found."
fi
