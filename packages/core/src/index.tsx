import { TransportProvider } from '@connectrpc/connect-query';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createRouter, NavigateOptions, RouterProvider, ToOptions } from '@tanstack/react-router';
import { Effect } from 'effect';
import { StrictMode } from 'react';
import { RouterProvider as AriaRouterProvider } from 'react-aria-components';
import { createRoot } from 'react-dom/client';

import { ApiTransport } from '@the-dev-tools/api/transport';

import { RouterContext } from './root';
import { routeTree } from './router-tree';

import '@the-dev-tools/ui/fonts';
import './styles.css';

const router = createRouter({ routeTree, context: {} as RouterContext });

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router;
  }
}

declare module 'react-aria-components' {
  interface RouterConfig {
    href: ToOptions | string;
    routerOptions: Omit<NavigateOptions, keyof ToOptions>;
  }
}

export const app = Effect.gen(function* () {
  const rootEl = document.getElementById('root');

  if (!rootEl) return;

  const transport = yield* ApiTransport;
  const queryClient = new QueryClient();

  const root = createRoot(rootEl);
  root.render(
    <StrictMode>
      <TransportProvider transport={transport}>
        <QueryClientProvider client={queryClient}>
          <AriaRouterProvider
            navigate={(to, options) => {
              if (typeof to === 'string') return;
              return router.navigate({ ...to, ...options });
            }}
            useHref={(to) => {
              if (typeof to === 'string') return to;
              return router.buildLocation(to).href;
            }}
          >
            <RouterProvider router={router} context={{ transport, queryClient }} />
          </AriaRouterProvider>
        </QueryClientProvider>
      </TransportProvider>
    </StrictMode>,
  );
});
