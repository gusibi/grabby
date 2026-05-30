# Grabby CLI 需求文档

## 背景

项目 `mcp-web-capture`（Grabby）提供 Python 和 Go 两套后端服务，暴露 HTTP API 用于网页内容抓取。当前使用方式需要手动发 HTTP 请求（curl），操作繁琐。目标是将 API 封装为 CLI 工具，简化调用。

---

## 目标

### 1. CLI 工具（`grabby`）

用 Go 实现 CLI，编译为单二进制，零依赖，通过 GitHub Releases 分发。

**CLI 命令清单：**

| 命令 | 功能 | Exit Code |
|------|------|-----------|
| `grabby health` | 检查服务 + 浏览器连接状态 | 0=正常, 1=服务未启动, 2=浏览器未连接 |
| `grabby extract <url>` | 抓取网页返回 Markdown | 0=成功, 3=失败 |
| `grabby screenshot <url>` | 网页截图 | 0=成功, 3=失败 |
| `grabby browsers list` | 列出已连接的浏览器 | — |
| `grabby browsers register <id> <name>` | 注册浏览器实例 | — |
| `grabby start python\|go` | 启动后端服务 | — |
| `grabby install` | 从 GitHub Releases 下载安装自身 | — |
| `grabby --version` | 查看版本 | — |

**环境变量：**
- `GRABBY_SERVER_URL` — 服务地址（默认 `http://localhost:5040`）
- `GRABBY_PROJECT_DIR` — 项目源码目录（`start` 命令需要）
- `GRABBY_INSTALL_DIR` — 安装路径（默认 `~/.local/bin`）

---

### 2. 配置文件

配置文件统一放在 `~/.grabby/.env`，内容示例：

```
HOST=0.0.0.0
PORT=5040
CONNECT_ID=browser-tools
DEBUG=false
DEFAULT_BROWSER=
```

- CLI 启动服务时自动从 `~/.grabby/` 读取 `.env`
- 安装脚本和 `grabby install` 命令自动创建 `~/.grabby/` 目录

---

### 3. 打包与发布

**GoReleaser** 配置（`.goreleaser.yaml`）：
- 构建平台：darwin/amd64, darwin/arm64, linux/amd64, linux/arm64, windows/amd64
- 打包格式：tar.gz（Windows 为 zip）
- 包含 README 和 LICENSE

**GitHub Actions**（`.github/workflows/release.yml`）：
- 推送 `grabby-v*` 标签时自动触发
- 使用 goreleaser-action 构建并上传 Release
- 非标签推送时仅做 build check

**安装方式：**
```bash
# 方式一：安装脚本（推荐）
curl -fsSL https://raw.githubusercontent.com/gusibi/mcp-web-capture/main/scripts/install.sh | bash

# 方式二：已有二进制时自安装
grabby install
```

---

### 4. Skills 更新（`skills/grabby/SKILL.md`）

Skill 工作流程改为：

**Step 1 — Install Check**
```bash
command -v grabby
# 找不到 → 检查 ./go-cli/grabby（本地开发）
# 还找不到 → 提示运行安装脚本
```

**Step 2 — Service Check**
```bash
grabby health
# exit 0 → 服务运行中，浏览器已连接
# exit 1 → 提示启动：grabby start python/go
# exit 2 → 提示打开浏览器扩展
```

**Step 3 — Extract**
```bash
grabby extract <url>
# 解析 JSON，展示 markdown
```

**Step 4 — 其他**
```bash
grabby browsers list
grabby screenshot <url>
grabby start python   # 或 grabby start go
```

---

### 5. 目录结构（最终）

```
mcp-web-capture/
├── .goreleaser.yaml                   # GoReleaser 构建配置
├── .github/workflows/release.yml      # GitHub Actions 发布流水线
├── scripts/
│   └── install.sh                     # curl|bash 安装脚本
├── go-cli/
│   ├── main.go                        # 入口
│   ├── cmd/
│   │   ├── root.go                    # 根命令 + 公共函数
│   │   ├── health.go                  # grabby health
│   │   ├── extract.go                 # grabby extract
│   │   ├── screenshot.go              # grabby screenshot
│   │   ├── browsers.go                # grabby browsers
│   │   ├── start.go                   # grabby start（加载 ~/.grabby/.env）
│   │   └── install.go                 # grabby install（从 GitHub 下载二进制）
│   ├── go.mod
│   └── grabby                         # 编译后的二进制
├── skills/grabby/
│   └── SKILL.md                       # 更新为 CLI 调用
└── ~/.grabby/
    └── .env                           # 统一配置文件
```

### 6. 未在本次需求范围内的

- 修改 Python 或 Go 服务端代码（config 加载逻辑不变）
- 支持 `grabby start` 从项目目录外运行（仍需源码目录存在）
- Windows 上 `grabby install` 的 tar 解压兼容性
- 自动更新（self-update）功能