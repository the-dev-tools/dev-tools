import { defineConfig } from '@rsbuild/core';
import { pluginReact } from '@rsbuild/plugin-react';

import packageJson from '../package.json';

export default defineConfig({
  plugins: [pluginReact()],
  source: { entry: { index: './renderer.entry.ts' } },
  server: { port: 5050 },
  html: { title: `${packageJson.name} - renderer` },
});
