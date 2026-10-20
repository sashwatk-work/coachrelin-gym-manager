/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        paper: '#f3f0e9',
        'paper-deep': '#e7e2d8',
        ink: '#11110f',
        'ink-soft': '#373731',
        muted: '#66635c',
        line: '#11110f30',
        surface: '#fbfaf6',
        raised: '#ffffff',
        signal: '#e9f21c',
        'signal-ink': '#11110f',
        error: '#9e251d',
        // Warm ink scale so existing slate-* utilities inherit Relin’s paper world.
        slate: {
          50: '#f3f0e9',
          100: '#e7e2d8',
          200: '#d4cfc3',
          300: '#b8b3a6',
          400: '#949087',
          500: '#66635c',
          600: '#4d4a44',
          700: '#373731',
          800: '#1c1c19',
          900: '#11110f',
          950: '#0d0d0c',
        },
        brand: {
          50: '#f4f6c8',
          100: '#eef46a',
          200: '#e9f21c',
          300: '#e9f21c',
          400: '#e9f21c',
          500: '#e9f21c',
          600: '#11110f',
          700: '#11110f',
          800: '#11110f',
          900: '#0d0d0c',
          950: '#0d0d0c',
        },
      },
      fontFamily: {
        sans: ['Manrope', 'Helvetica Neue', 'Arial', 'sans-serif'],
        display: ['Barlow Condensed', 'Arial Narrow', 'Impact', 'sans-serif'],
      },
      borderRadius: {
        sm: '0',
        DEFAULT: '0',
        md: '0',
        lg: '0',
        xl: '0',
        '2xl': '0',
      },
      boxShadow: {
        sm: '0 8px 20px rgba(17, 17, 15, 0.06)',
        DEFAULT: '0 12px 28px rgba(17, 17, 15, 0.08)',
        xl: '0 24px 48px rgba(17, 17, 15, 0.16)',
      },
      letterSpacing: {
        label: '0.12em',
      },
    },
  },
  plugins: [],
}
