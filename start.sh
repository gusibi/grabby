#!/bin/bash
# Grabby Server 启动脚本
# 依赖 uv (https://docs.astral.sh/uv/)

cd "$(dirname "$0")/python-server" || exit 1

# uv run 会自动：检测 pyproject.toml → 创建虚拟环境 → 安装依赖 → 运行
exec uv run python main.py "$@"
