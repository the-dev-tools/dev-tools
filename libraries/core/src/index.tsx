import { TransportProvider } from '@connectrpc/connect-query';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import {
  createBrowserHistory,
  createHashHistory,
  createRouter,
  NavigateOptions,
  RouterProvider,
  ToOptions,
} from '@tanstack/react-router';
import { Effect, Runtime } from 'effect';
import { StrictMode } from 'react';
import { RouterProvider as AriaRouterProvider } from 'react-aria-components';
import { createRoot } from 'react-dom/client';

import { LocalMode } from '@the-dev-tools/api/local';
import { QueryNormalizerProvider } from '@the-dev-tools/api/normalizer';
import { ApiTransport } from '@the-dev-tools/api/transport';

import { RouterContext } from './root';
import { routeTree } from './router-tree';

import './styles.css';

const makeRouter = Effect.gen(function* () {
  // TODO: create an Electron-related layer instead to better represent this logic
  const history = (yield* LocalMode) ? createHashHistory() : createBrowserHistory();
  return createRouter({ routeTree, context: {} as RouterContext, history });
});

declare module '@tanstack/react-router' {
  interface Register {
    router: Effect.Effect.Success<typeof makeRouter>;
  }
}

declare module 'react-aria-components' {
  interface RouterConfig {
    href: ToOptions | string;
    routerOptions: Omit<NavigateOptions, keyof ToOptions>;
  }
}

export const app = Effect.gen(function* () {
  const runtime = yield* Effect.runtime<RouterContext['runtime'] extends Runtime.Runtime<infer R> ? R : never>();

  const rootEl = document.getElementById('root');

  if (!rootEl) return;

  const transport = yield* ApiTransport;
  const queryClient = new QueryClient();
  const router = yield* makeRouter;

  const root = createRoot(rootEl);
  root.render(
    <StrictMode>
      <TransportProvider transport={transport}>
        <QueryNormalizerProvider queryClient={queryClient}>
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
              <RouterProvider router={router} context={{ queryClient, runtime, transport }} />
            </AriaRouterProvider>
          </QueryClientProvider>
        </QueryNormalizerProvider>
      </TransportProvider>
    </StrictMode>,
  );
});
