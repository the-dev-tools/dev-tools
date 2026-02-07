import { scan } from 'react-scan';

import { TransportProvider } from '@connectrpc/connect-query';
import { Atom, Result, useAtomValue } from '@effect-atom/atom-react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { RouterProvider } from '@tanstack/react-router';
import { ConfigProvider, Effect, pipe, Record, Runtime } from 'effect';
import { StrictMode } from 'react';
import { UiProvider } from '@the-dev-tools/ui/provider';
import { makeToastQueue } from '@the-dev-tools/ui/toast';
import { ApiCollections, ApiTransport } from '~/shared/api';
import { runtimeAtom } from '~/shared/lib/runtime';
import { RouterContext } from './context';
import { startOpenReplay } from './open-replay';
import { router } from './router';
import { initUmami } from './umami';

scan({ enabled: !import.meta.env.PROD, showToolbar: false });

const appAtom = runtimeAtom.atom(
  Effect.gen(function* () {
    yield* initUmami;
    yield* startOpenReplay;
    yield* ApiCollections;

    const runtime = yield* Effect.runtime<RouterContext['runtime'] extends Runtime.Runtime<infer R> ? R : never>();
    const transport = yield* ApiTransport;
    const queryClient = new QueryClient();
    const toastQueue = makeToastQueue();

    return { queryClient, runtime, toastQueue, transport };
  }),
);

export const App = () => {
  const context = useAtomValue(appAtom);

  return Result.match(context, {
    onFailure: () => <div>App startup error</div>,
    onInitial: () => <div>Loading...</div>,
    onSuccess: ({ value }) => {
      let _ = <RouterProvider context={value} router={router} />;
      _ = <UiProvider toastQueue={value.toastQueue}>{_}</UiProvider>;
      _ = <QueryClientProvider client={value.queryClient}>{_}</QueryClientProvider>;
      _ = <TransportProvider transport={value.transport}>{_}</TransportProvider>;
      _ = <StrictMode>{_}</StrictMode>;
      return _;
    },
  });
};

export const configProviderFromMetaEnv = (extra?: Record<string, string>) =>
  pipe(
    { ...import.meta.env, ...extra },
    Record.mapKeys((_) => _.replaceAll('__', '.')),
    Record.toEntries,
    (_) => new Map(_ as [string, string][]),
    ConfigProvider.fromMap,
  );

export const addGlobalLayer: Atom.RuntimeFactory['addGlobalLayer'] = Atom.runtime.addGlobalLayer;

export { runtimeAtom };
