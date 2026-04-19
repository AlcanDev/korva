# Korva Windows Installer — https://korva.dev/install.ps1
#
# Usage (run as Administrator for system-wide install):
#   irm https://korva.dev/install.ps1 | iex
#
# Options (environment variables):
#   $env:KORVA_VERSION      Pin a specific version, e.g. v0.3.0  (default: latest)
#   $env:KORVA_INSTALL_DIR  Override install directory            (default: %LOCALAPPDATA%\korva\bin)
#   $env:KORVA_NO_VAULT     Set to "yes" to skip korva-vault      (default: install it)
#
#Requires -Version 5.1
Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$Repo    = "alcandev/korva"
$Version = if ($env:KORVA_VERSION) { $env:KORVA_VERSION } else { "latest" }
$NoVault = $env:KORVA_NO_VAULT -eq "yes"

# ─── Detect architecture ──────────────────────────────────────────────────────

$Arch = switch ($env:PROCESSOR_ARCHITECTURE) {
    "AMD64"   { "amd64" }
    "ARM64"   { "arm64" }
    "x86"     {
        # Could be 32-bit on 64-bit OS — check via WMI
        $cpu = (Get-WmiObject Win32_Processor).AddressWidth
        if ($cpu -eq 64) { "amd64" } else {
            Write-Error "32-bit x86 is not supported. Download from: https://github.com/$Repo/releases"
            exit 1
        }
    }
    default {
        Write-Error "Unsupported architecture: $env:PROCESSOR_ARCHITECTURE"
        exit 1
    }
}

# ─── Resolve version ─────────────────────────────────────────────────────────

if ($Version -eq "latest") {
    Write-Host "  -> Fetching latest release..." -NoNewline
    try {
        $release = Invoke-RestMethod `
            -Uri "https://api.github.com/repos/$Repo/releases/latest" `
            -Headers @{ "User-Agent" = "korva-installer/1.0" }
        $Version = $release.tag_name
        Write-Host " $Version"
    } catch {
        Write-Error "Could not determine latest version. Set `$env:KORVA_VERSION explicitly."
        exit 1
    }
}

$VersionClean = $Version.TrimStart("v")
$Archive      = "korva_${VersionClean}_windows_${Arch}.zip"
$DownloadURL  = "https://github.com/$Repo/releases/download/$Version/$Archive"

# ─── Resolve install directory ───────────────────────────────────────────────

$InstallDir = if ($env:KORVA_INSTALL_DIR) {
    $env:KORVA_INSTALL_DIR
} else {
    Join-Path $env:LOCALAPPDATA "korva\bin"
}

if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

# ─── Download & extract ──────────────────────────────────────────────────────

$TmpDir  = Join-Path ([System.IO.Path]::GetTempPath()) "korva-install-$([guid]::NewGuid())"
$ZipFile = Join-Path $TmpDir $Archive

New-Item -ItemType Directory -Path $TmpDir -Force | Out-Null
try {
    Write-Host "  -> Downloading Korva $Version (windows/$Arch)..."
    $ProgressPreference = "SilentlyContinue"   # hide progress bar — much faster
    Invoke-WebRequest -Uri $DownloadURL -OutFile $ZipFile -UseBasicParsing

    Write-Host "  -> Extracting..."
    Expand-Archive -Path $ZipFile -DestinationPath $TmpDir -Force
} catch {
    Write-Error "Download failed: $_"
    Remove-Item $TmpDir -Recurse -Force -ErrorAction SilentlyContinue
    exit 1
}

# ─── Install binaries ────────────────────────────────────────────────────────

$Installed = @()

foreach ($bin in @("korva.exe", "korva-sentinel.exe")) {
    $src = Join-Path $TmpDir $bin
    if (Test-Path $src) {
        Copy-Item -Path $src -Destination (Join-Path $InstallDir $bin) -Force
        $Installed += $bin
    }
}

if (-not $NoVault) {
    $src = Join-Path $TmpDir "korva-vault.exe"
    if (Test-Path $src) {
        Copy-Item -Path $src -Destination (Join-Path $InstallDir "korva-vault.exe") -Force
        $Installed += "korva-vault.exe"
    }
}

Remove-Item $TmpDir -Recurse -Force -ErrorAction SilentlyContinue

if ($Installed.Count -eq 0) {
    Write-Error "No binaries found in archive. Check the release at: https://github.com/$Repo/releases/tag/$Version"
    exit 1
}

# ─── Update PATH ─────────────────────────────────────────────────────────────

$CurrentPath = [System.Environment]::GetEnvironmentVariable("Path", "User")
if ($CurrentPath -notlike "*$InstallDir*") {
    $NewPath = "$CurrentPath;$InstallDir"
    [System.Environment]::SetEnvironmentVariable("Path", $NewPath, "User")
    $PathUpdated = $true
} else {
    $PathUpdated = $false
}

# Also update the current session's PATH so the binaries are immediately usable
$env:Path = "$env:Path;$InstallDir"

# ─── Done ────────────────────────────────────────────────────────────────────

Write-Host ""
Write-Host "  + Korva $Version installed to $InstallDir" -ForegroundColor Green
Write-Host "    Binaries: $($Installed -join ', ')"
Write-Host ""

if ($PathUpdated) {
    Write-Host "  PATH updated for your user account." -ForegroundColor Cyan
    Write-Host "  Restart any open terminals to pick up the change,"
    Write-Host "  or run: `$env:Path = [System.Environment]::GetEnvironmentVariable('Path','User')"
    Write-Host ""
}

Write-Host "  Quick start:"
Write-Host "    korva init               # initialise project"
Write-Host "    korva vault start        # start the local vault server"
Write-Host "    korva doctor             # verify your setup"
Write-Host ""
Write-Host "  Docs: https://korva.dev/docs"
