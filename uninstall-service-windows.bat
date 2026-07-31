@echo off
:: ADB TV Bridge Connector - Windows Service Uninstaller (Batch Launcher)
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0uninstall-service-windows.ps1"
