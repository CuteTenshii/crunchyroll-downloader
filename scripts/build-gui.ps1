# Build the Wails desktop GUI with the required tags.
# Usage (from repo root):
#   .\scripts\build-gui.ps1
#   .\scripts\build-gui.ps1 -Out "dist\crunchyroll-downloader-gui.exe"

param(
    [string]$Out = "crunchyroll-downloader-gui.exe"
)

$ErrorActionPreference = "Stop"

$goBin = "C:\Program Files\Go\bin"
if (Test-Path $goBin) {
    $env:Path = $goBin + ";" + $env:USERPROFILE + "\go\bin;" + $env:Path
}

$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

Write-Host "Building GUI -> $Out (tags: desktop,production)"
go build -tags "desktop,production" -ldflags "-w -s -H windowsgui" -o $Out ./cmd/gui
if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
}

$item = Get-Item $Out
Write-Host ("OK: {0} ({1:N1} MB)" -f $item.FullName, ($item.Length / 1MB))
Write-Host "Run: .\$Out"
