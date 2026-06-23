#!/usr/bin/env bash
# Reddit scraping via the user's logged-in browser: thread, subreddit, search.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/_lib.sh"

usage() {
  cat <<'EOF'
Usage:
  reddit.sh thread <url> [browser]
  reddit.sh subreddit <name> [limit] [browser]
  reddit.sh search <query> [subreddit] [sort] [limit] [browser]

  subreddit name is without r/ (e.g. golang).
  search sort: relevance | new | top | comments (empty string to skip).

Environment:
  GRABBY_SERVER_URL  Default: http://localhost:5040
  GRABBY_API_TOKEN   Optional token for /api/* endpoints
EOF
}

cmd="${1:-}"
shift || true

case "$cmd" in
  thread)
    url="${1:-}"; browser="${2:-}"
    [[ -z "$url" ]] && { usage; exit 2; }
    gb_post /api/reddit/thread "$(printf '{"url":%s%s}' "$(json_string "$url")" "$(browser_field "$browser")")"
    ;;
  subreddit)
    name="${1:-}"; limit="${2:-100}"; browser="${3:-}"
    [[ -z "$name" ]] && { usage; exit 2; }
    gb_post /api/reddit/subreddit "$(printf '{"subreddit":%s,"limit":%s%s}' \
      "$(json_string "$name")" "$limit" "$(browser_field "$browser")")"
    ;;
  search)
    query="${1:-}"; subreddit="${2:-}"; sort="${3:-}"; limit="${4:-100}"; browser="${5:-}"
    [[ -z "$query" ]] && { usage; exit 2; }
    body="$(printf '{"query":%s,"limit":%s' "$(json_string "$query")" "$limit")"
    [[ -n "$subreddit" ]] && body+="$(printf ',"subreddit":%s' "$(json_string "$subreddit")")"
    [[ -n "$sort" ]] && body+="$(printf ',"sort":%s' "$(json_string "$sort")")"
    body+="$(browser_field "$browser")}"
    gb_post /api/reddit/search "$body"
    ;;
  ""|help|-h|--help)
    usage
    ;;
  *)
    usage
    exit 2
    ;;
esac
