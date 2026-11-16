# Grabby CLI installer for Windows
# Usage: irm https://raw.githubusercontent.com/gusibi/grabby/main/scripts/install.ps1 | iex
#
# Installs the latest grabby.exe from GitHub Releases and adds it to the user PATH.

$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"

$Repo = "gusibi/grabby"
$LocalAppData = if ($env:LOCALAPPDATA) { $env:LOCALAPPDATA } else { Join-Path $env:USERPROFILE "AppData\Local" }
$InstallDir = if ($env:GRABBY_INSTALL_DIR) {
  $env:GRABBY_INSTALL_DIR
} else {
  Join-Path $LocalAppData "Programs\grabby"
}

# 64-bit ARM and x64 are the release targets. On 32-bit PowerShell running on
# a 64-bit OS, PROCESSOR_ARCHITEW6432 holds the real architecture.
$IsArm = ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") -or ($env:PROCESSOR_ARCHITEW6432 -eq "ARM64")
$Arch = if ($IsArm) { "arm64" } else { "x86_64" }
$Asset = "grabby_Windows_${Arch}.zip"
$Url = "https://github.com/$Repo/releases/latest/download/$Asset"

Write-Host "⬇️  Downloading latest grabby (Windows/$Arch)..."
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null

$Zip = Join-Path $env:TEMP ("grabby-" + [guid]::NewGuid().ToString() + ".zip")
try {
  Invoke-WebRequest -Uri $Url -OutFile $Zip -UseBasicParsing
  Expand-Archive -Path $Zip -DestinationPath $InstallDir -Force
} finally {
  Remove-Item $Zip -Force -ErrorAction SilentlyContinue
}

$Exe = Join-Path $InstallDir "grabby.exe"
if (-not (Test-Path $Exe)) {
  throw "安装包中没有找到 grabby.exe：$Exe"
}

# Add the install directory to the user PATH (not the machine PATH).
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
$Parts = @()
if ($UserPath) {
  $Parts = $UserPath.Split(";", [System.StringSplitOptions]::RemoveEmptyEntries)
}
if ($Parts -notcontains $InstallDir) {
  $NewPath = (@($Parts) + $InstallDir) -join ";"
  [Environment]::SetEnvironmentVariable("Path", $NewPath, "User")
  Write-Host ""
  Write-Host "⚠️  已把 $InstallDir 加入用户 PATH。"
  Write-Host "    请重新打开终端（或重新登录）后再运行 grabby。"
}

Write-Host ""
Write-Host "✅ 安装完成: $Exe"
Write-Host "📝 配置文件目录: $env:USERPROFILE\.grabby"
Write-Host "🚀 重新打开终端后运行: grabby --version"
