import { defineConfig, loadEnv } from '@rsbuild/core';
import { pluginReact } from '@rsbuild/plugin-react';
import { TanStackRouterRspack } from '@tanstack/router-plugin/rspack';
import { Array, flow, pipe, Record, String } from 'effect';

// Rsbuild throws warnings on unexpected environments
const NODE_ENV = process.env['NODE_ENV'];
const mode = NODE_ENV === 'production' ? 'production' : 'development';
process.env['NODE_ENV'] = mode;

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
        Record.set('NODE_ENV', NODE_ENV),
        JSON.stringify,
      ),
    },
  },
  tools: {
    rspack: {
      plugins: [
        TanStackRouterRspack({
          generatedRouteTree: './src/routes/-generated-router-tree.tsx',
          semicolons: true,
        }),
      ],
    },
  },
});
