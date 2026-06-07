# Grabby Build Makefile

EXTENSION_DIR := chrome-extension
GO_SERVER_DIR := go-server
DIST_DIR := dist
VERSION := $(shell cat $(EXTENSION_DIR)/manifest.json | grep '"version"' | sed 's/.*"version": "\([^"]*\)".*/\1/')
PACKAGE_NAME := grabby-v$(VERSION).zip

.PHONY: all build package clean install dev build-go run-go clean-go build-frontend run

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

# Go 源文件列表（用于 Make 依赖追踪）
GO_SRC := $(shell find $(GO_SERVER_DIR) -name '*.go' -type f)

# 预下载 Go 依赖
go-deps:
	@echo "==> 下载 Go 依赖..."
	cd $(GO_SERVER_DIR) && GOPROXY=$(GO_PROXY) go mod download

# 构建 React 前端
build-frontend:
	@echo "==> 构建 React 前端..."
	cd $(GO_SERVER_DIR)/frontend && npm run build

# 构建 Go MCP Server（依赖前端构建及 Go 源码，确保最新前端被嵌入）
build-go: build-frontend $(GO_SRC)
	@echo "==> 构建 Go MCP Server..."
	cd $(GO_SERVER_DIR) && GOPROXY=$(GO_PROXY) go build -o go-server .
	@echo "==> Go Server 构建完成: $(GO_SERVER_DIR)/go-server"

# 运行 Go MCP Server
run-go: build-go
	@echo "==> 启动 Go MCP Server..."
	cd $(GO_SERVER_DIR) && ./go-server

# 编译前端，再编译 Go，最后运行
run: build-frontend build-go
	@echo "==> 启动 Go MCP Server..."
	cd $(GO_SERVER_DIR) && ./go-server

# 清理 Go 构建产物
clean-go:
	@echo "==> 清理 Go 构建产物..."
	@rm -f $(GO_SERVER_DIR)/go-server
	@echo "==> Go 构建产物清理完成"

# ==================== Common ====================

# 清理所有构建产物
clean: clean-go
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
	@echo "=== Common ==="
	@echo "  make clean        - 清理所有构建产物"
	@echo "  make clean-all    - 深度清理（包含 node_modules）"
	@echo "  make version      - 显示当前版本号"
	@echo "  make help         - 显示此帮助信息"
