/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{vue,js,ts,jsx,tsx}'],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        studio: {
          bg: 'var(--st-bg)',
          surface: 'var(--st-surface)',
          card: 'var(--st-card)',
          elevated: 'var(--st-elevated)',
          border: 'var(--st-border)',
          muted: 'var(--st-muted)',
          accent: 'var(--st-accent)',
          'accent-dim': 'var(--st-accent-dim)',
          highlight: 'var(--st-highlight)',
          fg: 'var(--st-fg)',
          'fg-muted': 'var(--st-fg-muted)',
          'fg-subtle': 'var(--st-fg-subtle)',
          input: 'var(--st-input)',
        },
      },
      fontFamily: {
        sans: ['"DM Sans"', 'system-ui', 'sans-serif'],
      },
    },
  },
  plugins: [],
}
