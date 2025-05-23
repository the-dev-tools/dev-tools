import { scan } from 'react-scan';
//* React Scan must be instantiated first
import { TransportProvider } from '@connectrpc/connect-query';
import { DataProvider, getDefaultManagers } from '@data-client/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import {
  createBrowserHistory,
  createHashHistory,
  createRouter,
  NavigateOptions,
  RouterProvider,
  ToOptions,
} from '@tanstack/react-router';
import { Effect, Layer, Option, pipe, Predicate, Runtime, Schema } from 'effect';
import { StrictMode } from 'react';
import { RouterProvider as AriaRouterProvider } from 'react-aria-components';
import { createRoot } from 'react-dom/client';

import { makeToastQueue, ToastQueueContext } from '@the-dev-tools/ui/toast';
import { LocalMode } from '~/api/local';
import { ApiErrorHandler, ApiTransport } from '~/api/transport';

import { RouterContext } from './root';
import './styles.css';
import { routeTree } from './router-tree';

scan({ enabled: !import.meta.env.PROD, showToolbar: false });

const makeRouter = Effect.gen(function* () {
  // TODO: create an Electron-related layer instead to better represent this logic
  const history = (yield* LocalMode) ? createHashHistory() : createBrowserHistory();
  return createRouter({ context: {} as RouterContext, history, routeTree });
});

declare module '@tanstack/react-router' {
  interface Register {
    router: Effect.Effect.Success<typeof makeRouter>;
  }
}

declare module 'react-aria-components' {
  interface RouterConfig {
    href: string | ToOptions;
    routerOptions: Omit<NavigateOptions, keyof ToOptions>;
  }
}

const toastQueue = makeToastQueue();

const managers = getDefaultManagers({
  devToolsManager: {
    serialize: {
      replacer: (_key, value) => {
        if (typeof value === 'bigint') return value.toString();
        if (Predicate.isUint8Array(value)) return Schema.encodeSync(Schema.Uint8ArrayFromBase64)(value);
        return value;
      },
    },
  },
});

export const ApiErrorHandlerLive = Layer.succeed(
  ApiErrorHandler,
  (error) => void toastQueue.add({ title: error.message }),
);

export const app = Effect.gen(function* () {
  const runtime = yield* Effect.runtime<RouterContext['runtime'] extends Runtime.Runtime<infer R> ? R : never>();

  const rootEl = document.getElementById('root');

  if (!rootEl) return;

  const transport = yield* ApiTransport;
  const queryClient = new QueryClient();
  const router = yield* makeRouter;

  pipe(
    <RouterProvider context={{ queryClient, runtime, transport }} router={router} />,
    (_) => <ToastQueueContext.Provider value={Option.some(toastQueue)}>{_}</ToastQueueContext.Provider>,
    (_) => (
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
        {_}
      </AriaRouterProvider>
    ),
    (_) => (
      <DataProvider devButton={null} managers={managers}>
        {_}
      </DataProvider>
    ),
    (_) => <QueryClientProvider client={queryClient}>{_}</QueryClientProvider>,
    (_) => <TransportProvider transport={transport}>{_}</TransportProvider>,
    (_) => <StrictMode>{_}</StrictMode>,
    (_) => void createRoot(rootEl).render(_),
  );
});
