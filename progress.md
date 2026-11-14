# Progress Log

## Session: 2026-11-14

### Phase 1 — Discovery
- **Status:** complete
- Read `go-server/desktop/dist/index.html`, `app.js`, `app.css`.
- Read `go-server/main_desktop.go` to confirm MyGO/WebView embedding (default 1180×800, min 900).
- Read item detail / analysis handlers to confirm response shapes.
- Sampled the live SQLite DB for realistic item fields and AI scores.
- Confirmed no build step for `desktop/dist`; files are embedded and tracked.
- Identified the renderer's scroll-reset behavior as the main constraint for a split reader.

### Phase 2 — Design
- **Status:** complete
- Chose a flex workspace with an animated sibling reader pane over a modal.
- Chose class-based adaptive feed (works on macOS 12 WebKit) + width media rules.
- Fixed the split threshold at 1121px and the overlay at ≤1120px after testing the default window.
- Defined list-view editorial row anatomy and reader anatomy (serif title, AI card, progress).

### Phase 3 — Implementation
- **Status:** complete
- `index.html`: added `workspace` split, `reader` shell (progress, bar, scroll, body), scrim,
  view switch in the topbar.
- `app.js`: added layout state + `localStorage`, `card()`/`listRow()`/`feed()` renderers,
  `openReader`/`closeReader` engine with request sequencing, read/star sync, prev/next,
  J/K/V/Esc/`/` shortcuts, active-row tracking, reader progress.
- `app.css`: view switch, editorial list rows, split reader with fixed inner shell, reader
  typography, AI score ring, skeleton/fallback, adaptive feed, overlay and reduced motion.

### Phase 4 — Quality iteration
- **Status:** complete
- Ran the Playwright matrix in WebKit + Chromium, light + dark, at 1440, 1180, 1121, 1100,
  1024, 900, 820 and 560 (sanity).
- Fixed: missing `#reader-bar` id; oversized score chips; clipped row focus ring; translucent
  reader mid-animation; overlay width overflowing behind the sidebar; 3+1 wrapped stats;
  ragged card baselines from the lead card and long source kickers; "正文" divider above the
  empty-body fallback; search field visible on non-library views despite `hidden`.
- Final reading mode: connection bar hidden, three stats in one row, compact report card,
  uniform 82px card covers, aligned one-line kickers, 13.5/11.5px card type; feed scroll preserved.
- Verified: no horizontal overflow, no console errors, scroll preservation, focus restoration,
  star/J/K/V interactions, loading/empty/AI-missing states, article images/code/tables/footer.

### Phase 5 — Verification & delivery
- **Status:** complete
- `node --check go-server/desktop/dist/app.js` passes.
- `go build -tags desktop ./...` passes.
- `go test ./...` passes (all packages).
- `readme.md` now documents the view switch, in-app reader, adaptive feed, shortcuts and fallback.
- Planning files updated with decisions, findings and QA results.

### Follow-up — sidebar links + full-width reader
- **Status:** complete
- Added a sidebar footer link row: 浏览器插件 (Chrome Web Store), 项目网站
  (https://grabby.eztoolab.com), Releases (GitHub). Links open in the system browser.
- Fixed the reader width: the feed is a fixed 420px compact column while reading; the reader
  fills every remaining pixel (`calc(100vw - sidebar - 420px)`), so at 1920 it is 1256px wide
  instead of the old 820px cap.
- Fixed the inner shell width bug that squeezed the article to ~470px inside an 840px panel.
- Fixed the sidebar at the 600px minimum window height (`min-height: 0`) so status and links stay
  visible; tightened the link row gap so it stays on one line.
- Re-verified 1920 / 1440 / 1180 / 1121 split, 1100 / 1024 / 820 overlay, default and minimum
  sidebar heights; no overflow or console errors.

### Follow-up — project site setup guide
- **Status:** complete
- Rewrote `site/src/pages/setup.astro`: desktop app install (macOS / Windows / Linux), first
  launch and data dirs, reading features, in-app reader behaviour, sidebar links; extension
  install; extension connect (CONNECT_ID / browser-tools, port shift); AI models; optional
  Go-only server; verification; Claude Desktop; refreshed troubleshooting.
- Removed all Python backend references from the guide; homepage now says one Go service with
  SQLite instead of "Python + Go".
- Added zh-CN and en i18n keys in `BaseLayout.astro` for every new block.
- Fixed two pre-existing guide rendering bugs: dev-mode list items duplicated their leading
  text, and code-block comments / JSON responses showed raw highlight markup because the
  highlighter ran over already-highlighted HTML.
- Verified: `npm run build` succeeds; WebKit audit of both languages has no page errors and no
  missing translation keys; scrolled screenshots of every section reviewed.

## Files changed
- `go-server/desktop/dist/index.html`
- `go-server/desktop/dist/app.css`
- `go-server/desktop/dist/app.js`
- `readme.md`
- `site/src/pages/setup.astro`
- `site/src/layouts/BaseLayout.astro`
- `site/src/pages/index.astro`
- `task_plan.md`, `findings.md`, `progress.md` (planning)
- `.tmp/ui-preview/` (ignored QA harness, not committed)
