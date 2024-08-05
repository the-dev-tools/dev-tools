import { defineConfig, loadEnv } from '@rsbuild/core';
import { pluginReact } from '@rsbuild/plugin-react';
import { Array, flow, pipe, Record, String } from 'effect';

export default defineConfig({
  plugins: [pluginReact()],
  html: {
    title: 'The Dev Tools',
    favicon: './src/icon.png',
  },
  source: {
    define: {
      PUBLIC_ENV: pipe(
        loadEnv().publicVars,
        Record.mapKeys(flow(String.split('.'), Array.lastNonEmpty)),
        Record.map((_): unknown => JSON.parse(_)),
        JSON.stringify,
      ),
    },
  },
});
