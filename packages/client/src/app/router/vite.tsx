import { tanstackRouter } from '@tanstack/router-plugin/vite';

export const routerVitePlugin = tanstackRouter({
  autoCodeSplitting: true,
  generatedRouteTree: 'src/app/router/route-tree.gen.ts',
  routesDirectory: 'src/app/router/routes',
  target: 'react',
});
