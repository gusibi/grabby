# Frontend App Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Split `go-server/frontend/src/App.tsx` into focused layout, feature, type, API, and utility files while preserving existing UI behavior.

**Architecture:** Keep `App.tsx` as the stateful application shell. Move render-only view sections into `components/layout` and `features/*`, shared data contracts into `types`, pure helpers into `lib`, and API wrappers into `lib/api.ts`. Do not introduce routing, Context, reducers, or new dependencies.

**Tech Stack:** React 19, TypeScript, Vite, Tailwind CSS, shadcn/ui components, lucide-react icons.

---

## File Structure

- Modify: `go-server/frontend/src/App.tsx` — stateful shell that wires data, effects, and callbacks into split components.
- Create: `go-server/frontend/src/types/index.ts` — shared TypeScript interfaces and view union type.
- Create: `go-server/frontend/src/lib/api.ts` — fetch wrappers for existing `/api/...` endpoints.
- Create: `go-server/frontend/src/lib/category.ts` — category label/color helpers.
- Create: `go-server/frontend/src/lib/format.ts` — date/time formatting helper.
- Create: `go-server/frontend/src/lib/daily-report.ts` — JSON daily report parsing helper.
- Create: `go-server/frontend/src/components/layout/Sidebar.tsx` — sidebar navigation, filters, stats, theme/collapse controls.
- Create: `go-server/frontend/src/components/layout/AppHeader.tsx` — header title, health indicator, read filter, page actions.
- Create: `go-server/frontend/src/features/items/GridView.tsx` — discovery card grid.
- Create: `go-server/frontend/src/features/items/ReaderView.tsx` — list/reader split pane.
- Create: `go-server/frontend/src/features/sources/SourcesView.tsx` — source management list.
- Create: `go-server/frontend/src/features/sources/SourceDialog.tsx` — add/edit source dialog.
- Create: `go-server/frontend/src/features/ai-settings/AISettingsView.tsx` — AI settings panel.
- Create: `go-server/frontend/src/features/logs/LogsView.tsx` — fetch logs table.
- Create: `go-server/frontend/src/features/daily-report/DailyReportView.tsx` — daily report page.
- Create: `go-server/frontend/src/features/daily-report/JsonDailyReportView.tsx` — structured JSON report renderer.

## Tasks

### Task 1: Extract shared types and pure helpers

**Files:**
- Create: `go-server/frontend/src/types/index.ts`
- Create: `go-server/frontend/src/lib/category.ts`
- Create: `go-server/frontend/src/lib/format.ts`
- Create: `go-server/frontend/src/lib/daily-report.ts`

- [ ] Move the `Source`, `ScrapedItem`, `FetchLog`, `Stats`, `AIProviderProfile`, and report-related interfaces from `App.tsx` into `types/index.ts`.
- [ ] Move `getCategoryColor` and `getCategoryLabel` into `lib/category.ts`.
- [ ] Move `formatTimeAgo` into `lib/format.ts`.
- [ ] Move daily report content cleanup and JSON extraction into `lib/daily-report.ts` as `parseDailyReportContent(content: string)`.
- [ ] Run `npm run build` from `go-server/frontend` and expect TypeScript to report no missing type/helper imports after later tasks are complete.

### Task 2: Extract API wrappers

**Files:**
- Create: `go-server/frontend/src/lib/api.ts`
- Modify: `go-server/frontend/src/App.tsx`

- [ ] Add wrappers for the existing endpoints used by `App.tsx`: health, stats, sources, logs, AI categories, daily report fetch/list/generate, items list/detail, item star/read, source toggle/run/delete/save, AI settings load/save/test/start evaluation.
- [ ] Keep the same URL paths, methods, request bodies, and response assumptions.
- [ ] Replace raw `fetch` calls in `App.tsx` with wrapper calls while keeping loading state, alerts, and local state updates in `App.tsx`.

### Task 3: Extract layout components

**Files:**
- Create: `go-server/frontend/src/components/layout/Sidebar.tsx`
- Create: `go-server/frontend/src/components/layout/AppHeader.tsx`
- Modify: `go-server/frontend/src/App.tsx`

- [ ] Move sidebar JSX into `Sidebar`, passing current view, stats, AI filters, theme/collapse state, and callbacks as props.
- [ ] Move header JSX into `AppHeader`, passing current view, browser health, read status filter, and page action callbacks as props.
- [ ] Keep navigation side effects in `App.tsx` callbacks, for example loading daily reports when entering the daily view and loading logs when entering logs.

### Task 4: Extract item views

**Files:**
- Create: `go-server/frontend/src/features/items/GridView.tsx`
- Create: `go-server/frontend/src/features/items/ReaderView.tsx`
- Modify: `go-server/frontend/src/App.tsx`

- [ ] Move grid view JSX into `GridView`.
- [ ] Move reader split-pane JSX into `ReaderView`.
- [ ] Pass item arrays, filters, selected item, HTML detail, loading flags, and callbacks as props.
- [ ] Use helpers from `lib/category.ts` and `lib/format.ts` inside these components.

### Task 5: Extract settings, logs, and daily report views

**Files:**
- Create: `go-server/frontend/src/features/sources/SourcesView.tsx`
- Create: `go-server/frontend/src/features/sources/SourceDialog.tsx`
- Create: `go-server/frontend/src/features/ai-settings/AISettingsView.tsx`
- Create: `go-server/frontend/src/features/logs/LogsView.tsx`
- Create: `go-server/frontend/src/features/daily-report/DailyReportView.tsx`
- Create: `go-server/frontend/src/features/daily-report/JsonDailyReportView.tsx`
- Modify: `go-server/frontend/src/App.tsx`

- [ ] Move sources list JSX into `SourcesView` and source modal JSX into `SourceDialog`.
- [ ] Move AI settings panel JSX into `AISettingsView`.
- [ ] Move logs JSX into `LogsView`.
- [ ] Move daily report page JSX into `DailyReportView`, and structured report renderer into `JsonDailyReportView`.
- [ ] Use `parseDailyReportContent` in `DailyReportView` before rendering `JsonDailyReportView`.

### Task 6: Verify behavior and clean up

**Files:**
- Modify: `go-server/frontend/src/App.tsx`
- Modify/Create: files from previous tasks as needed

- [ ] Run `npm run build` in `go-server/frontend`.
- [ ] Fix TypeScript import/type errors without changing behavior.
- [ ] Remove stale imports from `App.tsx` and feature files.
- [ ] Confirm the final `App.tsx` only contains app-level state, effects, callbacks, and component composition.
- [ ] Do not commit unless the user explicitly asks for a commit.

## Self-Review

- Spec coverage: The plan covers the requested `go-server/frontend/src` refactor, uses scheme A, and avoids behavior changes.
- Placeholder scan: No TODO/TBD placeholders remain.
- Type consistency: Types are centralized under `types/index.ts`; helper names are consistent across tasks.
