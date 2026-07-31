# ADB TV Bridge Connector - Windows Service Uninstaller (PowerShell)

$ErrorActionPreference = "Stop"

$currentPrincipal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
if (-not $currentPrincipal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Write-Host "[!] Requesting Administrator elevation..." -ForegroundColor Yellow
    Start-Process powershell.exe "-NoProfile -ExecutionPolicy Bypass -File `"$PSCommandPath`"" -Verb RunAs
    exit
}

$TaskName = "ADBTVBridgeConnector"

Write-Host "[*] Stopping and removing '$TaskName'..." -ForegroundColor Yellow

$existingTask = Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
if ($existingTask) {
    Stop-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
    Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false
    Write-Host "[+] Service successfully removed." -ForegroundColor Green
} else {
    Write-Host "[!] Service '$TaskName' was not found." -ForegroundColor Yellow
}
Pause
