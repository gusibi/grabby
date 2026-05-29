---
title: 2026-05-29-site-optimization-design
description: Balanced full-site optimization design for the Grabby Astro marketing/docs site, covering base experience, information architecture, and code structure improvements.
metadata:
  type: project
---

## Overview

The Grabby marketing/docs site at `site/` will be optimized in a balanced, phased way. The design targets three outcomes:

1. make the site immediately more shareable and performant,
2. make each page easier to scan, understand, and act on, and
3. reduce future maintenance cost by removing duplicated client-side logic.

The chosen track is **balanced full optimization**, executed across three phases.

## Current State

The site is built with Astro + Tailwind and has three pages:

- `site/src/pages/index.astro` (landing)
- `site/src/pages/setup.astro` (setup guide)
- `site/src/pages/api.astro` (API reference)

Key shared code lives in:

- `site/src/layouts/BaseLayout.astro`
- `site/src/components/SEO.astro`
- `site/src/components/Nav.astro`
- `site/src/components/Footer.astro`
- `site/src/styles/global.css`

Current weaknesses identified during exploration:

- no `og:image`/`twitter:image` despite `summary_large_image` behavior,
- external Google Fonts load adds a blocking third-party request,
- large inline script in `BaseLayout.astro` handles i18n, theme, mobile nav, and animations,
- pages reuse similar card patterns without extracting shared UI semantics,
- several uncommitted site changes already exist and must be handled carefully.

## Phase 1 — Base Experience and Discoverability

Goal: stabilize the base layer before visual and structural work.

### Scope

1. Add a 1200x630 social share image and wire it into `SEO.astro`.
2. Replace the render-blocking external Google Fonts usage with self-hosted font assets, preload hints, and `font-display: swap` to improve perceived load performance.
3. Standardize `Nav.astro` and `Footer.astro` behavior for theme toggle, language switch, and mobile menu states.
4. Add accessible landmarks and clearer semantics to shared layout regions.
5. Manage current uncommitted changes safely before deeper edits.

### Expected outcome

- Social share cards render correctly on major platforms.
- Fonts load without blocking first paint unnecessarily.
- Shared navigation behavior is consistent across pages.
- Current working changes are stabilized before the next refactor-heavy phase.

### Verification

- Lighthouse/WebPageTest-style checks for font request behavior and LCP.
- Manual checks for social preview tags.
- Manual accessibility pass for landmarks, focus states, and keyboard navigation.

## Phase 2 — Visual and Information Architecture

Goal: make each page easier to scan and more useful for its role.

### Homepage

- restrengthen the hero reading flow: title -> subtitle -> primary action,
- keep value proposition, primary action, and example block in one clear scanning band,
- make the right-side example block read like a product demo rather than a raw code dump,
- reduce feature grid fatigue by grouping or reducing visual sameness across the 6 cards,
- improve the CTA area to reinforce value and next actions.

### Setup

- convert the guide into clearer step cards with command blocks and verification checks,
- show users both “what to run” and “how to confirm it worked”.

### API reference

- increase scannability with stronger request/response contrast,
- keep the page suitable for quick reference rather than long reading,
- surface common errors and first-run examples more prominently.

### Cross-page UI semantics

- unify button, link, callout, code block, and inline code styles,
- reduce cognitive load when moving between home, setup, and API pages.

### Expected outcome

- users understand what to do faster on every page,
- setup feels like an actionable tutorial,
- API reference feels like a practical quick-reference resource.

### Verification

- manual page walkthroughs in both `zh-CN` and `en`,
- review against common developer marketing site heuristics (clarity, actionability, trust),
- check that navigation and call-to-action affordances are consistent.

## Phase 3 — Code Structure and Maintainability

Goal: make the optimization durable.

### Scope

1. Split the oversized script block in `BaseLayout.astro` into focused modules for theme, language, nav, and animation behavior.
2. Extract repeated page patterns into reusable components: step cards, command/response blocks, error blocks, callouts, and tip blocks.
3. Normalize i18n data access so adding a language or page does not duplicate logic.
4. Review current uncommitted changes and consolidate or clean them intentionally.
5. Add a minimal verification flow for local changes before merge.

### Expected outcome

- fewer repeated patterns across pages,
- clearer boundaries between layout logic and UI logic,
- easier future changes without touching many files at once.

### Verification

- build and preview pass after each structural change,
- no visible regression in locale switching, theme toggle, or navigation behavior,
- repeatable local verification steps documented in the final implementation plan.

## Non-goals

- no full redesign toward a heavy marketing website identity,
- no new backend, CMS, or deployment platform migration,
- no large accessibility audit program beyond site-critical behaviors,
- no unnecessary features added just because they are common elsewhere.

## Risks

1. the site already has uncommitted changes; a merge-order mistake could lose work,
2. font loading changes may subtly affect perceived typography if fallback handling is weak,
3. information architecture changes can unintentionally reduce content density that power users value,
4. i18n cleanup can regress either language if done without careful before/after checks.

## Mitigation

- start with change stabilization and clear verification gates,
- preserve current content meaning while improving layout and scanning,
- verify language parity before and after every page edit,
- split large changes into reviewable steps instead of one broad rewrite.

## Implementation Order

1. stabilize current site changes
2. implement Phase 1 technical improvements
3. implement Phase 2 page-by-page improvements
4. implement Phase 3 structural cleanup and verification

## Final approval summary

This spec documents the approved design direction: balanced full-site optimization for the Grabby Astro site, executed in three phases focused on base experience, information architecture, and maintainability.
