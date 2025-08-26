import { tanstackRouter } from '@tanstack/router-plugin/vite';
import { defineConfig } from 'vite';
import TSConfigPaths from 'vite-tsconfig-paths';
import { routes } from './src/routes/__virtual';

export default defineConfig({
  plugins: [
    TSConfigPaths(),
    tanstackRouter({
      autoCodeSplitting: true,
      generatedRouteTree: './src/routes/__tree.tsx',
      routesDirectory: './src/routes',
      target: 'react',
      virtualRouteConfig: routes,
    }),
  ],
  server: {
    port: 4400,
  },
});
