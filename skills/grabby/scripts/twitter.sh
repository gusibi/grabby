#!/usr/bin/env bash
# X/Twitter scraping via the user's logged-in browser: search, timeline, likes.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/_lib.sh"

usage() {
  cat <<'EOF'
Usage:
  twitter.sh search <query> [limit] [browser]
  twitter.sh timeline [for_you|following] [limit] [browser]
  twitter.sh likes [handle] [limit] [browser]

Notes:
  Requires the target browser to be logged into X/Twitter.
  Empty/error result usually means not logged in or page structure changed.
  likes handle defaults to $GRABBY_TWITTER_HANDLE, else "gusibix".

Environment:
  GRABBY_SERVER_URL     Default: http://localhost:5040
  GRABBY_API_TOKEN      Optional token for /api/* endpoints
  GRABBY_TWITTER_HANDLE Default X handle for `likes` (default: gusibix)
EOF
}

cmd="${1:-}"
shift || true

case "$cmd" in
  search)
    query="${1:-}"; limit="${2:-40}"; browser="${3:-}"
    [[ -z "$query" ]] && { usage; exit 2; }
    gb_post /api/twitter/search "$(printf '{"query":%s,"limit":%s%s}' \
      "$(json_string "$query")" "$limit" "$(browser_field "$browser")")"
    ;;
  timeline)
    kind="${1:-for_you}"; limit="${2:-40}"; browser="${3:-}"
    gb_post /api/twitter/timeline "$(printf '{"kind":%s,"limit":%s%s}' \
      "$(json_string "$kind")" "$limit" "$(browser_field "$browser")")"
    ;;
  likes)
    # handle: CLI arg > $GRABBY_TWITTER_HANDLE > default gusibix
    handle="${1:-${GRABBY_TWITTER_HANDLE:-gusibix}}"; limit="${2:-60}"; browser="${3:-}"
    gb_post /api/twitter/likes "$(printf '{"handle":%s,"limit":%s%s}' \
      "$(json_string "$handle")" "$limit" "$(browser_field "$browser")")"
    ;;
  ""|help|-h|--help)
    usage
    ;;
  *)
    usage
    exit 2
    ;;
esac
