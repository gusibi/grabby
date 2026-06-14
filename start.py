#!/usr/bin/env python3
"""
Grabby Server 启动脚本
用法: python start.py [go-run-args...]
"""

import os
import subprocess
import sys


def main():
    project_root = os.path.dirname(os.path.abspath(__file__))
    server_dir = os.path.join(project_root, "go-server")

    cmd = ["go", "run", ".", *sys.argv[1:]]
    subprocess.run(cmd, cwd=server_dir)


if __name__ == "__main__":
    main()
