import { TanStackRouterVite } from '@tanstack/router-plugin/vite';
import { defineConfig } from 'vite';
import TSConfigPaths from 'vite-tsconfig-paths';

import { routes } from './src/routes';

export default defineConfig({
  plugins: [
    TSConfigPaths(),
    TanStackRouterVite({
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
