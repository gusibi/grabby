#!/usr/bin/env bash
# Read Grabby's curated AI news & daily reports (no browser needed).
# Browser scraping lives in the per-platform scripts:
#   browser.sh  twitter.sh  reddit.sh  xiaohongshu.sh
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/_lib.sh"

usage() {
  cat <<'EOF'
Usage:
  grabby-api.sh daily [daily|morning|evening] [YYYY-MM-DD]
  grabby-api.sh categories
  grabby-api.sh items <category> [limit]
  grabby-api.sh quality [score_min] [days] [limit]
  grabby-api.sh stats

Browser scraping is in the per-platform scripts:
  browser.sh   - health, browsers, extract, screenshot, run-script, fetch
  twitter.sh   - search, timeline, likes
  reddit.sh    - thread, subreddit, search
  xiaohongshu.sh - note, search, user-notes

Environment:
  GRABBY_SERVER_URL  Default: http://localhost:5040
  GRABBY_API_TOKEN   Optional token for /api/* endpoints
EOF
}

cmd="${1:-}"
shift || true

case "$cmd" in
  daily)
    type="${1:-daily}"; date_arg="${2:-}"
    args=(--data-urlencode "type=$type")
    [[ -n "$date_arg" ]] && args+=(--data-urlencode "date=$date_arg")
    gb_get /open/api/ai/daily "${args[@]}"
    ;;
  categories)
    gb_get /open/api/ai/categories
    ;;
  items)
    category="${1:-}"; limit="${2:-10}"
    [[ -z "$category" ]] && { usage; exit 2; }
    gb_get /open/api/ai/items \
      --data-urlencode "category=$category" \
      --data-urlencode "limit=$limit"
    ;;
  quality)
    score_min="${1:-6}"; days="${2:-7}"; limit="${3:-10}"
    gb_get /open/api/ai/quality \
      --data-urlencode "score_min=$score_min" \
      --data-urlencode "days=$days" \
      --data-urlencode "limit=$limit"
    ;;
  stats)
    gb_get /open/api/ai/stats
    ;;
  ""|help|-h|--help)
    usage
    ;;
  *)
    usage
    exit 2
    ;;
esac
