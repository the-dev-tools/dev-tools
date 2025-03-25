import TailwindVite from '@tailwindcss/vite';
import ReactVite from '@vitejs/plugin-react';
import { defineConfig, externalizeDepsPlugin } from 'electron-vite';

export default defineConfig({
  main: {
    plugins: [externalizeDepsPlugin({ exclude: ['@effect/platform-node', 'electron-updater'] })],
  },
  preload: {
    plugins: [externalizeDepsPlugin()],
  },
  renderer: {
    envPrefix: 'PUBLIC_',
    plugins: [ReactVite(), TailwindVite()],
  },
});
