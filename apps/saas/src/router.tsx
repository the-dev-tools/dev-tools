import { createRootRoute, createRouter } from '@tanstack/react-router';

const rootRoute = createRootRoute();

const routeTree = rootRoute.addChildren([]);

export const router = createRouter({ routeTree });

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router;
  }
}
