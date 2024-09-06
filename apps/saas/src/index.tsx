import { TransportProvider } from '@connectrpc/connect-query';
import { QueryClientProvider } from '@tanstack/react-query';
import { createRouter, NavigateOptions, RouterProvider, ToOptions } from '@tanstack/react-router';
import { StrictMode } from 'react';
import { RouterProvider as AriaRouterProvider } from 'react-aria-components';
import { createRoot } from 'react-dom/client';

import { routeTree } from './routes/-generated-router-tree';
import { queryClient, transport } from './runtime';

import '@the-dev-tools/ui/fonts';
import './styles.css';
import 'ag-grid-community/styles/ag-grid.css';
import 'ag-grid-community/styles/ag-theme-quartz.css';

const router = createRouter({ routeTree });

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router;
  }
}

declare module 'react-aria-components' {
  interface RouterConfig {
    href: ToOptions;
    routerOptions: Omit<NavigateOptions, keyof ToOptions>;
  }
}

const rootEl = document.getElementById('root');
if (rootEl) {
  const root = createRoot(rootEl);
  root.render(
    <StrictMode>
      <TransportProvider transport={transport}>
        <QueryClientProvider client={queryClient}>
          <AriaRouterProvider
            navigate={(to, options) => router.navigate({ ...to, ...options })}
            useHref={(to) => router.buildLocation(to).href}
          >
            <RouterProvider router={router} />
          </AriaRouterProvider>
        </QueryClientProvider>
      </TransportProvider>
    </StrictMode>,
  );
}
