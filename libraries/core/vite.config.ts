import { TanStackRouterVite } from '@tanstack/router-plugin/vite';
import { defineConfig } from 'vite';

import { routes } from './src/routes';

export default defineConfig({
  plugins: [
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
