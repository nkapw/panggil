# install.ps1 - Windows installation script for panggil
# Run: irm https://raw.githubusercontent.com/nkapw/panggil/main/install.ps1 | iex
# Or: .\install.ps1

$ErrorActionPreference = "Stop"

$Repo = "nkapw/panggil"
$AppName = "panggil"
$InstallDir = "$env:LOCALAPPDATA\Programs\$AppName"

function Write-Info { param($Message) Write-Host "[INFO] $Message" -ForegroundColor Cyan }
function Write-Success { param($Message) Write-Host "[SUCCESS] $Message" -ForegroundColor Green }
function Write-Error { param($Message) Write-Host "[ERROR] $Message" -ForegroundColor Red; exit 1 }

Write-Info "Starting installation of '$AppName'..."

# Detect architecture
$Arch = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { "386" }
Write-Info "Detected System: windows/$Arch"

# Get latest version
Write-Info "Fetching the latest version..."
try {
    $Release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
    $LatestTag = $Release.tag_name
    $LatestVersion = $LatestTag -replace '^v', ''
} catch {
    Write-Error "Could not fetch the latest version from GitHub: $_"
}

Write-Info "Latest version is $LatestTag"

# Download binary
$BinaryName = "${AppName}_${LatestVersion}_windows_${Arch}.exe"
$DownloadUrl = "https://github.com/$Repo/releases/download/$LatestTag/$BinaryName"

Write-Info "Downloading from: $DownloadUrl"
$TempFile = "$env:TEMP\$AppName.exe"

try {
    Invoke-WebRequest -Uri $DownloadUrl -OutFile $TempFile -UseBasicParsing
} catch {
    Write-Error "Failed to download binary: $_"
}

Write-Info "Binary downloaded to $TempFile"

# Create install directory
if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

# Move binary
$DestPath = "$InstallDir\$AppName.exe"
Move-Item -Path $TempFile -Destination $DestPath -Force

Write-Info "Installed to $DestPath"

# Add to PATH if not already there
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$InstallDir*") {
    Write-Info "Adding $InstallDir to PATH..."
    [Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", "User")
    $env:Path = "$env:Path;$InstallDir"
    Write-Info "Please restart your terminal for PATH changes to take effect"
}

Write-Success "'$AppName' installed successfully!"
Write-Host ""
Write-Host "Run '$AppName' to start the application." -ForegroundColor Yellow
Write-Host "If command not found, restart your terminal first." -ForegroundColor Yellow
