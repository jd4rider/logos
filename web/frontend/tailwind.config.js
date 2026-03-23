/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  theme: {
    extend: {
      colors: {
        bg: '#0d0d1a',
        surface: '#1a1a2e',
        border: '#2a2a4a',
        text: '#e8e6d9',
        muted: '#888899',
        gold: '#d4af37',
        accent: '#7b8cde',
        highlight: '#16213e',
      },
      fontFamily: {
        serif: ['Georgia', 'Cambria', 'Times New Roman', 'serif'],
        mono: ['JetBrains Mono', 'Fira Code', 'monospace'],
      },
    },
  },
  plugins: [],
}
