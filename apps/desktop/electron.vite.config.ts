import { lezer } from '@lezer/generator/rollup';
import TailwindVite from '@tailwindcss/vite';
import ReactVite from '@vitejs/plugin-react';
import { defineConfig } from 'electron-vite';
import { Plugin } from 'vite';
import TSConfigPaths from 'vite-tsconfig-paths';

export default defineConfig({
  main: {
    build: { externalizeDeps: { exclude: ['electron-updater'] } },
  },
  preload: {
    build: { rollupOptions: { output: { format: 'cjs' } } },
  },
  renderer: {
    envPrefix: 'PUBLIC_',
    plugins: [
      TSConfigPaths({ configNames: ['tsconfig.json', 'tsconfig.lib.json'] }),
      ReactVite({ babel: { plugins: [['babel-plugin-react-compiler', {}]] } }),
      TailwindVite(),
      lezer() as Plugin,
    ],
  },
});
