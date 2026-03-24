/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  theme: {
    extend: {
      colors: {
        bg: '#06111c',
        surface: '#102131',
        border: '#254259',
        text: '#f7f1e3',
        muted: '#9eb0bf',
        gold: '#f5bf52',
        accent: '#8dd6ff',
        highlight: '#183347',
      },
      fontFamily: {
        display: ['Baskerville', 'Palatino Linotype', 'Book Antiqua', 'serif'],
        sans: ['Avenir Next', 'Segoe UI', 'sans-serif'],
        serif: ['Iowan Old Style', 'Baskerville', 'Palatino Linotype', 'serif'],
      },
      boxShadow: {
        panel: '0 22px 70px rgba(0, 0, 0, 0.32)',
      },
    },
  },
  plugins: [],
}
