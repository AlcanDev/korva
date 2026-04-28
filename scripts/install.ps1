# Korva - Instalador para Windows
# Uso: iwr -useb https://raw.githubusercontent.com/AlcanDev/korva/main/scripts/install.ps1 | iex
# O manualmente: .\scripts\install.ps1

param(
    [string]$Version = "latest",
    [string]$InstallDir = "$env:LOCALAPPDATA\korva\bin"
)

$ErrorActionPreference = "Stop"

$Repo = "AlcanDev/korva"
$BaseUrl = "https://github.com/$Repo/releases"

Write-Host ""
Write-Host "  ██╗  ██╗ ██████╗ ██████╗ ██╗   ██╗ █████╗ " -ForegroundColor Cyan
Write-Host "  ██║ ██╔╝██╔═══██╗██╔══██╗██║   ██║██╔══██╗" -ForegroundColor Cyan
Write-Host "  █████╔╝ ██║   ██║██████╔╝██║   ██║███████║" -ForegroundColor Cyan
Write-Host "  ██╔═██╗ ██║   ██║██╔══██╗╚██╗ ██╔╝██╔══██║" -ForegroundColor Cyan
Write-Host "  ██║  ██╗╚██████╔╝██║  ██║ ╚████╔╝ ██║  ██║" -ForegroundColor Cyan
Write-Host "  ╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═╝  ╚═══╝  ╚═╝  ╚═╝" -ForegroundColor Cyan
Write-Host ""
Write-Host "  Instalador de Korva para Windows" -ForegroundColor White
Write-Host ""

# --- Detectar arquitectura ---
# GoReleaser uses lowercase "windows" and skips arm64 for Windows
$arch = switch ($env:PROCESSOR_ARCHITECTURE) {
    "AMD64" { "amd64" }
    "ARM64" { "arm64" }
    default { "amd64" }  # fallback for x86 on 64-bit
}
$os = "windows"

Write-Host "[1/4] Detectando version..." -ForegroundColor Yellow

# --- Obtener version ---
if ($Version -eq "latest") {
    try {
        $releaseInfo = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -UseBasicParsing
        $Version = $releaseInfo.tag_name.TrimStart("v")
    } catch {
        Write-Error "No se pudo obtener la version mas reciente. Verifica tu conexion a internet."
        exit 1
    }
}

Write-Host "  Version: v$Version ($os/$arch)" -ForegroundColor Green

# --- Crear directorio de instalacion ---
Write-Host "[2/4] Preparando directorio de instalacion: $InstallDir" -ForegroundColor Yellow

if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

# --- Descargar binarios ---
Write-Host "[3/4] Descargando binarios..." -ForegroundColor Yellow

# GoReleaser uses .zip for Windows archives
$ArchiveName = "korva_${Version}_${os}_${arch}.zip"
$DownloadUrl  = "$BaseUrl/download/v$Version/$ArchiveName"
$TmpDir       = Join-Path $env:TEMP "korva_install_$([System.Guid]::NewGuid().ToString('N').Substring(0,8))"
$TmpZip       = Join-Path $TmpDir $ArchiveName

New-Item -ItemType Directory -Path $TmpDir -Force | Out-Null

try {
    Write-Host "  Descargando $DownloadUrl..." -ForegroundColor Gray
    $ProgressPreference = "SilentlyContinue"  # faster download
    Invoke-WebRequest -Uri $DownloadUrl -OutFile $TmpZip -UseBasicParsing
} catch {
    Write-Error "Error descargando Korva v$Version. Verifica que la version exista en: $BaseUrl"
    Remove-Item -Recurse -Force $TmpDir -ErrorAction SilentlyContinue
    exit 1
}

# --- Extraer ---
Write-Host "  Extrayendo archivos..." -ForegroundColor Gray

try {
    Expand-Archive -Path $TmpZip -DestinationPath $TmpDir -Force
} catch {
    Write-Error "Error extrayendo el archivo zip."
    Remove-Item -Recurse -Force $TmpDir -ErrorAction SilentlyContinue
    exit 1
}

# --- Copiar binarios ---
$binaries = @("korva.exe", "korva-vault.exe", "korva-sentinel.exe")
foreach ($bin in $binaries) {
    $src = Join-Path $TmpDir $bin
    if (Test-Path $src) {
        Copy-Item -Path $src -Destination $InstallDir -Force
        Write-Host "  Instalado: $bin" -ForegroundColor Green
    } else {
        Write-Warning "  No encontrado en el archivo: $bin"
    }
}

# Limpiar temporales
Remove-Item -Recurse -Force $TmpDir -ErrorAction SilentlyContinue

# --- Configurar PATH ---
Write-Host "[4/4] Configurando PATH..." -ForegroundColor Yellow

$currentPath = [System.Environment]::GetEnvironmentVariable("PATH", "User")
if ($currentPath -notlike "*$InstallDir*") {
    [System.Environment]::SetEnvironmentVariable(
        "PATH",
        "$currentPath;$InstallDir",
        "User"
    )
    Write-Host "  PATH actualizado. Reinicia tu terminal para aplicar los cambios." -ForegroundColor Yellow
} else {
    Write-Host "  PATH ya contiene $InstallDir" -ForegroundColor Green
}

# --- Verificar instalacion ---
Write-Host ""
Write-Host "Verificando instalacion..." -ForegroundColor Yellow

$kPath = Join-Path $InstallDir "korva.exe"
if (Test-Path $kPath) {
    try {
        $ver = & $kPath version 2>&1
        Write-Host ""
        Write-Host "  Korva instalado exitosamente!" -ForegroundColor Green
        Write-Host "  $ver" -ForegroundColor Gray
    } catch {
        Write-Host "  Binarios instalados. Reinicia tu terminal y ejecuta: korva version" -ForegroundColor Green
    }
} else {
    Write-Warning "  No se encontro korva.exe en $InstallDir"
}

Write-Host ""
Write-Host "Siguientes pasos:" -ForegroundColor Cyan
Write-Host "  1. Reinicia tu terminal (PowerShell o CMD)"
Write-Host "  2. Ejecuta: korva init"
Write-Host "  3. Para configurar tu equipo: korva init --profile <url-del-profile>"
Write-Host ""
Write-Host "Documentacion: https://github.com/AlcanDev/korva" -ForegroundColor Gray
Write-Host ""
