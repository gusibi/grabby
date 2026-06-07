---
name: grabby
description: 抓取网页内容并转换为 Markdown 格式。当用户说要抓取网页、提取网页内容、URL 转 Markdown、保存网页、grab page、extract content、fetch page、scrape webpage 时触发。即使没有明确说"markdown"，只要用户想把网页内容拿下来阅读或分析，就应该使用此 skill。
---

# Grabby — 网页内容抓取

通过本地 Grabby CLI 工具抓取网页，返回干净的 Markdown 内容。

## 前置检查

### 1. 检查 grabby CLI 是否已安装

```bash
command -v grabby
```

- **找到** → `grabby` 命令可用，继续下一步
- **未找到** → 按照以下步骤下载源码并完成安装：

  1. **克隆源码仓库**（若本地还没有源码）：
     ```bash
     git clone https://github.com/gusibi/mcp-web-capture.git ~/.grabby/src
     ```
  2. **进入源码目录**：
     ```bash
     cd ~/.grabby/src
     # 注：若已经处于克隆好的本地项目根目录下，直接在根目录执行即可
     ```
  3. **选择安装方式并运行**：
     - **推荐：安装 Python 版本 CLI** (免编译，适合 macOS，直接执行脚本)：
       ```bash
       python3 scripts/install.py --type python
       ```
     - **安装 Go 版本 CLI** (在本地基于源码编译)：
       - 先执行 `command -v go` 检查是否有 Go 环境。
       - 若无 Go 环境，提示用户先安装 Go 编译器（如 macOS 可使用 `brew install go`，或访问 https://go.dev/doc/install ）。
       - 拥有 Go 环境后，执行编译安装：
         ```bash
         python3 scripts/install.py --type go
         ```

### 2. 确定 grabby 命令

将所有后续命令中的 `grabby` 替换为实际路径：
- 如果 `command -v grabby` 成功 → 使用 `grabby`
- 如果 `./go-cli/grabby` 存在 → 使用 `./go-cli/grabby`
- 否则 → 先安装

## 工作流程

### 1. 确定目标 URL

从用户输入中提取目标 URL。如果 URL 不完整（缺少 `https://`），自动补全。

### 2. 检查服务状态

```bash
grabby health
```

**判断 exit code：**
- **exit 0**: 服务运行中，浏览器已连接 → 继续下一步
- **exit 1**: 服务未运行 → 提示用户启动服务：
  ```bash
  grabby start python
  # 或
  grabby start go
  ```
- **exit 2**: 服务运行中，但浏览器未连接 → 提示用户打开 Grabby Chrome 扩展

**JSON 输出参考：**
```json
{"status":"ok","browser_connected":true,"count":1,"browsers":["browser-tools"]}
```

### 3. 抓取网页

```bash
grabby extract <target-url>
```

**JSON 输出参考：**
```json
{"title": "页面标题", "url": "https://example.com", "markdown": "# Markdown 内容..."}
```

将返回的 `markdown` 字段内容展示给用户。同时显示 `title` 和原始 `url`。

### 4. 其他命令（按需使用）

```bash
# 列出已连接的浏览器
grabby browsers list

# 注册浏览器
grabby browsers register <connect_id> <name>

# 截取网页截图
grabby screenshot <url>
```

### 服务启动

```bash
# 在前台启动 Python 服务
grabby start python

# 启动 Go 服务
grabby start go
```

**配置文件：**
- 所有配置文件统一放在 `~/.grabby/` 目录下
- 服务端口、连接 ID 等配置写在 `~/.grabby/.env` 中
- CLI 启动服务时会自动读取 `~/.grabby/.env`
- 安装脚本会自动创建 `~/.grabby/` 目录

## 完整安装流程

```bash
# 1. 安装 grabby CLI (推荐 Python 脚本版本以避免安全策略拦截)
python3 scripts/install.py --type python

# 2. 验证安装
grabby --version

# 3. 启动服务 (根据需要选择 python 或 go 后端)
grabby start python

# 4. 检查状态
grabby health

# 5. 抓取网页
grabby extract https://example.com
```

## 错误处理

| 情况 | 表现 | 处理 |
|------|------|------|
| grabby 未安装 | command not found | 运行安装脚本 |
| 服务未启动 | exit 1 | 提示用户启动服务 |
| 浏览器未连接 | exit 2 | 提示打开 Grabby Chrome 扩展 |
| 提取失败 | exit 3 | 显示错误信息 |

## 注意事项

- 抓取依赖浏览器扩展实际加载页面，动态内容可能需要较长等待时间
- 如果页面需要登录才能访问，建议用户先在浏览器中登录
- 返回的 Markdown 内容由浏览器的内容提取算法生成，复杂页面可能不完美