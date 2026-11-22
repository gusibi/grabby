#!/usr/bin/env bash
# Grabby subscription text capture — one-command acceptance.
#
# Runs the repository's full Go regression suite plus the focused acceptance
# tests. Every fixture is local (httptest + temporary SQLite): no public
# network, browser, login, or model API is required.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root/go-server"

echo "==> go test ./... (full regression)"
go test -count=1 ./...

echo
echo "==> focused subscription acceptance tests"
go test -count=1 -v -run 'TestAcceptance' \
  ./internal/application/scraping ./internal/application/scheduler ./internal/interfaces/http

echo
echo "All acceptance tests passed."
