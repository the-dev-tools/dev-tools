import { TanStackRouterVite } from '@tanstack/router-plugin/vite';
import ReactVite from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

import { routes } from './src/routes';

export default defineConfig({
  plugins: [
    ReactVite(),
    TanStackRouterVite({
      routesDirectory: './src',
      generatedRouteTree: './src/router-tree.tsx',
      virtualRouteConfig: routes,
      semicolons: true,
    }),
  ],
  server: {
    port: 4400,
  },
});
