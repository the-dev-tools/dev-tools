import { isMessage } from '@bufbuild/protobuf';
import { TransportProvider } from '@connectrpc/connect-query';
import { QueryNormalizerProvider } from '@normy/react-query';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createRouter, NavigateOptions, RouterProvider, ToOptions } from '@tanstack/react-router';
import { Array, Effect, Option, pipe, Runtime } from 'effect';
import { Ulid } from 'id128';
import { StrictMode } from 'react';
import { RouterProvider as AriaRouterProvider } from 'react-aria-components';
import { createRoot } from 'react-dom/client';

import { getMessageId, getMessageIdKey } from '@the-dev-tools/api/meta';
import { ApiTransport } from '@the-dev-tools/api/transport';

import { RouterContext } from './root';
import { routeTree } from './router-tree';

import '@xyflow/react/dist/style.css';
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
  const runtime = yield* Effect.runtime<RouterContext['runtime'] extends Runtime.Runtime<infer R> ? R : never>();

  const rootEl = document.getElementById('root');

  if (!rootEl) return;

  const transport = yield* ApiTransport;
  const queryClient = new QueryClient();

  const root = createRoot(rootEl);
  root.render(
    <StrictMode>
      <TransportProvider transport={transport}>
        <QueryNormalizerProvider
          queryClient={queryClient}
          normalizerConfig={{
            getNormalizationObjectKey: (data) => {
              console.log({ data });
              if (!isMessage(data)) return undefined;
              const key = getMessageIdKey(data);
              const idCan = pipe(
                getMessageId(data),
                Option.map((_) => Ulid.construct(_).toCanonical()),
              );
              const a = pipe(Option.product(key, idCan), Option.map(Array.join(' ')), Option.getOrUndefined);
              console.log(a);
            },
          }}
        >
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
