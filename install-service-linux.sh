#!/bin/bash
# ADB TV Bridge Connector Server - Linux Systemd Service Installer

set -e

if [ "$EUID" -ne 0 ]; then
  echo "[-] Please run as root (e.g. sudo ./install-service-linux.sh)"
  exit 1
fi

INSTALL_DIR="/opt/tvbridge"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICE_NAME="tvbridge.service"

echo "=========================================="
echo "  ADBCast Android TV Server - Linux Systemd    "
echo "=========================================="


echo "[*] Creating installation directory: $INSTALL_DIR"
mkdir -p "$INSTALL_DIR"

# Build Go binary if missing
if [ ! -f "$SCRIPT_DIR/adb-connector-server" ]; then
  echo "[*] Building Go server binary..."
  (cd "$SCRIPT_DIR" && go build -o adb-connector-server main.go)
fi

echo "[*] Copying binary and platform tools to $INSTALL_DIR..."
cp -r "$SCRIPT_DIR/adb-connector-server" "$INSTALL_DIR/"
cp -r "$SCRIPT_DIR/config.json" "$INSTALL_DIR/" 2>/dev/null || true
if [ -d "$SCRIPT_DIR/platform-tools-linux64" ]; then
  cp -r "$SCRIPT_DIR/platform-tools-linux64" "$INSTALL_DIR/"
fi

echo "[*] Creating systemd service file..."
cat << EOF > /etc/systemd/system/$SERVICE_NAME
[Unit]
Description=ADBCast Android TV Server
After=network.target network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/adb-connector-server
Restart=always
RestartSec=3s
User=root

[Install]
WantedBy=multi-user.target
EOF

chmod 644 /etc/systemd/system/$SERVICE_NAME

echo "[*] Enabling and starting $SERVICE_NAME..."
systemctl daemon-reload
systemctl enable $SERVICE_NAME
systemctl restart $SERVICE_NAME

echo "[+] Service successfully installed and started!"
systemctl status $SERVICE_NAME --no-pager
