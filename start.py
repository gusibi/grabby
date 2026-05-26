#!/usr/bin/env python3
"""
MCP Web Capture Server 启动脚本
用法: python start.py [uv-run-args...]

依赖 uv (https://docs.astral.sh/uv/):
  curl -LsSf https://astral.sh/uv/install.sh | sh

uv run 会自动处理虚拟环境和依赖，无需手动创建 venv。
"""

import os
import subprocess
import sys


def main():
    project_root = os.path.dirname(os.path.abspath(__file__))
    server_dir = os.path.join(project_root, "python-server")

    # uv run 自动处理：虚拟环境创建 + 依赖安装 + 运行
    cmd = ["uv", "run", "python", "main.py", *sys.argv[1:]]
    subprocess.run(cmd, cwd=server_dir)


if __name__ == "__main__":
    main()
