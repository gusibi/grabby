#!/bin/bash
# Grabby Server 启动脚本

cd "$(dirname "$0")/go-server" || exit 1

exec go run . "$@"
