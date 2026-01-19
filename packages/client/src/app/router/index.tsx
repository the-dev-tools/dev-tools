import { createHashHistory, createRouter } from '@tanstack/react-router';
import { RouterContext } from '../context';
import { routeTree } from './route-tree.gen';

export const router = createRouter({
  context: {} as RouterContext,
  history: createHashHistory(),
  routeTree,
});

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router;
  }
}
