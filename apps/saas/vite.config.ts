import { TanStackRouterVite } from '@tanstack/router-plugin/vite';
import ReactVite from '@vitejs/plugin-react';
import Tailwind from 'tailwindcss';
import { defineConfig } from 'vite';

import { routes } from './src/routes';

export default defineConfig({
  root: import.meta.dirname,
  cacheDir: '../../node_modules/.vite/apps/saas',
  server: {
    port: 4200,
    host: 'localhost',
  },
  preview: {
    port: 4300,
    host: 'localhost',
  },
  plugins: [
    ReactVite(),
    TanStackRouterVite({
      routesDirectory: './src',
      generatedRouteTree: './src/router-tree.tsx',
      virtualRouteConfig: routes,
      semicolons: true,
    }),
  ],
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
