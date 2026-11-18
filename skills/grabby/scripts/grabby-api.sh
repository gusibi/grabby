#!/usr/bin/env bash
# Read Grabby's curated AI news & daily reports (no browser needed).
# Browser scraping lives in the per-platform scripts:
#   browser.sh  twitter.sh  reddit.sh  xiaohongshu.sh
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/_lib.sh"

usage() {
  cat <<'EOF'
Usage:
  grabby-api.sh feed [all|news|blog|youtube|podcast] [limit] [category] [score_min]
  grabby-api.sh picks [date] [limit]
  grabby-api.sh ingest <url> [type]
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
  feed)
    type="${1:-all}"; limit="${2:-20}"; category="${3:-}"; score_min="${4:-}"
    args=(--data-urlencode "type=$type" --data-urlencode "limit=$limit")
    [[ -n "$category" ]] && args+=(--data-urlencode "category=$category")
    [[ -n "$score_min" ]] && args+=(--data-urlencode "score_min=$score_min")
    gb_get /open/api/feed "${args[@]}"
    ;;
  picks)
    date_arg="${1:-}"; limit="${2:-20}"
    args=(--data-urlencode "limit=$limit")
    [[ -n "$date_arg" ]] && args+=(--data-urlencode "date=$date_arg")
    gb_get /open/api/feed/picks "${args[@]}"
    ;;
  ingest)
    target_url="${1:-}"; hint_type="${2:-auto}"
    [[ -z "$target_url" ]] && { usage; exit 2; }
    payload="{\"url\":$(json_string "$target_url"),\"type\":$(json_string "$hint_type"),\"analyze\":true}"
    gb_post /open/api/feed/ingest "$payload"
    ;;
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
