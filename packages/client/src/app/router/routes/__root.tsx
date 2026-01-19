import { createRootRouteWithContext, Outlet } from '@tanstack/react-router';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { ToastRegion } from '@the-dev-tools/ui/toast';
import { RouterContext } from '../../context';
import { DevToolsProvider, ReactQueryDevTools, ReactScanDevTools, TanStackRouterDevTools } from '../../dev-tools';
import { ErrorComponent } from '../../error';

export const Route = createRootRouteWithContext<RouterContext>()({
  component: () => (
    <>
      <div data-react-aria-top-layer id='cm-label-layer' />
      <ToastRegion />
      <Outlet />

      <DevToolsProvider>
        <TanStackRouterDevTools position='bottom-right' toggleButtonProps={{ className: tw`right-16! bottom-3!` }} />
        <ReactQueryDevTools buttonPosition='bottom-right' />
        <ReactScanDevTools />
      </DevToolsProvider>
    </>
  ),
  errorComponent: ErrorComponent,
});
