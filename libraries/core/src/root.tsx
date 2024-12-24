import { Transport } from '@connectrpc/connect';
import { KeyValueStore } from '@effect/platform/KeyValueStore';
import { QueryClient } from '@tanstack/react-query';
import { createRootRouteWithContext, Outlet } from '@tanstack/react-router';
import { Runtime } from 'effect';

import { AuthTransport, MagicClient } from '@the-dev-tools/api/auth';
import { tw } from '@the-dev-tools/ui/tailwind-literal';

import { DevToolsProvider, ReactQueryDevTools, TanStackRouterDevTools } from './dev-tools';

export interface RouterContext {
  queryClient: QueryClient;
  runtime: Runtime.Runtime<KeyValueStore | MagicClient | AuthTransport>;
  transport: Transport;
}

export const Route = createRootRouteWithContext<RouterContext>()({
  component: () => (
    <DevToolsProvider>
      <Outlet />
      <TanStackRouterDevTools position='bottom-right' toggleButtonProps={{ className: tw`!bottom-3 !right-16` }} />
      <ReactQueryDevTools buttonPosition='bottom-right' />
    </DevToolsProvider>
  ),
});
