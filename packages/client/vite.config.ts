import { tanstackRouter } from '@tanstack/router-plugin/vite';
import { defineConfig } from 'vite';
import TSConfigPaths from 'vite-tsconfig-paths';
import { routes } from './src/routes';

export default defineConfig({
  plugins: [
    TSConfigPaths(),
    tanstackRouter({
      generatedRouteTree: './src/router-tree.tsx',
      routesDirectory: './src',
      semicolons: true,
      target: 'react',
      virtualRouteConfig: routes,
    }),
  ],
  server: {
    port: 4400,
  },
});
