#!/usr/bin/env bash
set -euo pipefail

# Grabby CLI installer (macOS / Linux; Windows users should use install.ps1)
# Usage: curl -fsSL https://raw.githubusercontent.com/gusibi/grabby/main/scripts/install.sh | bash
#
# 安装最新 Release 里的 grabby 二进制。
# 默认安装到 ~/.local/bin/grabby；配置文件目录 ~/.grabby/。

REPO="gusibi/grabby"
BINARY="grabby"
CONFIG_DIR="${HOME}/.grabby"

# Detect OS and ARCH
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$OS" in
  darwin) OS="Darwin" ;;
  linux)  OS="Linux" ;;
  mingw*|msys*|cygwin*) OS="Windows" ;;
  *)
    echo "❌ 不支持的操作系统: $OS"
    exit 1
    ;;
esac

case "$ARCH" in
  x86_64|amd64) ARCH="x86_64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *)
    echo "❌ 不支持的架构: $ARCH"
    exit 1
    ;;
esac

if [ "$OS" = "Windows" ]; then
  EXT="zip"
  BINARY_NAME="grabby.exe"
  INSTALL_DIR="${GRABBY_INSTALL_DIR:-${LOCALAPPDATA:-$HOME/AppData/Local}/Programs/grabby}"
else
  EXT="tar.gz"
  BINARY_NAME="grabby"
  INSTALL_DIR="${GRABBY_INSTALL_DIR:-$HOME/.local/bin}"
fi

if ! command -v curl &>/dev/null; then
  echo "❌ 需要 curl，请先安装"
  exit 1
fi

FILENAME="${BINARY}_${OS}_${ARCH}.${EXT}"
DOWNLOAD_URL="https://github.com/${REPO}/releases/latest/download/${FILENAME}"

echo "📁 创建目录..."
mkdir -p "$INSTALL_DIR"
mkdir -p "$CONFIG_DIR"

echo "⬇️  下载最新版 ${BINARY} (${OS}/${ARCH})..."
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

curl -fsSL "$DOWNLOAD_URL" -o "${TMP_DIR}/${FILENAME}"

echo "📦 解压中..."
if [ "$EXT" = "zip" ]; then
  if ! command -v unzip &>/dev/null; then
    echo "❌ Windows 上需要 unzip 才能安装；也可以改用 PowerShell 安装脚本："
    echo "   irm https://raw.githubusercontent.com/${REPO}/main/scripts/install.ps1 | iex"
    exit 1
  fi
  unzip -q "${TMP_DIR}/${FILENAME}" -d "$TMP_DIR"
else
  tar -xzf "${TMP_DIR}/${FILENAME}" -C "$TMP_DIR"
fi

chmod +x "${TMP_DIR}/${BINARY_NAME}"

# Install
cp "${TMP_DIR}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
echo "✅ 安装完成: ${INSTALL_DIR}/${BINARY_NAME}"

# Check PATH
case ":$PATH:" in
  *":${INSTALL_DIR}:"*) ;;
  *)
    echo ""
    echo "⚠️  ${INSTALL_DIR} 不在 PATH 中，请将以下内容添加到 shell 配置 (~/.zshrc / ~/.bashrc)："
    echo ""
    echo "    export PATH=\"\$PATH:${INSTALL_DIR}\""
    echo ""
    ;;
esac

echo ""
echo "📝 配置文件目录: ${CONFIG_DIR}"
echo ""
echo "🚀 运行 ${BINARY_NAME} --version 验证安装"
