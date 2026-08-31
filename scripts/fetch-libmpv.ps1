# Download official Windows libmpv (shinchiro mpv-dev) next to the GUI exe.
# Usage (from repo root):
#   .\scripts\fetch-libmpv.ps1
param(
    [string]$DestDir = ""
)

$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
if (-not $DestDir) {
    $DestDir = $root
}

$outDll = Join-Path $DestDir "libmpv-2.dll"
if (Test-Path $outDll) {
    Write-Host "libmpv already present: $outDll"
    exit 0
}

$seven = @(
    "${env:ProgramFiles}\7-Zip\7z.exe",
    "${env:ProgramFiles(x86)}\7-Zip\7z.exe"
) | Where-Object { Test-Path $_ } | Select-Object -First 1

$work = Join-Path $env:TEMP ("crdl-libmpv-" + [guid]::NewGuid().ToString("n"))
New-Item -ItemType Directory -Path $work | Out-Null
try {
    if (-not $seven) {
        $seven = Join-Path $work "7zr.exe"
        Write-Host "Downloading 7zr.exe"
        Invoke-WebRequest -Uri "https://www.7-zip.org/a/7zr.exe" -OutFile $seven -UseBasicParsing
    }

    $archive = $null
    $gh = Get-Command gh -ErrorAction SilentlyContinue
    if ($gh) {
        Write-Host "Downloading latest shinchiro mpv-dev x86_64 via gh"
        gh release download -R shinchiro/mpv-winbuild-cmake --pattern "mpv-dev-x86_64-20*.7z" --dir $work --clobber
        $archive = Get-ChildItem $work -Filter "mpv-dev-x86_64-*.7z" | Select-Object -First 1
        if ($archive) { $archive = $archive.FullName }
    }
    if (-not $archive) {
        Write-Host "Resolving latest shinchiro mpv-dev x86_64"
        $rel = Invoke-RestMethod -Uri "https://api.github.com/repos/shinchiro/mpv-winbuild-cmake/releases/latest"
        $asset = $rel.assets | Where-Object { $_.name -match '^mpv-dev-x86_64-\d{8}-git-' } | Select-Object -First 1
        if (-not $asset) {
            throw "no mpv-dev-x86_64 asset on latest release"
        }
        $archive = Join-Path $work $asset.name
        Write-Host "Downloading $($asset.name)"
        Invoke-WebRequest -Uri $asset.browser_download_url -OutFile $archive -UseBasicParsing
    }

    $extract = Join-Path $work "extract"
    New-Item -ItemType Directory -Path $extract | Out-Null
    & $seven x "-o$extract" -y $archive | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "7-Zip extract failed"
    }

    $found = Get-ChildItem -Path $extract -Filter "libmpv-2.dll" -Recurse | Select-Object -First 1
    if (-not $found) {
        throw "archive did not contain libmpv-2.dll"
    }
    Copy-Item $found.FullName $outDll -Force
    Write-Host "Installed $outDll ($([math]::Round($found.Length / 1MB, 1)) MB)"
    Write-Host "libmpv is GPL-2.0-or-later; see https://mpv.io/"
}
finally {
    Remove-Item $work -Recurse -Force -ErrorAction SilentlyContinue
}
