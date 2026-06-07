---
version: "2.0"
name: "Grabby Design System"
description: "A native macOS-inspired design system for data aggregation and RSS reading."
colors:
  primary: "#007aff"
  primary-dark: "#0a84ff"
  background: "#f5f5f7"
  background-dark: "#1a1a1a"
  surface: "#ffffff"
  surface-dark: "#1c1c1e"
  sidebar: "rgba(235, 235, 235, 0.7)"
  sidebar-dark: "rgba(35, 35, 35, 0.7)"
  border: "rgba(0, 0, 0, 0.1)"
  border-dark: "rgba(255, 255, 255, 0.1)"
  text: "#172033"
  text-dark: "#f5f5f7"
  text-muted: "#71717a"
  text-muted-dark: "#a1a1aa"
  success: "#28c840"
  warning: "#febc2e"
  danger: "#ff5f57"
typography:
  display:
    fontFamily: "'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif"
    fontSize: 36px
    fontWeight: 900
    lineHeight: 1.2
    letterSpacing: -0.02em
  title:
    fontFamily: "'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif"
    fontSize: 24px
    fontWeight: 700
    lineHeight: 1.2
    letterSpacing: -0.01em
  section-title:
    fontFamily: "'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif"
    fontSize: 18px
    fontWeight: 600
    lineHeight: 1.3
  subtitle:
    fontFamily: "'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif"
    fontSize: 14px
    fontWeight: 500
    lineHeight: 1.5
  body:
    fontFamily: "'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif"
    fontSize: 14px
    fontWeight: 400
    lineHeight: 1.6
  body-small:
    fontFamily: "'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif"
    fontSize: 12px
    fontWeight: 400
    lineHeight: 1.5
  caption:
    fontFamily: "'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif"
    fontSize: 10px
    fontWeight: 700
    lineHeight: 1.3
    letterSpacing: 0.05em
spacing:
  none: 0px
  xs: 4px
  sm: 8px
  md: 16px
  lg: 24px
  xl: 32px
  2xl: 48px
  3xl: 64px
rounded:
  none: 0px
  sm: 6px
  md: 12px
  lg: 16px
  xl: 24px
  2xl: 40px
  pill: 999px
components:
  mac-window:
    backgroundColor: "{colors.background}"
    border: "1px solid {colors.border}"
    rounded: "{rounded.xl}"
    shadow: "0 30px 60px -12px rgba(0, 0, 0, 0.25)"
  sidebar-vibrancy:
    backgroundColor: "{colors.sidebar}"
    backdropFilter: "blur(40px) saturate(200%)"
    borderRight: "1px solid {colors.border}"
  news-card:
    backgroundColor: "{colors.surface}"
    border: "1px solid rgba(0,0,0,0.04)"
    rounded: "{rounded.lg}"
    padding: 20px
  primary-action:
    backgroundColor: "{colors.primary}"
    textColor: "#ffffff"
    rounded: "{rounded.md}"
    padding: "8px 16px"
---

## Overview

Grabby V2 is a macOS-native inspired design system built for data aggregation, RSS reading, and content discovery. Its visual language merges Apple's human interface guidelines (vibrancy, traffic lights, depth) with shadcn/ui's engineering precision and stark minimalism.

The system is designed to handle high-density information feeds while maintaining a clean, breathable, and distraction-free environment. It dynamically adapts to system Light and Dark modes.

## Colors

The palette uses structural system colors that map directly to macOS environments, utilizing pure Apple tones and highly specific opacity values for borders and sidebars.

- Use `background` for the main application window canvas.
- Use `surface` for distinct content containers (cards, modals, detail panes).
- Use `sidebar` for the navigation pane (must be combined with background blur).
- Use `primary` (Apple Blue) for active states, primary buttons, and key interactive accents.
- Use `text` for high-contrast reading and `text-muted` for secondary metadata, timestamps, and captions.
- Use semantic traffic-light colors (`danger` red, `warning` yellow, `success` green) strictly for window controls or critical system status indicators.

Do not introduce ad hoc brand colors. The application should feel like an extension of the operating system.

## Typography

Grabby uses the `Inter` font family, gracefully falling back to `-apple-system` (SF Pro) to ensure a crisp, modern, and native reading experience.

- Use `display` for large empty states or massive data metrics.
- Use `title` for article headers in the detail view.
- Use `section-title` for pane headers or settings block titles.
- Use `subtitle` for feed titles in lists or grids.
- Use `body` for long-form article text and extracted content.
- Use `body-small` for feed summaries and descriptions.
- Use `caption` (uppercase, tracked out) for category tags, source labels, and tiny metadata.

Article reading typography should always prioritize legibility, utilizing generous line heights (`1.6`) and constrained line widths.

## Layout

Layouts are rigid, multi-pane structural environments. Choose one of the canonical views; do not mix paradigms loosely.

Core Application Views:
- `Three-Pane (List View)`: A classic Mac Mail/RSS paradigm. 
  1. Sidebar (Navigation & Sources)
  2. List Pane (Feed items)
  3. Detail Pane (Immersive reading)
- `Discovery (Grid View)`: A visual-first card grid for aggregate exploration.
- `Settings View`: A focused, centered container for source management and app configuration.

Every layout should respect the absolute structural constraints:
- Window controls (Traffic Lights) must remain anchored at the top-left of the sidebar.
- The sidebar must feature a vibrant, translucent background.
- Content panes must be scrollable independently without scrolling the global window.

## Elevation & Depth

Grabby uses depth to establish hierarchy and focus, never just for decoration.

- The root `mac-window` requires a heavy, diffuse drop shadow to float above the desktop.
- Floating panels, modals, and settings cards should cast soft, large-radius shadows.
- Flat lists and sidebar items do not use shadows; they rely on hover background changes or left-border markers for active states.
- News cards in the grid view feature an interactive lift (`translateY(-4px)`) and a slight shadow increase on hover to indicate clickability.

## Shapes

Corners follow Apple's continuous squircle logic:
- `2xl (40px)` or `xl (24px)` for outer main window containers and large modals.
- `lg (16px)` for distinct UI cards, news containers, and large image thumbnails.
- `md (12px)` for standard buttons, sidebar items, and smaller content blocks.
- `pill (999px)` for category tags, small badges, and search bars.

Borders are universally thin (`1px`) and highly transparent (`rgba(0,0,0,0.1)` in light mode, `rgba(255,255,255,0.1)` in dark mode) to create subtle, barely-there separation.

## Components

Compose the application from these standard building blocks:

- **Sidebar Item**: Contains a Lucide icon, label, and optional numeric badge. Active states are highlighted with the primary blue and a distinct background.
- **Traffic Lights**: Fixed at 12px dimensions with standard macOS red, yellow, and green.
- **News Card**: An image-led block featuring an absolute-positioned category pill, source icon/text, time, bold title, clamped summary, and a bottom score/action bar.
- **List Feed Item**: A text-heavy row with a subtle left-border highlight (`border-l-blue-500`) when active.
- **Immersive Reader**: The right-hand detail view featuring a large hero image, prominent title, metadata row, and a clean, centered `prose` container for the article body.

## Do's and Don'ts

**Do:**
- Rely on structural padding and margins rather than visible dividers whenever possible.
- Use `backdrop-filter` to create genuine glassmorphism on sidebars and sticky headers.
- Provide smooth, low-duration transitions (`transition-all duration-200` or `300`) for hovers, clicks, and view swaps.
- Clamp long text (using `line-clamp-2` or `3`) in list and grid views to preserve strict alignment.
- Ensure the app scales cleanly across standard desktop resolutions, collapsing the sidebar gracefully if necessary.

**Don't:**
- Do not use harsh black (`#000000`) or white (`#FFFFFF`) for borders; always use transparent rgba values.
- Do not introduce non-system icon sets. Stick exclusively to clean, stroke-based Lucide icons.
- Do not clutter the reading pane with floating actions; keep utility buttons anchored to the header or bottom of the article.
- Do not use thick borders or heavy border radii on small, nested elements.
- Do not make the sidebar completely opaque.
