import TailwindVite from '@tailwindcss/vite';
import ReactVite from '@vitejs/plugin-react';
import { defineConfig, externalizeDepsPlugin } from 'electron-vite';
import TSConfigPaths from 'vite-tsconfig-paths';

export default defineConfig({
  main: {
    plugins: [externalizeDepsPlugin({ exclude: ['electron-updater'] })],
  },
  preload: {
    plugins: [externalizeDepsPlugin()],
  },
  renderer: {
    envPrefix: 'PUBLIC_',
    plugins: [
      TSConfigPaths(),
      ReactVite({ babel: { plugins: [['babel-plugin-react-compiler', {}]] } }),
      TailwindVite(),
    ],
  },
});
