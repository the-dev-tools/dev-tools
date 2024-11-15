import { defineConfig, loadEnv } from '@rsbuild/core';
import { pluginReact } from '@rsbuild/plugin-react';
import { TanStackRouterRspack } from '@tanstack/router-plugin/rspack';
import { Array, flow, pipe, Record, String } from 'effect';

import { routes } from './src/routes';

// Rsbuild throws warnings on unexpected environments
const NODE_ENV = process.env['NODE_ENV'];
const mode = NODE_ENV === 'production' ? 'production' : 'development';
process.env['NODE_ENV'] = mode;

export default defineConfig(({ command }) => ({
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
  dev: {
    watchFiles: {
      paths: ['src/routes.ts'],
      type: 'reload-server',
    },
  },
  tools: {
    rspack: {
      // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
      plugins: [
        command === 'dev' &&
          TanStackRouterRspack({
            routesDirectory: './src',
            generatedRouteTree: './src/router-tree.tsx',
            virtualRouteConfig: routes,
            semicolons: true,
          }),
      ],
    },
  },
}));
