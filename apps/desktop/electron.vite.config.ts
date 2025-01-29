import ReactVite from '@vitejs/plugin-react';
import { defineConfig, externalizeDepsPlugin } from 'electron-vite';
import Tailwind from 'tailwindcss';

export default defineConfig({
  main: {
    plugins: [externalizeDepsPlugin()],
  },
  preload: {
    plugins: [externalizeDepsPlugin()],
  },
  renderer: {
    plugins: [ReactVite()],
    css: {
      postcss: {
        plugins: [Tailwind()],
      },
    },
    envPrefix: 'PUBLIC_',
  },
});
