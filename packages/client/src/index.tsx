import { createClient } from '@connectrpc/connect';
import { TransportProvider } from '@connectrpc/connect-query';
import { DataProvider, getDefaultManagers } from '@data-client/react';
import { Registry, RegistryContext } from '@effect-atom/atom-react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createHashHistory, createRouter, RouterProvider } from '@tanstack/react-router';
import { Effect, Option, pipe, Predicate, Runtime, Schedule, Schema } from 'effect';
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { scan } from 'react-scan';
import { HealthService } from '@the-dev-tools/spec/health/v1/health_pb';
import { AriaRouterProvider } from '@the-dev-tools/ui/router';
import { makeToastQueue, ToastQueueContext } from '@the-dev-tools/ui/toast';
import { ApiTransport } from '~/api/transport';
import { useMakeDataClient } from '~data-client';
import { RouterContext } from '~routes/context';
import { routeTree } from './routes/__tree';

import './styles.css';

scan({ enabled: !import.meta.env.PROD, showToolbar: false });

const router = createRouter({
  context: {} as RouterContext,
  history: createHashHistory(),
  routeTree,
});

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router;
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

interface RootProps extends Omit<RouterContext, 'dataClient'> {}

const Root = (context: RootProps) => {
  const dataClient = useMakeDataClient();
  return <RouterProvider context={{ ...context, dataClient }} router={router} />;
};

export const app = Effect.gen(function* () {
  const runtime = yield* Effect.runtime<RouterContext['runtime'] extends Runtime.Runtime<infer R> ? R : never>();

  const rootEl = document.getElementById('root');

  if (!rootEl) return;

  const transport = yield* ApiTransport;
  const queryClient = new QueryClient();
  const atomRegistry = yield* Registry.AtomRegistry;

  let _ = <Root {...{ queryClient, runtime, transport }} />;
  _ = <RegistryContext value={atomRegistry}>{_}</RegistryContext>;
  _ = <ToastQueueContext.Provider value={Option.some(toastQueue)}>{_}</ToastQueueContext.Provider>;
  _ = <AriaRouterProvider>{_}</AriaRouterProvider>;
  _ = (
    <DataProvider devButton={null} managers={managers}>
      {_}
    </DataProvider>
  );
  _ = <QueryClientProvider client={queryClient}>{_}</QueryClientProvider>;
  _ = <TransportProvider transport={transport}>{_}</TransportProvider>;
  _ = <StrictMode>{_}</StrictMode>;

  // Wait for the server to start up before first render
  const { healthCheck } = createClient(HealthService, transport);
  yield* pipe(
    Effect.tryPromise((signal) => healthCheck({}, { signal, timeoutMs: 0 })),
    Effect.retry({ schedule: Schedule.exponential('10 millis'), times: 100 }),
  );

  createRoot(rootEl).render(_);
});
