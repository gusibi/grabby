# Task Plan: Grabby Desktop — Layout Switch + In-App Split Reader

## Goal
Raise the Grabby desktop (MyGO) reading experience to award-level quality:

1. **View switch** — toggle the feed between card and list layouts (persisted).
2. **In-app reading** — clicking an article opens a right-hand reader panel with the full
   extracted article; it can be closed; no browser hand-off for the primary path.
3. **Adaptive feed** — when the reader opens, the existing feed reflows/shrinks smoothly
   instead of being covered or clipped.

Quality bar: Awwwards / Webby / FWA level. Self-review screenshots in light + dark,
WebKit (the macOS WebView engine) and Chromium, desktop + narrow widths; iterate until
the result is genuinely polished, not merely functional.

Target files (hand-written UI, no build step):
- `go-server/desktop/dist/index.html`
- `go-server/desktop/dist/app.css`
- `go-server/desktop/dist/app.js`
- `readme.md` (feature docs)

## Current Phase
Complete — implementation, responsive QA, and verification finished.

## Phases

### Phase 1: Discovery
- [x] Map desktop UI structure, data flow, API shapes
- [x] Confirm "原文" is the server-extracted article (`/open/api/items/:id` + `/open/api/ai/analysis/:id`)
- [x] Confirm app shell: MyGO WebView, `desktop/dist` hand-written HTML/CSS/JS
- [x] Inventory motion / type / color tokens
- **Status:** complete

### Phase 2: Design
- [x] Define split-reader layout, width behavior, responsive breakpoints
- [x] Define card → list visual language
- [x] Define motion, focus, keyboard, reduced-motion behavior
- [x] Build static/mock QA harness + screenshot matrix
- **Status:** complete

### Phase 3: Implementation
- [x] `index.html`: workspace wrapper, reader shell, view switch, scrim
- [x] `app.css`: view switch, list rows, split reader, adaptive feed, motion
- [x] `app.js`: layout state/persistence, list renderer, reader open/close/load, read/star sync, prev/next, keyboard
- [x] Keep dialogs for sources/logs/settings; only article detail moves to the reader
- **Status:** complete

### Phase 4: Quality iteration
- [x] Screenshot matrix: today grid/list, library grid/list, reader open, reader narrow, dark
- [x] Fix hierarchy, spacing, transition, overflowing text, long CJK/English titles
- [x] Accessibility pass: focus, labels, Esc, reduced motion
- [x] Re-screenshot and compare
- **Status:** complete

### Phase 5: Verification & delivery
- [x] `node --check app.js`
- [x] `go build -tags desktop ./...`
- [x] `go test ./...`
- [x] Manual smoke of view switch, open/close, prev/next, star/read
- [x] Update `readme.md`
- [x] Summarize outcome and residual risks
- **Status:** complete

## Key Decisions
| Decision | Rationale |
|----------|-----------|
| In-app reader shows extracted article HTML, not a remote iframe | Sites set `X-Frame-Options`/CSP; MyGO WebView has no address-bar browser. Extracted full text is reliable, typographically consistent, and works offline. A secondary "浏览器" link stays for pages with no extraction. |
| Reader is a sibling pane in a flex workspace, not a modal | True split view, preserves feed scroll position, allows returning to the feed without losing state. |
| Panel width animates via `width` + clipped inner shell | Avoids text reflow during the animation; motion stays smooth and cheap. The panel itself stays opaque so the feed cannot show through mid-animation. |
| Feed adapts via `.is-reading` + width media rules | Delivers a real "shrunk" feed; class-based rules work on macOS 12 WebKit where container queries may be missing. |
| Split breakpoint at 1121px; full-width overlay at ≤1120px | At 1180 (default window) the split leaves a 420px feed and 516px reader — both usable. Below that a split is too cramped, so the article takes over the content area (Apple News / iPad pattern). Window minimum is 900px. |
| Reader fills all remaining width; the feed stays a 420px compact column | The earlier `clamp(440px, 54%, 820px)` cap left the reader static at 820px on large displays while the feed kept growing. The panel now grows with the window and the article keeps a centered 43rem measure inside it. |
| The inner reader shell gets the final pixel width, not a percentage of the panel | A percentage (`54%`) on the absolutely-positioned shell resolved against the panel's animated width, so the article was squeezed (e.g. 470px inside an 840px panel). The shell width is now derived from `100vw - sidebar - feed`, eliminating both the squeeze and reflow during the open animation. |
| Reading mode simplifies the 今天 chrome | Hides the connection bar, hides the redundant "已连接浏览器" stat, and fits the remaining stats on one row, so the compact feed starts with content instead of stacked dashboards. |
| Compact cards use uniform 82px covers and 2-line clamps | The lead card no longer half-emphasises: all cards share baselines and kicker height, making the shrunken grid read as a deliberate system. |
| Layout preference persisted in `localStorage` | Expected desktop behavior; avoids re-choosing on every launch. |
| List layout is a first-class editorial list, not a table | Matches Apple News visual language: cover chip, kicker, title, summary, date/score. |
| Keyboard: `Esc` close, `J`/`K` next/prev, `V` toggle layout, `/` search | Low-friction power-user reading without discoverability debt. |

## QA / Quality Bar
- Motion: 500–600 ms, `cubic-bezier(.22,1,.36,1)`, interruptible; reader content fades up once.
- Type: display serif for article body; SF/system for UI. Measure ~43–70 chars per line.
- Reader: title hierarchy, AI insight callout, reading time, progress line, sticky action bar.
- Feed: no clipping, no horizontal scroll, no layout jump; scroll position preserved.
- States: loading skeleton, empty content fallback, fetch error + retry.
- Dark mode: no low-contrast borders, gradients stay subtle.
- Reduced motion: width/opacity/fade become near-instant.
- Focus visible on every interactive element; close restores focus to the article.

## QA Results
- Width matrix (WebKit): 1440 / 1180 / 1121 split (feed 420 throughout, reader 776 / 516 / 457);
  1920 adds a 1256px reader; 1100 / 1024 / 820 overlay at full content width; 900px app minimum
  covered by the sidebar and 820 run.
- Reader panel fills the remaining workspace width; the article body keeps a 43rem measure,
  centered. The shell width matches the panel exactly (no right-side dead space).
- Sidebar footer links (浏览器插件 / 项目网站 / Releases) fit on one line at 244px; the sidebar
  now shrinks properly at the 600px minimum height, keeping the status card and links visible.
- No horizontal overflow, no console/page errors in WebKit or Chromium, light or dark.
- Feed scroll preserved when opening/closing, J/K, and star toggles verified.
- Esc restores focus to the active row/card; layout choice persists after close.
- Loading skeleton, no-AI-analysis path, and empty-body fallback verified.
- `node --check app.js` passes; `go build -tags desktop ./...` and `go test ./...` pass.

## Errors Encountered
| Error | Attempt | Resolution |
|-------|---------|------------|
| Playwright browser build mismatch (`webkit-2388` missing) | 1 | Ran `npx playwright install chromium webkit`. |
| `$("reader-bar").classList` null on open | 1 | `<header class="reader-bar">` had no id; added `id="reader-bar"`. |
| Compact score chips rendered at cover-letter size | 1 | Scoped the big monogram rule to `.cover-letter` instead of all `.card-cover span`. |
| Row focus ring clipped by `.item-list { overflow:hidden }` | 1 | Switched rows to an inset `box-shadow` focus ring. |
| Reader panel looked translucent mid-animation | 1 | Removed opacity transition from `.reader`; only `.reader-shell` fades/translates. |
| Overlay reader wider than the workspace at narrow widths | 1 | Changed overlay width to `100%` of the workspace instead of a viewport-based min. |
| 4 stats wrapped 3+1 in split view | 1 | In reading mode hide the redundant connection stat and flex the remaining three equally. |
| Long source kicker wrapped and pushed card titles off-baseline | 1 | Single-line kicker with ellipsis at 9.5px in compact mode; all titles align. |
| "正文" divider appeared above the empty-body fallback | 1 | Render the divider only when `contentText` exists. |
| Search field stayed visible on 今天 / AI 分析 despite the `hidden` attribute | 1 | `.search { display: flex }` outranked the UA `[hidden]` rule; added `.search[hidden] { display: none }` (library search still shows). |
| Article squeezed into the left half of a wide reader panel (470px inside an 840px panel) | 1 | The shell's `width: var(--reader-width)` percentage resolved against the animating panel, not the workspace. The shell now uses `calc(100vw - sidebar - feed)`, matching the final panel width exactly. |
| Sidebar content overflowed the 600px minimum window height, hiding status and footer | 1 | Added `min-height: 0` to the sidebar so its nav scrolls instead of stretching the grid row. |

## Follow-up: sidebar links
Added a compact footer nav (always visible) with three external links:
- 浏览器插件 → Chrome Web Store listing
- 项目网站 → https://grabby.eztoolab.com
- Releases → https://github.com/gusibi/grabby/releases
All use `target="_blank" rel="noopener"`; MyGO's default window-open behavior sends http(s)
links to the system browser.

## Follow-up: site setup guide
Refreshed `site/src/pages/setup.astro` for the shipped desktop app:
1. desktop install (macOS / Windows / Linux), first launch, data dirs, reading features;
2. extension install; 3. extension connect (CONNECT_ID, port fallback); 4. AI models;
5. optional Go-only server with env table; 6. verify; 7. Claude Desktop;
plus troubleshooting (quarantine, SmartScreen, Go build, occupied port).
Python backend references removed from the guide and the homepage copy; zh/en i18n keys
added in `BaseLayout.astro`; highlighter bugs fixed; Astro build and WebKit checks pass.

