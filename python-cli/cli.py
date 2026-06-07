#!/usr/bin/env python3
import sys
import os
import json
import urllib.request
import urllib.error
import subprocess
import argparse

VERSION = "v0.1.0"

def load_env():
    config_dir = os.path.expanduser("~/.grabby")
    env_file = os.path.join(config_dir, ".env")
    if not os.path.exists(env_file):
        return {}
    
    env_vars = {}
    try:
        with open(env_file, "r", encoding="utf-8") as f:
            for line in f:
                line = line.strip()
                if not line or line.startswith("#"):
                    continue
                parts = line.split("=", 1)
                if len(parts) == 2:
                    key = parts[0].strip()
                    val = parts[1].strip()
                    # Strip quotes if present
                    if (val.startswith('"') and val.endswith('"')) or (val.startswith("'") and val.endswith("'")):
                        val = val[1:-1]
                    env_vars[key] = val
    except Exception:
        pass
    return env_vars

def get_server_url(env_vars):
    url = os.environ.get("GRABBY_SERVER_URL") or env_vars.get("GRABBY_SERVER_URL")
    if url:
        return url.rstrip('/')
    port = os.environ.get("PORT") or env_vars.get("PORT") or "5040"
    return f"http://localhost:{port}"

def make_request(url, method="GET", data=None):
    headers = {"Content-Type": "application/json"}
    req = urllib.request.Request(url, method=method, headers=headers)
    if data is not None:
        req.data = json.dumps(data).encode("utf-8")
    
    try:
        with urllib.request.urlopen(req, timeout=70) as response:
            return response.status, response.read().decode("utf-8")
    except urllib.error.HTTPError as e:
        try:
            body = e.read().decode("utf-8")
        except Exception:
            body = ""
        return e.code, body
    except urllib.error.URLError as e:
        return None, str(e.reason)

def cmd_health(server_url):
    status_code, body = make_request(f"{server_url}/api/health", "GET")
    if status_code is None:
        print(json.dumps({"error": "服务未运行", "exit_code": 1}))
        sys.exit(1)
    
    if status_code != 200:
        print(json.dumps({"error": "解析响应失败", "exit_code": 1}))
        sys.exit(1)
        
    try:
        data = json.loads(body)
    except Exception:
        print(json.dumps({"error": "解析响应失败", "exit_code": 1}))
        sys.exit(1)
        
    browser_connected = data.get("browser_connected", False)
    if browser_connected:
        out = {
            "status": "ok",
            "browser_connected": True,
            "count": data.get("browser_count", 0),
            "browsers": data.get("browsers", [])
        }
        print(json.dumps(out, ensure_ascii=False))
        sys.exit(0)
    else:
        out = {
            "status": "ok",
            "browser_connected": False,
            "message": "浏览器未连接，请打开 Grabby Chrome 扩展"
        }
        print(json.dumps(out, ensure_ascii=False))
        sys.exit(2)

def cmd_extract(server_url, url, browser):
    if not (url.startswith("http://") or url.startswith("https://")):
        url = "https://" + url
        
    payload = {"url": url}
    if browser:
        payload["browser"] = browser
        
    status_code, body = make_request(f"{server_url}/api/extract", "POST", payload)
    if status_code is None:
        print(json.dumps({"error": "服务未运行", "exit_code": 1}))
        sys.exit(1)
        
    try:
        data = json.loads(body)
    except Exception:
        print(json.dumps({"error": "解析响应失败", "exit_code": 3}))
        sys.exit(3)
        
    if status_code != 200 or not data.get("success", False):
        detail = data.get("detail") or data.get("error") or "提取失败"
        print(json.dumps({"error": detail, "exit_code": 3}))
        sys.exit(3)
        
    out = {
        "title": data.get("title", ""),
        "url": data.get("url", ""),
        "markdown": data.get("markdown", "")
    }
    print(json.dumps(out, ensure_ascii=False))
    sys.exit(0)

def cmd_screenshot(server_url, url, browser):
    if not (url.startswith("http://") or url.startswith("https://")):
        url = "https://" + url
        
    payload = {"url": url, "fullPage": False}
    if browser:
        payload["browser"] = browser
        
    status_code, body = make_request(f"{server_url}/api/screenshot", "POST", payload)
    if status_code is None:
        print(json.dumps({"error": "服务未运行", "exit_code": 1}))
        sys.exit(1)
        
    try:
        data = json.loads(body)
    except Exception:
        print(json.dumps({"error": "解析响应失败", "exit_code": 3}))
        sys.exit(3)
        
    if status_code != 200 or not data.get("success", False):
        detail = data.get("detail") or data.get("error") or "截图失败"
        print(json.dumps({"error": detail, "exit_code": 3}))
        sys.exit(3)
        
    print(json.dumps(data, ensure_ascii=False))
    sys.exit(0)

def cmd_browsers_list(server_url):
    status_code, body = make_request(f"{server_url}/api/browsers", "GET")
    if status_code is None:
        print(json.dumps({"error": "服务未运行", "exit_code": 1}))
        sys.exit(1)
        
    if status_code != 200:
        print(json.dumps({"error": "解析响应失败", "exit_code": 3}))
        sys.exit(3)
        
    try:
        data = json.loads(body)
    except Exception:
        print(json.dumps({"error": "解析响应失败", "exit_code": 3}))
        sys.exit(3)
        
    out = {
        "count": data.get("count", 0),
        "browsers": data.get("browsers", [])
    }
    print(json.dumps(out, ensure_ascii=False))
    sys.exit(0)

def cmd_browsers_register(server_url, connect_id, name):
    payload = {"connect_id": connect_id, "name": name}
    status_code, body = make_request(f"{server_url}/api/browsers/register", "POST", payload)
    if status_code is None:
        print(json.dumps({"error": "服务未运行", "exit_code": 1}))
        sys.exit(1)
        
    try:
        data = json.loads(body)
    except Exception:
        print(json.dumps({"error": "解析响应失败", "exit_code": 3}))
        sys.exit(3)
        
    if status_code != 200 or not data.get("success", False):
        detail = data.get("detail") or data.get("error") or "注册失败"
        print(json.dumps({"error": detail, "exit_code": 3}))
        sys.exit(3)
        
    print(json.dumps(data, ensure_ascii=False))
    sys.exit(0)

def find_project_dir(env_vars):
    pdir = os.environ.get("GRABBY_PROJECT_DIR") or env_vars.get("GRABBY_PROJECT_DIR")
    if pdir and os.path.isdir(pdir):
        return pdir
        
    # Search upward from cwd
    cwd = os.getcwd()
    while True:
        if os.path.isdir(os.path.join(cwd, "python-server")) or os.path.isdir(os.path.join(cwd, "go-server")):
            return cwd
        parent = os.path.dirname(cwd)
        if parent == cwd:
            break
        cwd = parent
        
    # Search upward from script location
    script_dir = os.path.dirname(os.path.abspath(__file__))
    cwd = script_dir
    while True:
        if os.path.isdir(os.path.join(cwd, "python-server")) or os.path.isdir(os.path.join(cwd, "go-server")):
            return cwd
        parent = os.path.dirname(cwd)
        if parent == cwd:
            break
        cwd = parent
        
    return None

def start_python(project_dir, env_vars):
    server_dir = os.path.join(project_dir, "python-server")
    main_py = os.path.join(server_dir, "main.py")
    if not os.path.exists(main_py):
        print(json.dumps({"error": f"找不到 Python 服务: {main_py}", "exit_code": 3}))
        sys.exit(3)
        
    config_dir = os.path.expanduser("~/.grabby")
    port = int(env_vars.get("PORT") or os.environ.get("PORT") or "5040")
    out = {
        "message": "正在启动 Python Grabby 服务...",
        "port": port,
        "config_dir": config_dir
    }
    print(json.dumps(out, ensure_ascii=False))
    
    # Locate uv
    uv_path = None
    for p in os.environ.get("PATH", "").split(os.pathsep):
        candidate = os.path.join(p, "uv")
        if os.path.isfile(candidate) and os.access(candidate, os.X_OK):
            uv_path = candidate
            break
            
    merged_env = os.environ.copy()
    for k, v in env_vars.items():
        merged_env[k] = v
        
    if uv_path:
        cmd = [uv_path, "run", "python", "main.py"]
    elif os.path.exists(os.path.join(project_dir, ".venv", "bin", "python")):
        cmd = [os.path.join(project_dir, ".venv", "bin", "python"), "main.py"]
    else:
        cmd = ["python3", "main.py"]
        
    try:
        # Run inside server_dir so local module imports and relative paths resolve properly
        subprocess.run(cmd, cwd=server_dir, env=merged_env)
    except KeyboardInterrupt:
        pass
    except Exception as e:
        print(f"Python 服务启动失败: {e}", file=sys.stderr)
        sys.exit(1)

def start_go(project_dir, env_vars):
    go_dir = os.path.join(project_dir, "go-server")
    main_go = os.path.join(go_dir, "main.go")
    if not os.path.exists(main_go):
        print(json.dumps({"error": f"找不到 Go 服务: {go_dir}", "exit_code": 3}))
        sys.exit(3)
        
    config_dir = os.path.expanduser("~/.grabby")
    port = int(env_vars.get("PORT") or os.environ.get("PORT") or "5040")
    out = {
        "message": "正在启动 Go Grabby 服务...",
        "port": port,
        "config_dir": config_dir
    }
    print(json.dumps(out, ensure_ascii=False))
    
    merged_env = os.environ.copy()
    for k, v in env_vars.items():
        merged_env[k] = v
        
    cmd = ["go", "run", "./go-server/..."]
    try:
        subprocess.run(cmd, cwd=project_dir, env=merged_env)
    except KeyboardInterrupt:
        pass
    except Exception as e:
        print(f"Go 服务启动失败: {e}", file=sys.stderr)
        sys.exit(1)

def main():
    parser = argparse.ArgumentParser(description="Grabby - 网页内容抓取工具", add_help=False)
    parser.add_argument("-h", "--help", action="store_true", help="显示帮助信息")
    parser.add_argument("--version", action="store_true", help="显示版本号")
    
    # Custom parser to match Cobra's default behavior and parsing
    # First, check quick flags
    args, remaining = parser.parse_known_args()
    if args.version:
        print(f"grabby version {VERSION}")
        sys.exit(0)
        
    if args.help and not remaining:
        print_general_help()
        sys.exit(0)
        
    if not remaining:
        print_general_help()
        sys.exit(0)
        
    cmd = remaining[0]
    cmd_args = remaining[1:]
    
    env_vars = load_env()
    server_url = get_server_url(env_vars)
    
    if cmd == "version":
        print(f"grabby version {VERSION}")
        sys.exit(0)
        
    elif cmd == "health":
        cmd_health(server_url)
        
    elif cmd == "extract":
        parser_extract = argparse.ArgumentParser(description="抓取指定 URL 的网页内容为 Markdown")
        parser_extract.add_argument("url")
        parser_extract.add_argument("-b", "--browser", default="")
        if "-h" in cmd_args or "--help" in cmd_args:
            parser_extract.print_help()
            sys.exit(0)
        parsed_extract = parser_extract.parse_args(cmd_args)
        cmd_extract(server_url, parsed_extract.url, parsed_extract.browser)
        
    elif cmd == "screenshot":
        parser_screenshot = argparse.ArgumentParser(description="捕获指定 URL 的网页截图")
        parser_screenshot.add_argument("url")
        parser_screenshot.add_argument("-b", "--browser", default="")
        if "-h" in cmd_args or "--help" in cmd_args:
            parser_screenshot.print_help()
            sys.exit(0)
        parsed_screenshot = parser_screenshot.parse_args(cmd_args)
        cmd_screenshot(server_url, parsed_screenshot.url, parsed_screenshot.browser)
        
    elif cmd == "browsers":
        if not cmd_args:
            print("错误: 缺少 browsers 子命令 (可用: list, register)", file=sys.stderr)
            sys.exit(1)
        sub = cmd_args[0]
        sub_args = cmd_args[1:]
        
        if sub == "list":
            cmd_browsers_list(server_url)
        elif sub == "register":
            parser_reg = argparse.ArgumentParser(description="注册浏览器实例")
            parser_reg.add_argument("connect_id")
            parser_reg.add_argument("name")
            if "-h" in sub_args or "--help" in sub_args:
                parser_reg.print_help()
                sys.exit(0)
            parsed_reg = parser_reg.parse_args(sub_args)
            cmd_browsers_register(server_url, parsed_reg.connect_id, parsed_reg.name)
        else:
            print(f"未知的 browsers 子命令: {sub} (可用: list, register)", file=sys.stderr)
            sys.exit(1)
            
    elif cmd == "start":
        if not cmd_args:
            print("错误: 缺少 start 子命令 (可用: python, go)", file=sys.stderr)
            sys.exit(1)
        server_type = cmd_args[0]
        if server_type not in ["python", "go"]:
            print(f"未知的服务类型: {server_type} (可用: python, go)", file=sys.stderr)
            sys.exit(1)
            
        project_dir = find_project_dir(env_vars)
        if not project_dir:
            print("错误: 找不到项目目录 (需要包含 python-server/ 或 go-server/)", file=sys.stderr)
            print("      请设置 GRABBY_PROJECT_DIR 环境变量指定路径", file=sys.stderr)
            sys.exit(1)
            
        if server_type == "python":
            start_python(project_dir, env_vars)
        elif server_type == "go":
            start_go(project_dir, env_vars)
            
    elif cmd == "install":
        print("错误: Python 版本的 CLI 不支持 install 子命令。请使用 scripts/install.py 进行安装/配置。", file=sys.stderr)
        sys.exit(1)
        
    else:
        print(f"未知命令: {cmd}", file=sys.stderr)
        print_general_help()
        sys.exit(1)

def print_general_help():
    help_text = """Grabby - 网页内容抓取工具

通过本地 Grabby 服务抓取网页，返回干净的 Markdown 内容。

Usage:
  grabby [command]

Available Commands:
  health      检查 Grabby 服务状态和浏览器连接
  extract     抓取指定 URL 的网页内容为 Markdown
  screenshot  捕获指定 URL 的网页截图
  browsers    管理浏览器连接 (list, register)
  start       启动 Grabby 服务 (python 或 go)
  version     显示版本号

Flags:
  -h, --help  显示帮助信息
      --version   显示版本号

Use "grabby [command] --help" for more information about a command.
"""
    print(help_text)

if __name__ == "__main__":
    main()
