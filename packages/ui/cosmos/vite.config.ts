import ReactVite from '@vitejs/plugin-react';
import Tailwind from 'tailwindcss';
import { defineConfig } from 'vite';

export default defineConfig({
  plugins: [ReactVite()],
  css: {
    postcss: {
      plugins: [Tailwind()],
    },
  },
});
