# ADBCast Android TV - Windows Service Installer (PowerShell)
# Requires Administrator Privileges

$ErrorActionPreference = "Stop"

# Check Admin Privileges
$currentPrincipal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
if (-not $currentPrincipal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Write-Host "[!] Requesting Administrator elevation..." -ForegroundColor Yellow
    Start-Process powershell.exe "-NoProfile -ExecutionPolicy Bypass -File `"$PSCommandPath`"" -Verb RunAs
    exit
}

$TaskName = "ADBTVBridgeConnector"
$WorkingDir = $PSScriptRoot
$ExePath = Join-Path $WorkingDir "adb-connector-server.exe"

Write-Host "==========================================" -ForegroundColor Cyan
Write-Host "  ADBCast Android TV Server - Windows Service  " -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan

# Check Exe Existence or Build
if (-not (Test-Path $ExePath)) {
    Write-Host "[*] adb-connector-server.exe not found. Building Go binary..." -ForegroundColor Yellow
    Push-Location $WorkingDir
    go build -o adb-connector-server.exe main.go
    Pop-Location
    if (-not (Test-Path $ExePath)) {
        Write-Error "[-] Failed to build adb-connector-server.exe"
        exit 1
    }
}

# Remove old task if exists
$existingTask = Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
if ($existingTask) {
    Write-Host "[*] Removing existing task '$TaskName'..." -ForegroundColor Yellow
    Stop-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
    Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false
}

# Create Scheduled Task for Auto-Start on Boot
Write-Host "[*] Creating startup background service '$TaskName'..." -ForegroundColor Green

$Action = New-ScheduledTaskAction -Execute $ExePath -WorkingDirectory $WorkingDir
$Trigger = New-ScheduledTaskTrigger -AtStartup
$Principal = New-ScheduledTaskPrincipal -UserId "SYSTEM" -LogonType ServiceAccount -RunLevel Highest
$Settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1) -ExecutionTimeLimit 0

$Task = New-ScheduledTask -Action $Action -Trigger $Trigger -Principal $Principal -Settings $Settings
Register-ScheduledTask -TaskName $TaskName -InputObject $Task | Out-Null

Write-Host "[+] Service successfully installed and set to auto-start on Windows boot!" -ForegroundColor Green

# Start Task Immediately
Write-Host "[*] Starting service now..." -ForegroundColor Green
Start-ScheduledTask -TaskName $TaskName

Write-Host "[+] ADBCast Android TV Server is running in the background." -ForegroundColor Green
Pause
