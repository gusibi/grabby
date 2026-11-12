# Grabby Build Makefile

EXTENSION_DIR := chrome-extension
GO_SERVER_DIR := go-server
DIST_DIR := dist
VERSION := $(shell cat $(EXTENSION_DIR)/manifest.json | grep '"version"' | sed 's/.*"version": "\([^"]*\)".*/\1/')
PACKAGE_NAME := grabby-v$(VERSION).zip

.PHONY: all build package clean install dev build-go run-go clean-go build-frontend run build-desktop run-desktop clean-desktop package-macos-app app app-stop app-log stop-desktop

# ==================== Chrome Extension ====================

# 默认目标：构建并打包扩展
all: build package

# 安装扩展依赖
install:
	@echo "==> 安装 chrome-extension 依赖..."
	cd $(EXTENSION_DIR) && npm install

# 构建 defuddle bundle
build-bundle:
	@echo "==> 构建 defuddle bundle..."
	cd $(EXTENSION_DIR) && npm run build
	@echo "==> Bundle 构建完成: $(EXTENSION_DIR)/lib/defuddle.bundle.js"

# 完整构建流程
build: install build-bundle
	@echo "==> 扩展构建完成"

# 开发模式构建（不压缩）
dev:
	@echo "==> 开发模式构建..."
	cd $(EXTENSION_DIR) && npm install && npm run build

# 打包扩展为 zip（仅包含发布到 Chrome Web Store 的必需文件）
package: build
	@echo "==> 打包扩展 (版本: $(VERSION))..."
	@mkdir -p $(DIST_DIR)
	@rm -f $(DIST_DIR)/$(PACKAGE_NAME)
	cd $(EXTENSION_DIR) && zip -r ../$(DIST_DIR)/$(PACKAGE_NAME) \
		manifest.json \
		background.js \
		content/ \
		lib/ \
		icons/ \
		popup/ \
		options/ \
		offscreen/ \
		PRIVACY_POLICY*.md \
		-x "lib/*.map" \
		-x "src/*" \
		-x "node_modules/*" \
		-x "package.json" \
		-x "package-lock.json" \
		-x "build.js" \
		-x "logs/*" \
		2>/dev/null || true
	@echo "==> 打包完成: $(DIST_DIR)/$(PACKAGE_NAME)"

# ==================== Go Server ====================

# Go 模块代理（goproxy.cn 国内访问更快）
GO_PROXY ?= https://goproxy.cn,direct

# 本机 `go env -w GOSUMDB=off` 会让工具链自动切换直接失败：
#   golang.org/toolchain@v0.0.1-go1.27.3: checksum database disabled by GOSUMDB=off
# go.mod 要求 go 1.27.3，而系统 Go 只有 1.26.x，必须靠 GOTOOLCHAIN 自动下载。
# 因此所有 go 命令统一在子进程里把 GOSUMDB 覆盖回来（goproxy.cn 支持 sumdb 代理）。
export GOSUMDB := sum.golang.org
export GOPROXY := $(GO_PROXY)

# Go 源文件列表（用于 Make 依赖追踪）
GO_SRC := $(shell find $(GO_SERVER_DIR) -name '*.go' -type f)

# 预下载 Go 依赖
go-deps:
	@echo "==> 下载 Go 依赖..."
	cd $(GO_SERVER_DIR) && go mod download

# 构建 React 前端
build-frontend:
	@echo "==> 构建 React 前端..."
	cd $(GO_SERVER_DIR)/frontend && npm run build

# 构建 Go MCP Server（依赖前端构建及 Go 源码，确保最新前端被嵌入）
build-go: build-frontend $(GO_SRC)
	@echo "==> 构建 Go MCP Server..."
	cd $(GO_SERVER_DIR) && go build -o go-server .
	@echo "==> Go Server 构建完成: $(GO_SERVER_DIR)/go-server"

# 运行 Go MCP Server
run-go: build-go
	@echo "==> 启动 Go MCP Server..."
	cd $(GO_SERVER_DIR) && ./go-server

# 编译前端，然后直接运行 Go (使用 go run 规避 macOS 签名限制)
run: build-frontend
	@echo "==> 启动 Go MCP Server..."
	cd $(GO_SERVER_DIR) && go run .

# 清理 Go 构建产物
clean-go:
	@echo "==> 清理 Go 构建产物..."
	@rm -f $(GO_SERVER_DIR)/go-server
	@echo "==> Go 构建产物清理完成"

# ==================== Desktop App (MyGO) ====================

DESKTOP_BIN := $(GO_SERVER_DIR)/grabby-desktop

# 构建桌面 APP（使用独立的 desktop/dist 前端，不影响 Web 页面）
build-desktop:
	@echo "==> 构建 Grabby 桌面 APP..."
	cd $(GO_SERVER_DIR) && go build -tags desktop -o grabby-desktop .
	@echo "==> 完成: $(DESKTOP_BIN)"

# 构建并启动桌面 APP（前台运行，日志直接打印在终端）
run-desktop: build-desktop
	@echo "==> 启动 Grabby 桌面 APP..."
	cd $(GO_SERVER_DIR) && ./grabby-desktop

# 生成可在 macOS 双击运行的 .app（未签名，仅限本机使用）
package-macos-app: build-desktop
	@echo "==> 打包 macOS .app..."
	@rm -rf $(DIST_DIR)/Grabby.app
	@mkdir -p $(DIST_DIR)/Grabby.app/Contents/MacOS
	@cp $(DESKTOP_BIN) $(DIST_DIR)/Grabby.app/Contents/MacOS/Grabby
	@cp $(GO_SERVER_DIR)/desktop/Info.plist $(DIST_DIR)/Grabby.app/Contents/Info.plist
	@chmod +x $(DIST_DIR)/Grabby.app/Contents/MacOS/Grabby
	@xattr -cr $(DIST_DIR)/Grabby.app 2>/dev/null || true
	@echo "==> 完成: $(DIST_DIR)/Grabby.app"

clean-desktop:
	@rm -f $(DESKTOP_BIN)

# ==================== make app（一条命令开 APP） ====================

APP_BUNDLE := $(DIST_DIR)/Grabby.app
APP_LOG_DIR := $(HOME)/Library/Logs/Grabby

# 结束已在运行的 APP（单实例锁会让新进程直接退出，所以先停掉）
stop-desktop:
	@osascript -e 'quit app "Grabby"' >/dev/null 2>&1 || true
	@if pgrep -f '$(abspath $(APP_BUNDLE))/Contents/MacOS/Grabby' >/dev/null 2>&1; then \
		echo "==> 停止已在运行的 Grabby..."; \
		pkill -f '$(abspath $(APP_BUNDLE))/Contents/MacOS/Grabby' >/dev/null 2>&1 || true; \
		for i in 1 2 3 4 5 6 7 8 9 10; do \
			pgrep -f '$(abspath $(APP_BUNDLE))/Contents/MacOS/Grabby' >/dev/null 2>&1 || break; \
			sleep 0.5; \
		done; \
		pkill -9 -f '$(abspath $(APP_BUNDLE))/Contents/MacOS/Grabby' >/dev/null 2>&1 || true; \
	fi

# 一条命令启动 APP：打包 → 启动。HTTP / WebSocket / MCP 服务由 APP 进程自己拉起
# （见 go-server/main_desktop.go），不需要再另开终端跑 make run。
# 关掉窗口不会退出，它常驻菜单栏；从菜单栏图标「退出 Grabby」。
app: stop-desktop package-macos-app
	@mkdir -p $(APP_LOG_DIR)
	@echo "==> 启动 $(APP_BUNDLE) ..."
	@open $(APP_BUNDLE)
	@echo "==> 已启动（进程内自带 server，无需另跑 make run）"
	@echo "    日志: $(APP_LOG_DIR)/desktop.log"
	@echo "    退出: 菜单栏图标 → 退出 Grabby，或 make app-stop"

# 停止 APP
app-stop: stop-desktop
	@echo "==> Grabby 已停止"

# 跟踪 APP 日志
app-log:
	@tail -f $(APP_LOG_DIR)/desktop.log

# ==================== Common ====================

# 清理所有构建产物
clean: clean-go clean-desktop
	@echo "==> 清理扩展构建产物..."
	@rm -rf $(DIST_DIR)
	@rm -f $(EXTENSION_DIR)/lib/defuddle.bundle.js
	@echo "==> 清理完成"

# 深度清理（包括 node_modules）
clean-all: clean
	@echo "==> 清理 node_modules..."
	@rm -rf $(EXTENSION_DIR)/node_modules
	@rm -f $(EXTENSION_DIR)/package-lock.json
	@echo "==> 深度清理完成"

# 显示版本号
version:
	@echo "当前版本: $(VERSION)"

# 帮助信息
help:
	@echo "Grabby - Makefile"
	@echo ""
	@echo "=== Chrome Extension ==="
	@echo "  make install      - 安装 npm 依赖"
	@echo "  make build        - 完整构建扩展（npm install + bundle）"
	@echo "  make build-bundle - 仅构建 defuddle bundle"
	@echo "  make package      - 构建并打包为 zip"
	@echo "  make all          - 完整流程（build + package）"
	@echo ""
	@echo "=== Go MCP Server ==="
	@echo "  make build-frontend - 仅构建 React 前端"
	@echo "  make build-go       - 构建 Go MCP Server (自动构建前端)"
	@echo "  make run-go         - 构建并运行 Go MCP Server"
	@echo "  make run            - 先编译前端，再编译 Go，最后运行"
	@echo "  make clean-go       - 清理 Go 构建产物"
	@echo ""
	@echo "=== Desktop App (MyGO) ==="
	@echo "  make app                - 一条命令启动 APP（APP 内自带 server）"
	@echo "  make app-stop           - 停止 APP"
	@echo "  make app-log            - 跟踪 APP 日志"
	@echo "  make build-desktop      - 构建桌面 APP"
	@echo "  make run-desktop        - 前台运行 APP（日志打在终端，调 bug 用）"
	@echo "  make package-macos-app  - 打包为可双击的 .app（未签名）"
	@echo "  make clean-desktop      - 清理桌面 APP 二进制"
	@echo ""
	@echo "=== Common ==="
	@echo "  make clean        - 清理所有构建产物"
	@echo "  make clean-all    - 深度清理（包含 node_modules）"
	@echo "  make version      - 显示当前版本号"
	@echo "  make help         - 显示此帮助信息"
