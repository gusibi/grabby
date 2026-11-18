#!/usr/bin/env bash
# 一键安装 Grabby 本地音视频转文字依赖：ffmpeg、yt-dlp、mlx-whisper（Apple Silicon）或 openai-whisper
set -euo pipefail

export PATH="/opt/homebrew/bin:/usr/local/bin:$HOME/.local/bin:$PATH"

log() { printf '[grabby-asr] %s\n' "$*"; }
has_cmd() { command -v "$1" >/dev/null 2>&1; }

OS="$(uname -s)"
ARCH="$(uname -m)"

log "系统环境: $OS ($ARCH)"

mkdir -p "$HOME/.local/bin"

install_brew_pkg() {
  local pkg="$1"
  local bin_name="${2:-$1}"
  if has_cmd "$bin_name"; then
    log "$bin_name 已安装: $(command -v "$bin_name")"
    return 0
  fi
  if has_cmd brew; then
    log "通过 Homebrew 安装 $pkg ..."
    brew install "$pkg"
    return 0
  fi
  return 1
}

# 1. ffmpeg
if ! install_brew_pkg ffmpeg ffmpeg; then
  if has_cmd apt-get; then
    log "尝试通过 apt-get 安装 ffmpeg ..."
    sudo apt-get update && sudo apt-get install -y ffmpeg
  elif has_cmd dnf; then
    sudo dnf install -y ffmpeg
  elif has_cmd pacman; then
    sudo pacman -S --noconfirm ffmpeg
  else
    log "警告: 未检测到 brew/apt/dnf/pacman，请手动安装 ffmpeg"
  fi
fi

# 2. yt-dlp
if ! install_brew_pkg yt-dlp yt-dlp; then
  if has_cmd uv; then
    log "通过 uv 安装 yt-dlp ..."
    uv tool install yt-dlp
  elif has_cmd pipx; then
    log "通过 pipx 安装 yt-dlp ..."
    pipx install yt-dlp
  elif has_cmd pip3; then
    log "通过 pip3 安装 yt-dlp ..."
    pip3 install --user --break-system-packages yt-dlp 2>/dev/null || pip3 install --user yt-dlp
  else
    log "警告: 无法自动安装 yt-dlp，请手动安装"
  fi
fi

# 3. Whisper (Apple Silicon 优先 mlx-whisper，其余平台 openai-whisper)
install_python_cli() {
  local pkg="$1"
  local bin_name="$2"
  if has_cmd "$bin_name"; then
    log "$bin_name 已安装: $(command -v "$bin_name")"
    return 0
  fi
  if has_cmd uv; then
    log "通过 uv tool 安装 $pkg ..."
    uv tool install "$pkg" && return 0
  fi
  if has_cmd pipx; then
    log "通过 pipx 安装 $pkg ..."
    pipx install "$pkg" && return 0
  fi
  if has_cmd pip3; then
    log "通过 pip3 安装 $pkg ..."
    pip3 install --user --break-system-packages "$pkg" 2>/dev/null || pip3 install --user "$pkg"
    return 0
  fi
  log "警告: 未找到 uv/pipx/pip3，无法自动安装 $pkg"
  return 1
}

if [[ "$OS" == "Darwin" && "$ARCH" == "arm64" ]]; then
  install_python_cli "mlx-whisper" "mlx_whisper" || true
else
  install_python_cli "openai-whisper" "whisper" || true
fi

log "=== 检查结果 ==="
for tool in ffmpeg yt-dlp mlx_whisper whisper; do
  if has_cmd "$tool"; then
    log "  [OK] $tool -> $(command -v "$tool")"
  else
    log "  [--] $tool (未安装)"
  fi
done
