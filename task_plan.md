# Task Plan: Echo API Route Refactor

## Goal
Refactor the Go service to use Echo for HTTP routing, split public APIs under `/open/api`, and protect `/api` management/browser/config APIs with shared authentication middleware.

## Current Phase
Phase 4

## Phases

### Phase 1: Requirements & Discovery
- [x] Confirm API split and middleware expectation
- [x] Inspect current Go router, handlers, auth, frontend API calls, and extension registration
- [x] Document findings in findings.md
- **Status:** complete

### Phase 2: Planning & Structure
- [x] Choose conservative Echo migration with wrapped existing handlers
- [x] Decide public vs protected route groups
- [x] Update code structure
- **Status:** complete

### Phase 3: Implementation
- [x] Add Echo dependency and token config
- [x] Replace ServeMux routing with Echo groups and middleware
- [x] Remove per-handler auth checks made redundant by middleware
- [x] Update frontend public-read paths to `/open/api`
- [x] Update browser extension registration auth support if available
- **Status:** complete

### Phase 4: Testing & Verification
- [ ] Run Go tests
- [x] Run frontend build/type checks if practical
- [x] Document verification in progress.md
- **Status:** in_progress

### Phase 5: Delivery
- [ ] Review changed files
- [ ] Summarize result and any residual risk
- **Status:** pending

## Key Questions
1. Which APIs are public? Public read/data APIs go under `/open/api`; management, configuration, and browser interaction APIs stay under `/api`.
2. How is auth enforced? Echo middleware on `/api`, skipping auth endpoints, accepting cookie or fixed token.

## Decisions Made
| Decision | Rationale |
|----------|-----------|
| Use Echo groups with `echo.WrapHandlerFunc` | Meets framework requirement while minimizing churn in existing handlers. |
| Add `GRABBY_API_TOKEN` | Explicit fixed token separate from admin login key. |
| No compatibility aliases for old public `/api` paths | User asked for clear `/open/api` vs `/api` split. |
| Keep sources/logs/browsers under protected `/api` | They are settings/config/browser management surfaces. |

## Errors Encountered
| Error | Attempt | Resolution |
|-------|---------|------------|
| `go get github.com/labstack/echo/v4@v4.13.4` failed due DNS in sandbox and escalation was rejected due usage limit | 1 | Added dependency to `go.mod`; verification remains blocked until module download/go.sum update is possible. |

## Notes
- Preserve user’s existing uncommitted changes.
- Avoid unrelated cleanup.
