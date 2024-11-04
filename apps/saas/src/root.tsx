import { Transport } from '@connectrpc/connect';
import { QueryClient } from '@tanstack/react-query';
import { createRootRouteWithContext, Outlet } from '@tanstack/react-router';
import { ComponentType, lazy, Suspense } from 'react';

import { tw } from '@the-dev-tools/ui/tailwind-literal';

const makeLazyDevtools = <Component extends ComponentType>(lazyComponent: () => Promise<Component>) =>
  process.env['NODE_ENV'] !== 'development'
    ? // Render nothing in production
      () => null
    : // Lazy load in development
      lazy(() => lazyComponent().then((_) => ({ default: _ })));

const TanStackRouterDevtools = makeLazyDevtools(() =>
  import('@tanstack/router-devtools').then((_) => _.TanStackRouterDevtools),
);

const ReactQueryDevtools = makeLazyDevtools(() =>
  import('@tanstack/react-query-devtools').then((_) => _.ReactQueryDevtools),
);

export interface RouterContext {
  transport: Transport;
  queryClient: QueryClient;
}

export const Route = createRootRouteWithContext<RouterContext>()({
  component: () => (
    <>
      <Outlet />
      <Suspense>
        <TanStackRouterDevtools position='bottom-right' toggleButtonProps={{ className: tw`!bottom-3 !right-16` }} />
        <ReactQueryDevtools buttonPosition='bottom-right' />
      </Suspense>
    </>
  ),
});
