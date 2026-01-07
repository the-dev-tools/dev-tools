import { lezer } from '@lezer/generator/rollup';
import TailwindVite from '@tailwindcss/vite';
import { tanstackRouter } from '@tanstack/router-plugin/vite';
import ReactVite from '@vitejs/plugin-react';
import { defineConfig, Plugin } from 'vite';
import TSConfigPaths from 'vite-tsconfig-paths';
import { routes } from './src/routes/__virtual';

export default defineConfig({
  envPrefix: 'PUBLIC_',
  plugins: [
    tanstackRouter({
      autoCodeSplitting: true,
      generatedRouteTree: './src/routes/__tree.tsx',
      routesDirectory: './src/routes',
      target: 'react',
      virtualRouteConfig: routes,
    }),
    TSConfigPaths({ configNames: ['tsconfig.json', 'tsconfig.lib.json'] }),
    ReactVite({ babel: { plugins: [['babel-plugin-react-compiler', {}]] } }),
    TailwindVite(),
    lezer() as Plugin,
  ],
  server: {
    port: 4400,
  },
});
