#!/usr/bin/env bash
# 打包桌面 App 的发布产物（单文件，已包含 Go 服务端、WebView UI 与 SQLite）。
#
#   scripts/package-desktop.sh macos   <version>   # 通用架构 .dmg（需 macOS: lipo/hdiutil/codesign）
#   scripts/package-desktop.sh windows <version>   # .exe / portable zip / NSIS 安装包（需 go-winres、makensis）
#   scripts/package-desktop.sh linux   <version>   # 可执行文件 / tar.gz / .deb（需 dpkg-deb）
#
# 构建使用 CGO_ENABLED=0：MyGO 是纯 Go，交叉编译即可，产物不依赖 Go/Node 运行时。
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

TARGET="${1:-}"
VERSION="${2:-}"
if [[ -z "$TARGET" || -z "$VERSION" ]]; then
  echo "usage: $0 <macos|windows|linux> <version>" >&2
  exit 2
fi
VERSION="${VERSION#v}"
if [[ ! "$VERSION" =~ ^[0-9] ]]; then
  echo "invalid version: $VERSION" >&2
  exit 2
fi

DIST="$ROOT/dist"
APP_NAME="Grabby"
BUILD_SRC="$ROOT/go-server"
# ARCH limits windows/linux packaging to one architecture (used by the CI matrix).
ARCH="${ARCH:-}"

log() { printf '==> %s\n' "$*"; }

build_binary() {
  local goos="$1" goarch="$2" out="$3" ldflags="$4"
  ( cd "$BUILD_SRC" && CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
      go build -tags desktop -trimpath -ldflags "$ldflags" -o "$out" . )
}

package_macos() {
  for tool in lipo hdiutil codesign /usr/libexec/PlistBuddy; do
    command -v "$tool" >/dev/null 2>&1 || { echo "missing macOS tool: $tool" >&2; exit 1; }
  done

  local universal="$DIST/universal"
  rm -rf "$universal" "$DIST/$APP_NAME.app" "$DIST/dmg"
  mkdir -p "$universal"

  log "building darwin/arm64 and darwin/amd64"
  build_binary darwin arm64 "$universal/Grabby-arm64" "-s -w"
  build_binary darwin amd64 "$universal/Grabby-amd64" "-s -w"
  lipo -create -output "$universal/Grabby" "$universal/Grabby-arm64" "$universal/Grabby-amd64"

  log "assembling $APP_NAME.app (universal)"
  mkdir -p "$DIST/$APP_NAME.app/Contents/MacOS" "$DIST/$APP_NAME.app/Contents/Resources"
  cp go-server/desktop/Info.plist "$DIST/$APP_NAME.app/Contents/Info.plist"
  /usr/libexec/PlistBuddy -c "Set :CFBundleShortVersionString $VERSION" "$DIST/$APP_NAME.app/Contents/Info.plist"
  /usr/libexec/PlistBuddy -c "Set :CFBundleVersion $VERSION" "$DIST/$APP_NAME.app/Contents/Info.plist"
  cp go-server/desktop/assets/AppIcon.icns "$DIST/$APP_NAME.app/Contents/Resources/AppIcon.icns"
  cp "$universal/Grabby" "$DIST/$APP_NAME.app/Contents/MacOS/Grabby"
  chmod +x "$DIST/$APP_NAME.app/Contents/MacOS/Grabby"
  xattr -cr "$DIST/$APP_NAME.app" 2>/dev/null || true
  codesign --force --deep --sign - "$DIST/$APP_NAME.app"

  log "creating disk image"
  mkdir -p "$DIST/dmg"
  cp -R "$DIST/$APP_NAME.app" "$DIST/dmg/"
  ln -s /Applications "$DIST/dmg/Applications"
  local dmg="$DIST/Grabby-${VERSION}-macOS-universal.dmg"
  rm -f "$dmg"
  hdiutil create -quiet -volname "Grabby $VERSION" -srcfolder "$DIST/dmg" -ov -format UDZO "$dmg"
  rm -rf "$DIST/dmg" "$universal"
  log "done: $dmg"
}

package_windows() {
  local tools="${RUNNER_TEMP:-$ROOT/.tmp}/bin"
  mkdir -p "$tools"
  if ! command -v go-winres >/dev/null 2>&1; then
    log "installing go-winres"
    GOBIN="$tools" go install github.com/tc-hib/go-winres@v0.3.3
    export PATH="$tools:$PATH"
  fi
  command -v zip >/dev/null 2>&1 || { echo "missing zip" >&2; exit 1; }

  local icon="$ROOT/assets/icons/windows/icon.ico"
  local arches="${ARCH:-amd64 arm64}"
  log "generating Windows icon/version resources"
  ( cd "$BUILD_SRC" && go-winres simply \
      --icon "$icon" \
      --product-name "$APP_NAME" \
      --product-version "$VERSION" \
      --file-version "$VERSION" \
      --file-description "$APP_NAME" \
      --manifest gui \
      --arch "${arches// /,}" )

  for arch in $arches; do
    local exe="$DIST/Grabby-${VERSION}-Windows-${arch}.exe"
    log "building windows/$arch"
    build_binary windows "$arch" "$exe" "-s -w -H windowsgui"
    ( cd "$DIST" && rm -f "Grabby-${VERSION}-Windows-${arch}.zip" && \
      zip -q -j "Grabby-${VERSION}-Windows-${arch}.zip" "Grabby-${VERSION}-Windows-${arch}.exe" )

    if command -v makensis >/dev/null 2>&1; then
      log "building Windows installer ($arch)"
      makensis -V2 \
        -DVERSION="$VERSION" \
        -DARCH="$arch" \
        -DSOURCE="$exe" \
        -DICON="$icon" \
        -DOUTFILE="$DIST/Grabby-${VERSION}-Windows-${arch}-setup.exe" \
        "$ROOT/packaging/windows/installer.nsi"
    else
      log "makensis not found, skipping the Windows installer"
    fi
  done
  rm -f "$BUILD_SRC"/rsrc_windows_*.syso
  log "done: $DIST/Grabby-${VERSION}-Windows-*"
}

package_linux() {
  local icon="$ROOT/assets/icons/linux/hicolor/512x512/apps/grabby.png"
  local icon256="$ROOT/assets/icons/linux/hicolor/256x256/apps/grabby.png"

  for arch in ${ARCH:-amd64 arm64}; do
    local bin="$DIST/.build/grabby-linux-${arch}"
    mkdir -p "$DIST/.build"
    log "building linux/$arch"
    build_binary linux "$arch" "$bin" "-s -w"

    local stage="$DIST/Grabby-${VERSION}-Linux-${arch}"
    rm -rf "$stage"
    mkdir -p "$stage"
    cp "$bin" "$stage/grabby"
    cp "$icon" "$stage/grabby.png"
    cat > "$stage/README.txt" <<'EOF'
Grabby — 本地优先的浏览器采集 + AI 阅读台

运行 ./grabby 启动。程序已包含 HTTP / WebSocket / MCP 服务与桌面界面，
不需要再安装 Go、Node 或单独启动服务端。

系统依赖：GTK 3 与 WebKitGTK 4.1（libwebkit2gtk-4.1-0）。
EOF
    chmod +x "$stage/grabby"
    tar -czf "$DIST/Grabby-${VERSION}-Linux-${arch}.tar.gz" -C "$DIST" "Grabby-${VERSION}-Linux-${arch}"
    rm -rf "$stage"

    if command -v dpkg-deb >/dev/null 2>&1; then
      local deb="$DIST/deb-${arch}"
      rm -rf "$deb"
      mkdir -p "$deb/opt/grabby" \
        "$deb/usr/share/applications" \
        "$deb/usr/share/icons/hicolor/256x256/apps" \
        "$deb/usr/share/icons/hicolor/512x512/apps" \
        "$deb/DEBIAN"
      install -m 0755 "$bin" "$deb/opt/grabby/grabby"
      cp "$icon" "$deb/usr/share/icons/hicolor/512x512/apps/grabby.png"
      [ -f "$icon256" ] && cp "$icon256" "$deb/usr/share/icons/hicolor/256x256/apps/grabby.png"
      cp "$ROOT/packaging/linux/grabby.desktop" "$deb/usr/share/applications/grabby.desktop"
      cat > "$deb/DEBIAN/control" <<EOF
Package: grabby
Version: ${VERSION}
Section: utils
Priority: optional
Architecture: ${arch}
Maintainer: Grabby Contributors <noreply@users.noreply.github.com>
Installed-Size: $(du -sk "$deb/opt" | cut -f1)
Depends: libgtk-3-0, libwebkit2gtk-4.1-0, libayatana-appindicator3-1
Recommends: libnotify4
Description: Local-first web capture and AI reading desk
 Grabby captures web pages through a real browser and organises them with AI.
 The desktop app embeds the Go HTTP / WebSocket / MCP server; no separate
 backend process is needed.
EOF
      log "building .deb ($arch)"
      dpkg-deb --build --root-owner-group "$deb" "$DIST/grabby_${VERSION}_${arch}.deb" >/dev/null
      rm -rf "$deb"
    else
      log "dpkg-deb not found, skipping the .deb"
    fi
  done
  rm -rf "$DIST/.build"
  log "done: $DIST/Grabby-${VERSION}-Linux-*"
}

mkdir -p "$DIST"
case "$TARGET" in
  macos) package_macos ;;
  windows) package_windows ;;
  linux) package_linux ;;
  *) echo "unknown target: $TARGET" >&2; exit 2 ;;
esac
