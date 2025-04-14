import { Transport } from '@connectrpc/connect';
import { KeyValueStore } from '@effect/platform/KeyValueStore';
import { QueryClient } from '@tanstack/react-query';
import { createRootRouteWithContext, Outlet } from '@tanstack/react-router';
import { Runtime } from 'effect';

import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { ToastRegion } from '@the-dev-tools/ui/toast';
import { AuthTransport, MagicClient } from '~/api/auth';
import { DevToolsProvider, ReactQueryDevTools, ReactScanDevTools, TanStackRouterDevTools } from '~dev-tools';

import { ErrorComponent } from './error';

export interface RouterContext {
  queryClient: QueryClient;
  runtime: Runtime.Runtime<AuthTransport | KeyValueStore | MagicClient>;
  transport: Transport;
}

export const Route = createRootRouteWithContext<RouterContext>()({
  component: () => (
    <>
      <DevToolsProvider>
        <ToastRegion />
        <TanStackRouterDevTools position='bottom-right' toggleButtonProps={{ class: tw`!bottom-3 !right-16` }} />
        <ReactQueryDevTools buttonPosition='bottom-right' />
        <ReactScanDevTools />
      </DevToolsProvider>
      <Outlet />
    </>
  ),
  errorComponent: ErrorComponent,
});
