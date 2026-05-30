#!/usr/bin/env bash
set -euo pipefail

# Grabby CLI Installer
# Usage: curl -fsSL https://raw.githubusercontent.com/gusibi/mcp-web-capture/main/scripts/install.sh | bash
#
# 安装 grabby 二进制到 ~/.local/bin/grabby
# 配置文件目录: ~/.grabby/

REPO="gusibi/mcp-web-capture"
BINARY="grabby"
INSTALL_DIR="${GRABBY_INSTALL_DIR:-${HOME}/.local/bin}"
CONFIG_DIR="${HOME}/.grabby"

# Detect OS and ARCH
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$OS" in
  darwin)  OS="Darwin" ;;
  linux)   OS="Linux" ;;
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

echo "📡 获取最新版本..."
if ! command -v curl &>/dev/null; then
  echo "❌ 需要 curl，请先安装"
  exit 1
fi

LATEST_RELEASE=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)

if [ -z "$LATEST_RELEASE" ]; then
  echo "❌ 无法获取最新版本号"
  echo "   请确认 https://github.com/${REPO}/releases 有 release"
  exit 1
fi

VERSION="${LATEST_RELEASE#grabby-v}"

# Build filename
FILENAME="${BINARY}_${OS}_${ARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${LATEST_RELEASE}/${FILENAME}"

# Create install dir
echo "📁 创建目录..."
mkdir -p "$INSTALL_DIR"
mkdir -p "$CONFIG_DIR"

# Download and extract
echo "⬇️  下载 ${BINARY} v${VERSION} (${OS}/${ARCH})..."
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

curl -fsSL "$DOWNLOAD_URL" -o "${TMP_DIR}/${FILENAME}"

echo "📦 解压中..."
tar -xzf "${TMP_DIR}/${FILENAME}" -C "$TMP_DIR"
chmod +x "${TMP_DIR}/${BINARY}"

# Install
cp "${TMP_DIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
echo "✅ 安装完成: ${INSTALL_DIR}/${BINARY}"

# Check PATH
if ! echo "$PATH" | tr ':' '\n' | grep -q "${INSTALL_DIR}"; then
  echo ""
  echo "⚠️  ${INSTALL_DIR} 不在 PATH 中，请将以下内容添加到 shell 配置 (~/.zshrc / ~/.bashrc)："
  echo ""
  echo "    export PATH=\"\$PATH:${INSTALL_DIR}\""
  echo ""
fi

echo ""
echo "📝 配置文件目录: ${CONFIG_DIR}"
echo "   将配置放入 ${CONFIG_DIR}/.env"
echo ""
echo "🚀 运行 grabby --version 验证安装"