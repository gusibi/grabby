#!/usr/bin/env bash
# Xiaohongshu (小红书) scraping via the user's logged-in browser:
# note detail (+comments), search, user notes.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/_lib.sh"

usage() {
  cat <<'EOF'
Usage:
  xiaohongshu.sh note <url> [browser]
  xiaohongshu.sh search <query> [limit] [browser]
  xiaohongshu.sh user-notes <profile-url> [limit] [browser]

  note url is the explore URL including its xsec_token.

Environment:
  GRABBY_SERVER_URL  Default: http://localhost:5040
  GRABBY_API_TOKEN   Optional token for /api/* endpoints
EOF
}

cmd="${1:-}"
shift || true

case "$cmd" in
  note)
    url="${1:-}"; browser="${2:-}"
    [[ -z "$url" ]] && { usage; exit 2; }
    gb_post /api/xiaohongshu/note "$(printf '{"url":%s%s}' "$(json_string "$url")" "$(browser_field "$browser")")"
    ;;
  search)
    query="${1:-}"; limit="${2:-50}"; browser="${3:-}"
    [[ -z "$query" ]] && { usage; exit 2; }
    gb_post /api/xiaohongshu/search "$(printf '{"query":%s,"limit":%s%s}' \
      "$(json_string "$query")" "$limit" "$(browser_field "$browser")")"
    ;;
  user-notes)
    url="${1:-}"; limit="${2:-50}"; browser="${3:-}"
    [[ -z "$url" ]] && { usage; exit 2; }
    gb_post /api/xiaohongshu/user_notes "$(printf '{"url":%s,"limit":%s%s}' \
      "$(json_string "$url")" "$limit" "$(browser_field "$browser")")"
    ;;
  ""|help|-h|--help)
    usage
    ;;
  *)
    usage
    exit 2
    ;;
esac
