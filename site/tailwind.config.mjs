/** @type {import('tailwindcss').Config} */
export default {
  content: ['./src/**/*.{astro,html,js,jsx,md,mdx,svelte,ts,tsx,vue}'],
  darkMode: 'class',
  theme: {
    extend: {
      fontFamily: {
        sans: ['Space Grotesk', 'system-ui', 'sans-serif'],
        mono: ['JetBrains Mono', 'ui-monospace', 'monospace'],
      },
      colors: {
        accent: '#6366F1',
        'accent-hover': '#4F46E5',
        'accent-light': '#818CF8',
        surf: '#F8FAFC',
        'surf-elevated': '#F1F5F9',
        bord: '#E2E8F0',
        'bord-light': '#E2E8F0',
        mute: '#64748B',
        'mute-dim': '#94A3B8',
      },
    },
  },
  plugins: [],
};
