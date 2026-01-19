import { lezer } from '@lezer/generator/rollup';
import TailwindVite from '@tailwindcss/vite';
import ReactVite from '@vitejs/plugin-react';
import { defineConfig, Plugin } from 'vite';
import TSConfigPaths from 'vite-tsconfig-paths';
import { routerVitePlugin } from './src/app/router/vite';

export default defineConfig({
  envPrefix: 'PUBLIC_',
  plugins: [
    routerVitePlugin,
    TSConfigPaths({ configNames: ['tsconfig.json', 'tsconfig.lib.json'] }),
    ReactVite({ babel: { plugins: [['babel-plugin-react-compiler', {}]] } }),
    TailwindVite(),
    lezer() as Plugin,
  ],
  server: {
    port: 4400,
  },
});
