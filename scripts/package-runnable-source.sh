#!/usr/bin/env bash
set -euo pipefail

source_dir=$(cd "${1:-.}" && pwd -P)
if [[ ! -r "$source_dir/go-server/go.mod" ]]; then
  printf '无法读取 %s/go-server/go.mod，请从可访问源码的终端运行。\n' "$source_dir" >&2
  exit 1
fi

archive="$(dirname "$source_dir")/$(basename "$source_dir")-source-$(date +%Y%m%d-%H%M%S).zip"
stage=$(mktemp -d "$(dirname "$source_dir")/.grabby-package.XXXXXX")
trap 'rm -r -- "$stage"' EXIT
mkdir "$stage/extracted"

(
  cd "$source_dir"
  find . \( -name .git -o -name .claude -o -name .workbuddy -o -name .antigravitycli \
    -o -name node_modules -o -name vendor -o -name .venv -o -name .astro \
    -o -name dist -o -name build -o -name .tmp -o -name __pycache__ \
    -o -path './assets/logo-concepts-*' -o -path './apis/environments' \
    -o -path './go-server/testdata/raw' \) -prune -o \( -type f -o -type l \) -print0 |
    while IFS= read -r -d '' path; do
      case "${path##*/}" in
        .env.example|.env.sample|.env.template) ;;
        .env|.env.*|*.db|*.db-*|*.sqlite*|*.log|*.pem|*.key|*.artifact.json|.DS_Store|\
        browser_registry.json|desktop-settings.json|local_config.json|*.exe|*.dmg|*.deb|*.crx)
          continue ;;
      esac
      case "$path" in
        ./go-server/go-server|./go-server/grabby-server|./go-server/grabby-desktop|./go-cli/grabby)
          continue ;;
      esac
      printf '%s\0' "$path"
    done | COPYFILE_DISABLE=1 /usr/bin/tar --format zip -cf "$stage/source.zip" --null -T -
)

/usr/bin/unzip -tq "$stage/source.zip"
/usr/bin/unzip -q "$stage/source.zip" -d "$stage/extracted"
export GOWORK=off GOSUMDB=sum.golang.org

printf '\n正在从解压后的源码运行测试并构建。\n'
(
  cd "$stage/extracted/go-server"
  go test -mod=readonly ./...
  go build -mod=readonly -o "$stage/grabby-server" .
  if [[ $(uname -s) == Darwin ]]; then
    go build -mod=readonly -tags desktop -o "$stage/grabby-desktop" .
  fi
)

printf '\n正在用临时数据库启动服务并检查健康接口。\n'
python3 - "$stage" <<'PY'
import json
import os
from pathlib import Path
import socket
import subprocess
import sys
import time
from urllib.request import ProxyHandler, build_opener

stage = Path(sys.argv[1])
db_path = stage / 'smoke.db'
with socket.socket() as listener:
    listener.bind(('127.0.0.1', 0))
    port = listener.getsockname()[1]

environment = os.environ | {
    'HOST': '127.0.0.1', 'PORT': str(port), 'DB_PATH': str(db_path),
    'AI_ENABLED': 'false', 'AI_API_KEY': '', 'GRABBY_LOG_FILE': str(stage / 'smoke.log'),
    'GRABBY_RAW_DIR': str(stage / 'raw'),
}
opener = build_opener(ProxyHandler({}))
with (stage / 'startup.log').open('w') as log:
    server = subprocess.Popen([str(stage / 'grabby-server')], cwd=stage, env=environment,
                              stdout=log, stderr=subprocess.STDOUT)
    try:
        for _ in range(60):
            if server.poll() is not None:
                break
            try:
                url = f'http://127.0.0.1:{port}/open/api/health'
                with opener.open(url, timeout=1) as response:
                    if json.load(response).get('status') == 'ok' and db_path.is_file():
                        print('健康接口正常，SQLite 已在临时目录创建。')
                        break
            except (OSError, ValueError):
                pass
            time.sleep(0.25)
        else:
            sys.exit('健康检查超时，未生成最终 ZIP。')
        if server.poll() is not None:
            sys.exit(f'服务提前退出，退出码为 {server.returncode}。未生成最终 ZIP。')
    finally:
        if server.poll() is None:
            server.terminate()
            try:
                server.wait(timeout=5)
            except subprocess.TimeoutExpired:
                server.kill()
                server.wait()
PY

# Stage and destination are on the same filesystem. Refuse to overwrite a ZIP.
ln "$stage/source.zip" "$archive"
ls -lh "$archive"
