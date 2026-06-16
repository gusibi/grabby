#!/usr/bin/env bash
set -euo pipefail

BASE="${GRABBY_SERVER_URL:-http://localhost:5040}"

auth_args=()
if [[ -n "${GRABBY_API_TOKEN:-}" ]]; then
  auth_args=(-H "Authorization: Bearer ${GRABBY_API_TOKEN}")
fi

usage() {
  cat <<'EOF'
Usage:
  grabby-api.sh health
  grabby-api.sh daily [daily|morning|evening] [YYYY-MM-DD]
  grabby-api.sh categories
  grabby-api.sh items <category> [limit]
  grabby-api.sh quality [score_min] [days] [limit]
  grabby-api.sh extract <url> [browser]
  grabby-api.sh screenshot <url> [browser]
  grabby-api.sh browsers

Environment:
  GRABBY_SERVER_URL  Default: http://localhost:5040
  GRABBY_API_TOKEN   Optional token for /api/* endpoints
EOF
}

json_string() {
  local value="$1"
  value="${value//\\/\\\\}"
  value="${value//\"/\\\"}"
  value="${value//$'\n'/\\n}"
  value="${value//$'\r'/\\r}"
  value="${value//$'\t'/\\t}"
  printf '"%s"' "$value"
}

json_extract_payload() {
  local url="$1"
  local browser="${2:-}"

  if [[ -n "$browser" ]]; then
    printf '{"url":%s,"browser":%s}' "$(json_string "$url")" "$(json_string "$browser")"
  else
    printf '{"url":%s}' "$(json_string "$url")"
  fi
}

json_screenshot_payload() {
  local url="$1"
  local browser="${2:-}"

  if [[ -n "$browser" ]]; then
    printf '{"url":%s,"fullPage":true,"browser":%s}' "$(json_string "$url")" "$(json_string "$browser")"
  else
    printf '{"url":%s,"fullPage":true}' "$(json_string "$url")"
  fi
}

cmd="${1:-}"
shift || true

case "$cmd" in
  health)
    curl -sS "$BASE/open/api/health"
    ;;
  daily)
    type="${1:-daily}"
    date_arg="${2:-}"
    args=(-sS -G "$BASE/open/api/ai/daily" --data-urlencode "type=$type")
    if [[ -n "$date_arg" ]]; then
      args+=(--data-urlencode "date=$date_arg")
    fi
    curl "${args[@]}"
    ;;
  categories)
    curl -sS "$BASE/open/api/ai/categories"
    ;;
  items)
    category="${1:-}"
    limit="${2:-10}"
    if [[ -z "$category" ]]; then
      usage
      exit 2
    fi
    curl -sS -G "$BASE/open/api/ai/items" \
      --data-urlencode "category=$category" \
      --data-urlencode "limit=$limit"
    ;;
  quality)
    score_min="${1:-6}"
    days="${2:-7}"
    limit="${3:-10}"
    curl -sS -G "$BASE/open/api/ai/quality" \
      --data-urlencode "score_min=$score_min" \
      --data-urlencode "days=$days" \
      --data-urlencode "limit=$limit"
    ;;
  extract)
    url="${1:-}"
    browser="${2:-}"
    if [[ -z "$url" ]]; then
      usage
      exit 2
    fi
    curl -sS -X POST "$BASE/api/extract" \
      "${auth_args[@]}" \
      -H "Content-Type: application/json" \
      -d "$(json_extract_payload "$url" "$browser")"
    ;;
  screenshot)
    url="${1:-}"
    browser="${2:-}"
    if [[ -z "$url" ]]; then
      usage
      exit 2
    fi
    curl -sS -X POST "$BASE/api/screenshot" \
      "${auth_args[@]}" \
      -H "Content-Type: application/json" \
      -d "$(json_screenshot_payload "$url" "$browser")"
    ;;
  browsers)
    curl -sS "$BASE/api/browsers" "${auth_args[@]}"
    ;;
  ""|help|-h|--help)
    usage
    ;;
  *)
    usage
    exit 2
    ;;
esac
