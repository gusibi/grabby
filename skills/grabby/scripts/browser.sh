#!/usr/bin/env bash
# Browser-driven scraping: health, connected browsers, extract, screenshot,
# whitelisted page scripts and in-page fetch. Borrows the user's logged-in browser.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/_lib.sh"

usage() {
  cat <<'EOF'
Usage:
  browser.sh health
  browser.sh browsers
  browser.sh extract <url> [browser]
  browser.sh screenshot <url> [browser]
  browser.sh run-script <url> <script> [browser]
  browser.sh fetch <page-url> <request-url> [browser]

Whitelisted scripts for run-script:
  youtube.initialPlayerResponse | bilibili.initialState
  page.extractJsonLd | page.readMeta

Environment:
  GRABBY_SERVER_URL  Default: http://localhost:5040
  GRABBY_API_TOKEN   Optional token for /api/* endpoints
EOF
}

cmd="${1:-}"
shift || true

case "$cmd" in
  health)
    gb_get /open/api/health
    ;;
  browsers)
    gb_get /api/browsers
    ;;
  extract)
    url="${1:-}"; browser="${2:-}"
    [[ -z "$url" ]] && { usage; exit 2; }
    gb_post /api/extract "$(printf '{"url":%s%s}' "$(json_string "$url")" "$(browser_field "$browser")")"
    ;;
  screenshot)
    url="${1:-}"; browser="${2:-}"
    [[ -z "$url" ]] && { usage; exit 2; }
    gb_post /api/screenshot "$(printf '{"url":%s,"fullPage":true%s}' "$(json_string "$url")" "$(browser_field "$browser")")"
    ;;
  run-script)
    url="${1:-}"; script="${2:-}"; browser="${3:-}"
    [[ -z "$url" || -z "$script" ]] && { usage; exit 2; }
    gb_post /api/run_page_script "$(printf '{"url":%s,"script":%s,"params":{},"visible":false%s}' \
      "$(json_string "$url")" "$(json_string "$script")" "$(browser_field "$browser")")"
    ;;
  fetch)
    url="${1:-}"; request_url="${2:-}"; browser="${3:-}"
    [[ -z "$url" || -z "$request_url" ]] && { usage; exit 2; }
    gb_post /api/fetch_in_page "$(printf '{"url":%s,"requestUrl":%s,"method":"GET","credentials":"include"%s}' \
      "$(json_string "$url")" "$(json_string "$request_url")" "$(browser_field "$browser")")"
    ;;
  ""|help|-h|--help)
    usage
    ;;
  *)
    usage
    exit 2
    ;;
esac
