import { tanstackRouter } from '@tanstack/router-plugin/vite';
import { defineConfig } from 'vite';
import TSConfigPaths from 'vite-tsconfig-paths';

import { routes } from './src/routes';

export default defineConfig({
  plugins: [
    TSConfigPaths(),
    tanstackRouter({
      enableRouteGeneration: false,
      generatedRouteTree: './src/router-tree.tsx',
      routesDirectory: './src',
      semicolons: true,
      virtualRouteConfig: routes,
    }),
  ],
  server: {
    port: 4400,
  },
});
