# Findings & Decisions

## Requirements
- Refactor Go service routing to use Echo rather than the current `http.ServeMux` router.
- Public, no-login APIs should begin with `/open/api`.
- Management, settings/configuration, and browser-extension interaction APIs should begin with `/api`.
- Protected `/api` authentication must be implemented as middleware, not repeated inside each handler.
- Protected APIs support cookie session authentication and a fixed token.

## Research Findings
- Current router is `go-server/internal/interfaces/http/router.go` using `net/http.ServeMux`.
- Current cookie auth lives in `go-server/internal/interfaces/http/auth.go` with `grabby_admin_session`.
- Some handlers already manually call `auth.isAuthenticated` for source mutations; these checks become redundant when the route group is protected.
- Frontend API calls are centralized in `go-server/frontend/src/lib/api.ts`, with item list URLs assembled in `go-server/frontend/src/App.tsx`.
- Chrome extension registration currently posts to `/api/browsers/register` in `chrome-extension/lib/websocket.js`.
- Source configuration, logs, and browser lists are used by settings/config/browser-management views, so they belong under protected `/api`.

## Technical Decisions
| Decision | Rationale |
|----------|-----------|
| Use Echo route groups | Cleanly separates `/open/api` and `/api`, and applies auth middleware once. |
| Keep existing handler function signatures initially | Reduces risk and avoids a large unrelated rewrite. |
| Use `Authorization: Bearer` and `X-Grabby-Token` | Covers common API-token usage and simple extension configuration. |
| Add extension API Token setting | Browser registration is protected and extension fetches cannot rely on app cookie authentication. |

## Issues Encountered
| Issue | Resolution |
|-------|------------|
| Echo dependency could not be downloaded in this environment | Added `go.mod` requirement and recorded that `go.sum`/tests need a successful `go get` or `go mod tidy` with network. |

## Resources
- `go-server/internal/interfaces/http/router.go`
- `go-server/internal/interfaces/http/handlers.go`
- `go-server/internal/interfaces/http/ai_handlers.go`
- `go-server/internal/interfaces/http/auth.go`
- `go-server/frontend/src/lib/api.ts`
- `chrome-extension/lib/websocket.js`

## Visual/Browser Findings
- None.
