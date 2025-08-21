import { Transport } from '@connectrpc/connect';
import { Registry } from '@effect-atom/atom-react';
import { KeyValueStore } from '@effect/platform/KeyValueStore';
import { QueryClient } from '@tanstack/react-query';
import { createRootRouteWithContext, Outlet } from '@tanstack/react-router';
import { Runtime } from 'effect';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { ToastRegion } from '@the-dev-tools/ui/toast';
import { AuthTransport, MagicClient } from '~/api/auth';
import { DataClient } from '~data-client';
import { DevToolsProvider, ReactQueryDevTools, ReactScanDevTools, TanStackRouterDevTools } from '~dev-tools';
import { ErrorComponent } from './error';

export interface RouterContext {
  dataClient: DataClient;
  queryClient: QueryClient;
  runtime: Runtime.Runtime<AuthTransport | KeyValueStore | MagicClient | Registry.AtomRegistry>;
  transport: Transport;
}

export const Route = createRootRouteWithContext<RouterContext>()({
  component: () => (
    <>
      <ToastRegion />
      <Outlet />

      <DevToolsProvider>
        <TanStackRouterDevTools position='bottom-right' toggleButtonProps={{ className: tw`!right-16 !bottom-3` }} />
        <ReactQueryDevTools buttonPosition='bottom-right' />
        <ReactScanDevTools />
      </DevToolsProvider>
    </>
  ),
  errorComponent: ErrorComponent,
});
