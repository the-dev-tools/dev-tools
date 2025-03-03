import ReactVite from '@vitejs/plugin-react';
import Tailwind from 'tailwindcss';
import { defineConfig } from 'vite';

import { FaviconPlugin } from '@the-dev-tools/core/favicon';

export default defineConfig({
  root: import.meta.dirname,
  cacheDir: '../../node_modules/.vite/apps/web',
  server: {
    port: 4200,
    host: 'localhost',
  },
  preview: {
    port: 4300,
    host: 'localhost',
  },
  plugins: [ReactVite(), FaviconPlugin()],
  css: {
    postcss: {
      plugins: [Tailwind()],
    },
  },
  envPrefix: 'PUBLIC_',
  build: {
    outDir: './dist',
  },
});
