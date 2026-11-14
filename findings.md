# Findings & Decisions

## Goal recap
Grabby Desktop (`go-server/desktop/dist`, vanilla HTML/CSS/JS embedded in the Go binary via
`go:embed`) needs an award-level split reader, a card/list switch, and an adaptive feed.

## Architecture facts
- Desktop shell: MyGO v0.8.3, system WebView; macOS uses `TitleBarHidden`, Apple News style.
- UI source = `go-server/desktop/dist/{index.html,app.js,app.css}`. Hand-written, no build step;
  tracked in git deliberately (see `.gitignore` exception).
- Main layout today: `#app` grid `sidebar | main`; `.main` = topbar + `.content`.
- `render()` writes `#content.innerHTML` and resets `scrollTop = 0` on every full render.
  The reader must therefore update independently to preserve feed scroll.
- Content clicks use one delegated listener reading `[data-action]`; `detail` currently opens a modal.
- `openDetail(id)` fetches:
  - `GET /open/api/items/:id` → `{ item, html_content, summary_html }`
  - `GET /open/api/ai/analysis/:id` → `{ analysis }` with `quality_score`, `ai_category`,
    `ai_summary`, `ai_comment`, `ai_tags`, `model_used`, `processed_at`.
- Mark read: `POST /api/items/:id/read {read_status:1}`; star: `POST /api/items/:id/star {starred}`.
- Items list: `GET /open/api/items?limit=60&cursor=…`; quality list: `/open/api/ai/items?score_min=7…`.
- DB sample: 15,213 items, fields `origin_source`, `title`, `summary`, `content`, `category`,
  `published_at`, `read_status`, `starred`. Quality score lives in `ai_analyses`.
- The page already has one `.modal` system (`dialog-modal`), `toast`, view routing in `state.view`.
- CSS tokens: `--bg --surface --surface-alt --border --text --muted --faint --accent --accent-soft
  --card-radius --card-shadow --sidebar-width --content-pad`, mac + win overrides, dark mode via
  `prefers-color-scheme`.
- `safeHtml()` sanitizes extracted HTML (removes scripts/styles/iframes/forms, strips `on*`, `style`,
  `javascript:`/`data:` URLs, forces links to `_blank`). Reuse it for the reader body.

## Design direction
- **Split view, not overlay (desktop):** feed stays live and scrollable; reader is a sibling pane.
  The feed stays a fixed 420px compact column while reading; the reader takes every remaining pixel
  (`calc(100vw - sidebar - 420px)`), with no max-width cap. The inner shell uses the same pixel
  width so the article does not reflow during the open animation.
- **Article measure:** `.reader-body` stays capped at 43rem and centered inside the panel. The
  panel now fills wide windows (e.g. 1256px at 1920), while the text column keeps a readable
  60–85 character measure.
- **Shrinking feed:** `.workspace.is-reading` reduces page title, card cover height, card padding,
  title/summary sizes, grid min column, and hides summaries in list mode. This reads as an
  intentional "reading mode" rather than a squeezed desktop page.
- **Reading-mode chrome:** the connection bar is hidden (the sidebar already shows status), the
  redundant "已连接浏览器" stat is dropped, and the remaining three stats share one row. The daily
  report card collapses to one preview line without its meta row.
- **Breakpoints:** split view at `min-width: 1121px`. At `max-width: 1120px` the reader becomes a
  full-width right overlay (no clipped feed sliver); the app window minimum is 900px
  (`main_desktop.go`), so the overlay always has room. At `max-width: 720px` reader padding and
  type drop one step.
- **Sidebar footer:** a one-line link row (浏览器插件 / 项目网站 / Releases) above the
  "Grabby Desktop" label. External links rely on MyGO's default `target="_blank"` behavior, which
  opens http(s) URLs in the system browser. The sidebar has `min-height: 0` so the footer and
  status card stay visible at the 600px minimum window height.
- **Card typography in reading mode:** uniform 82px covers (lead card no longer spans or grows),
  one-line ellipsised kickers so titles share a baseline, 13.5px titles, 11.5px summaries clamped
  to two lines.
- **List view language:** editorial rows, gradient cover chip with monogram + score, kicker,
  title (1–2 lines), summary (1–2 lines), right-aligned date + category. Hover reveals accent rail.
  Selected row keeps the accent rail + tinted surface.
- **Reader language:** sticky top bar (close, source, prev/next, star), 2px progress line,
  large serif display title, meta with reading time, AI "精读" card with score ring/tags/comment,
  summary pull-quote, long-form body, footer with source + browser fallback. The "正文" divider
  is only rendered when extracted body text exists.
- **Motion:** width/opacity 520–620 ms `cubic-bezier(.22,1,.36,1)`; content staggered fade-up;
  reduced-motion honored by existing global media query (add explicit rules).
- **Persistence:** `localStorage["grabby_layout"] = "grid" | "list"`.
- **Keyboard:** Esc closes; J/K moves to next/previous article in the current list; V toggles layout
  (only outside inputs); `/` focuses search.

## QA approach
- Mock API server serving `desktop/dist` + realistic items (long CJK/English titles, rich HTML
  article, AI analysis, no-content fallback). Screenshot with Playwright WebKit (macOS WebView
  engine) and Chromium at 1440×900, 1180×800, 1121, 1100, 1024×768, 900, 820, light + dark.
- Screenshot matrix: today/grid, today/list, library/grid, library/list, reader loading,
  reader loaded, reader scrolled through image/code/table, reader narrow overlay,
  empty-body fallback, missing AI analysis.
- Inspect focus states, scroll preservation, J/K navigation, star toggle and layout persistence
  via Playwright; measure feed/reader widths and horizontal overflow.

## QA results
- Split: at 1180 the feed is 420px and the reader 516px; at 1440 420 / 776; at 1121 420 / 457;
  at 1920 420 / 1256. No overflow in either pane, no page errors in WebKit or Chromium, light or dark.
- Overlay: at 1100 / 1024 / 820 the reader fills the content area (856 / 780 / 576) and the close
  button remains fully visible. The old 600px cap overflowed behind the sidebar below 844px, so
  the overlay now uses the full workspace width.
- The reader shell width matches the panel at every size (previous bug: the article rendered at
  470px inside an 840px panel because the shell resolved `54%` against the animating panel).
- Sidebar: the three footer links fit on one line at 244px; status card + links stay visible at
  1180×780 and 900×600 after adding `min-height: 0` to the sidebar.
- Interaction: V toggles list/grid while reading; J/K move between articles; the star syncs
  between panel and feed; Esc closes and restores focus to the active row; layout choice persists.
- States: loading skeleton shows the real title; empty body shows summary + AI analysis + fallback
  card; a missing analysis response simply omits the AI card.
- Verification: `node --check app.js`, `go build -tags desktop ./...`, `go test ./...` all pass.

## Site guide (follow-up)
- `site/` is Astro 5 + Tailwind; `src/pages/setup.astro` is the 使用指南, deployed with
  `npm run build && wrangler pages deploy dist`.
- i18n lives in `BaseLayout.astro` as a `zh-CN` / `en` object; a client script replaces
  `textContent` of every `[data-i18n]` element on load, so any inner markup (`<strong>`,
  `<code>`) is wiped. Put translations on plain text elements only.
- `hlShell()` previously ran a quote-highlighting regex over comment spans it had just
  inserted, and `hlShell(hlJson(...))` re-highlighted class attributes, dumping raw
  `<span class="...">` into code blocks. Both highlighters now process one line at a time
  and JSON lines are highlighted once.
- Backend facts used in the guide: single self-contained Go binary; built-in port 5040 with
  automatic 5041/5042 fallback; `CONNECT_ID` defaults to `browser-tools`; `GRABBY_ADMIN_KEY`
  / `GRABBY_API_TOKEN` guard web/API; Go 1.27.3+; data dirs per OS.

## Topbar title (follow-up)
- The page title now renders inside the topbar row (`#topbar-head`), left of the actions, so it
  starts at the top of the content area instead of after the toolbar. Descriptions stay in the
  scrollable content as their first line.
- Page titles use `var(--accent)`; on macOS that is the brand red, on Windows it follows the
  Fluent accent. Section headings stay `--text` for hierarchy.
- TITLES doubles as the page-title map (`today` → 今天, `library` → 全部内容 …); the topbar date
  meta for 今天 is generated by `syncTopbarHead()`.
- The preview harness lives in `go-server/.tmp/ui-preview/` (gitignored); Playwright is borrowed
  from another checkout via a temporary `node_modules` symlink when needed.

## Residual risks
- The reader uses extracted article HTML. Pages the server cannot extract show the fallback link
  instead of an in-app body; switching to a remote iframe would not fix that (framing is blocked).
- The old WebKit in macOS 12 has no `color-mix`/`text-wrap: pretty` support; the UI already has
  fallbacks, and macro layout uses width media queries rather than container queries.

## Open questions (resolved)
- *Live website in the panel?* No. Remote pages can't be iframed reliably; use extracted full text,
  keep an external link as secondary fallback.
- *Should clicking a card also mark read?* Yes, silently (Apple News behavior), with optimistic
  UI update and no feed reload.
