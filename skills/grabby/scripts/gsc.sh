#!/usr/bin/env bash
# Google Search Console via the user's logged-in browser.
#
# Stage 0: capture the raw responses the real console page requests, so the
# parsers are written against real payloads. Data always comes from the
# browser — never from Google's documented API and never from a server-side
# request.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/_lib.sh"

usage() {
  cat <<'EOF'
Usage:
  gsc.sh record sites [browser] [visible]
  gsc.sh record performance <property> [days] [browser] [visible]

  property must be the canonical Search Console identifier:
    sc-domain:example.com        (Domain property)
    https://www.example.com/     (URL-prefix property, keep the trailing slash)

  days defaults to 28. visible=1 briefly activates the tab (try it if the
  background tab captures nothing).

  The command opens the real console in your browser and writes the captured
  responses to go-server/testdata/raw/ (override with GRABBY_RAW_DIR on the
  server side). It never calls the Google API and needs no Google credential.

  Requirements: the browser is logged in to Search Console and the extension is
  connected.

Environment:
  GRABBY_SERVER_URL  Default: http://localhost:5040
  GRABBY_API_TOKEN   Optional token for /api/* endpoints
EOF
}

visible_field() {
  local visible="${1:-}"
  case "$visible" in
    1|true|yes) printf ',"visible":true' ;;
  esac
}

cmd="${1:-}"
shift || true

case "$cmd" in
  record)
    kind="${1:-}"; shift || true
    case "$kind" in
      sites)
        browser="${1:-}"; visible="${2:-}"
        gb_post /api/gsc/record "$(printf '{"kind":"sites"%s%s}' \
          "$(browser_field "$browser")" "$(visible_field "$visible")")"
        ;;
      performance)
        property="${1:-}"; days="${2:-28}"; browser="${3:-}"; visible="${4:-}"
        [[ -z "$property" ]] && { usage; exit 2; }
        gb_post /api/gsc/record "$(printf '{"kind":"performance","property":%s,"days":%s%s%s}' \
          "$(json_string "$property")" "$days" "$(browser_field "$browser")" "$(visible_field "$visible")")"
        ;;
      *)
        usage
        exit 2
        ;;
    esac
    ;;
  ""|help|-h|--help)
    usage
    ;;
  *)
    usage
    exit 2
    ;;
esac
