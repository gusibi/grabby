import { defineTheme } from "@astryxdesign/core/theme";

/**
 * Grabby's own Astryx theme.
 *
 * One deliberately opinionated art direction instead of four half-finished
 * presets: warm paper / near-black ink, a signal-green accent, Geist for UI,
 * and a serif display face for editorial content.
 */
type TokenValue = string | [light: string, dark: string];

const pair = (light: string, dark: string): [string, string] => [light, dark];

const tokens: Record<string, TokenValue> = {
  // Surfaces
  "--color-background-body": pair("#F7F7F3", "#0B0D0B"),
  "--color-background-surface": pair("#FFFFFF", "#121512"),
  "--color-background-card": pair("#FFFFFF", "#151815"),
  "--color-background-muted": pair("#F0F1EB", "#1A1E1A"),
  "--color-background-popover": pair("#FFFFFF", "#181C18"),
  "--color-background-inverted": pair("#151815", "#F2F4EE"),
  "--color-background-body-subtle": pair("#FBFBF8", "#0F110F"),

  // Ink
  "--color-text-primary": pair("#171A16", "#F1F3EC"),
  "--color-text-secondary": pair("#62685E", "#9BA197"),
  "--color-text-disabled": pair("#A3A89E", "#62685F"),
  "--color-text-accent": pair("#1F6238", "#C9F267"),
  "--color-icon-primary": pair("#171A16", "#F1F3EC"),
  "--color-icon-secondary": pair("#62685E", "#9BA197"),

  // Lines & overlays
  "--color-border": pair("#E1E3DA", "#292E28"),
  "--color-border-emphasized": pair("#C7CBC0", "#3B423A"),
  "--color-overlay-hover": pair("rgba(20, 25, 18, 0.04)", "rgba(255, 255, 255, 0.05)"),
  "--color-overlay-pressed": pair("rgba(20, 25, 18, 0.08)", "rgba(255, 255, 255, 0.09)"),
  "--color-tint-hover": pair("rgba(20, 25, 18, 0.03)", "rgba(255, 255, 255, 0.04)"),
  "--color-overlay": pair("rgba(12, 14, 11, 0.42)", "rgba(0, 0, 0, 0.62)"),

  // Accent
  "--color-accent": pair("#205F37", "#C9F267"),
  "--color-accent-muted": pair("#E4F0DF", "#222C1C"),
  "--color-on-accent": pair("#FFFFFF", "#10140B"),
  "--color-icon-accent": pair("#205F37", "#C9F267"),

  // Status
  "--color-success": pair("#247A47", "#7CD69A"),
  "--color-success-muted": pair("#E3F3E7", "#182A1D"),
  "--color-error": pair("#BB3B2F", "#FF8878"),
  "--color-error-muted": pair("#FAE6E3", "#2C1A18"),
  "--color-warning": pair("#A66E12", "#E9BE68"),
  "--color-warning-muted": pair("#F8EED8", "#2A2314"),
  "--color-on-success": pair("#FFFFFF", "#0C150E"),
  "--color-on-error": pair("#FFFFFF", "#1A0D0B"),
  "--color-on-warning": pair("#FFFFFF", "#1A1408"),
  "--color-background-blue": pair("#E7EFFB", "#152031"),
  "--color-background-purple": pair("#EFE9FA", "#211B30"),
  "--color-background-teal": pair("#DFF3F0", "#132823"),
  "--color-background-orange": pair("#FBEBDC", "#2C2015"),
  "--color-background-gray": pair("#EFEFEA", "#1D211D"),
  "--color-background-green": pair("#E3F3E7", "#182A1D"),
  "--color-background-red": pair("#FAE6E3", "#2C1A18"),

  // Type
  "--font-family-body": '"Geist Variable", -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", sans-serif',
  "--font-family-heading": '"Geist Variable", -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif',
  "--font-family-code": '"Geist Mono", ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
  "--font-size-4xs": "0.625rem",
  "--font-size-3xs": "0.6875rem",
  "--font-size-2xs": "0.75rem",
  "--font-size-xs": "0.8125rem",
  "--font-size-sm": "0.875rem",
  "--font-size-base": "0.9375rem",
  "--font-size-lg": "1.0625rem",
  "--font-size-xl": "1.25rem",
  "--font-size-2xl": "1.5rem",
  "--font-size-3xl": "1.875rem",
  "--font-size-4xl": "2.375rem",
  "--font-size-5xl": "3rem",

  // Shape
  "--radius-none": "0",
  "--radius-inner": "6px",
  "--radius-element": "9px",
  "--radius-container": "14px",
  "--radius-page": "18px",
  "--radius-full": "999px",
};

export const grabbyTheme = defineTheme({
  name: "grabby",
  tokens: tokens as Record<string, string>,
});
