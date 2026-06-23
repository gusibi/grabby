#!/usr/bin/env bash
# Shared helpers for grabby skill scripts. Source this from each platform script.
set -euo pipefail

BASE="${GRABBY_SERVER_URL:-http://localhost:5040}"

AUTH_ARGS=()
if [[ -n "${GRABBY_API_TOKEN:-}" ]]; then
  AUTH_ARGS=(-H "Authorization: Bearer ${GRABBY_API_TOKEN}")
fi

die() {
  echo "$1" >&2
  exit "${2:-2}"
}

# Escape a value into a JSON string literal.
json_string() {
  local value="$1"
  value="${value//\\/\\\\}"
  value="${value//\"/\\\"}"
  value="${value//$'\n'/\\n}"
  value="${value//$'\r'/\\r}"
  value="${value//$'\t'/\\t}"
  printf '"%s"' "$value"
}

# Append a `,"browser":"..."` fragment when a browser name is given.
# Usage: browser_field "$browser"
browser_field() {
  local browser="${1:-}"
  [[ -n "$browser" ]] && printf ',"browser":%s' "$(json_string "$browser")"
}

# POST a JSON body to a path. Usage: gb_post /api/extract '{"url":"..."}'
# The ${arr[@]+...} guard keeps an empty AUTH_ARGS safe under `set -u` (bash 3.2).
gb_post() {
  curl -sS -X POST "$BASE$1" \
    ${AUTH_ARGS[@]+"${AUTH_ARGS[@]}"} \
    -H "Content-Type: application/json" \
    -d "$2"
}

# GET a path, passing through extra curl args (e.g. --data-urlencode k=v).
# Usage: gb_get /open/api/ai/daily --data-urlencode type=daily
gb_get() {
  local path="$1"
  shift
  curl -sS -G "$BASE$path" ${AUTH_ARGS[@]+"${AUTH_ARGS[@]}"} "$@"
}
