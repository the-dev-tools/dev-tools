import { TransportProvider } from '@connectrpc/connect-query';
import { DataProvider, getDefaultManagers, useController } from '@data-client/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createBrowserHistory, createHashHistory, createRouter, RouterProvider } from '@tanstack/react-router';
import { Effect, Layer, Option, pipe, Predicate, Runtime, Schema } from 'effect';
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { scan } from 'react-scan';
import { AriaRouterProvider } from '@the-dev-tools/ui/router';
import { makeToastQueue, ToastQueueContext } from '@the-dev-tools/ui/toast';
import { LocalMode } from '~/api/local';
import { ApiErrorHandler, ApiTransport } from '~/api/transport';
import { makeDataClient } from '~data-client';
import { RouterContext } from './root';
import { routeTree } from './router-tree';

import './styles.css';

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

interface RootProps extends Omit<RouterContext, 'dataClient'> {
  router: Effect.Effect.Success<typeof makeRouter>;
}

const Root = ({ router, transport, ...context }: RootProps) => {
  const controller = useController();
  const dataClient = makeDataClient({ controller, transport });
  return <RouterProvider context={{ ...context, dataClient, transport }} router={router} />;
};

export const app = Effect.gen(function* () {
  const runtime = yield* Effect.runtime<RouterContext['runtime'] extends Runtime.Runtime<infer R> ? R : never>();

  const rootEl = document.getElementById('root');

  if (!rootEl) return;

  const transport = yield* ApiTransport;
  const queryClient = new QueryClient();
  const router = yield* makeRouter;

  pipe(
    <Root {...{ queryClient, router, runtime, transport }} />,
    (_) => <ToastQueueContext.Provider value={Option.some(toastQueue)}>{_}</ToastQueueContext.Provider>,
    (_) => <AriaRouterProvider>{_}</AriaRouterProvider>,
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
