import ReactVite from '@vitejs/plugin-react';
import { defineConfig, externalizeDepsPlugin } from 'electron-vite';
import Tailwind from 'tailwindcss';

// console.log(process.env.ELECTRON_OVERRIDE_DIST_PATH);

export default defineConfig({
  main: {
    plugins: [externalizeDepsPlugin()],
    // envPrefix: 'PUBLIC_',
    build: {
      outDir: 'dist/main',
    },
  },
  preload: {
    plugins: [externalizeDepsPlugin()],
    build: {
      outDir: 'dist/preload',
    },
  },
  renderer: {
    plugins: [ReactVite()],
    css: {
      postcss: {
        plugins: [Tailwind()],
      },
    },
    envPrefix: 'PUBLIC_',
    build: {
      outDir: 'dist/renderer',
    },
  },
});
