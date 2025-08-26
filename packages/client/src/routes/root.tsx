import { createRootRouteWithContext, Outlet } from '@tanstack/react-router';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { ToastRegion } from '@the-dev-tools/ui/toast';
import { DevToolsProvider, ReactQueryDevTools, ReactScanDevTools, TanStackRouterDevTools } from '~dev-tools';
import { RouterContext } from './context';
import { ErrorComponent } from './error';

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
