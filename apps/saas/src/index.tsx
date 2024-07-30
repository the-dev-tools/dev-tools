import { TransportProvider } from '@connectrpc/connect-query';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { RouterProvider } from '@tanstack/react-router';
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';

import { ApiTransport } from '@the-dev-tools/api/transport';

import { router } from './router';
import { Runtime } from './runtime';

import './styles.css';
import '@the-dev-tools/ui/fonts';

const queryClient = new QueryClient();
const transport = Runtime.runSync(ApiTransport);

const rootEl = document.getElementById('root');
if (rootEl) {
  const root = createRoot(rootEl);
  root.render(
    <StrictMode>
      <TransportProvider transport={transport}>
        <QueryClientProvider client={queryClient}>
          <RouterProvider router={router} />
        </QueryClientProvider>
      </TransportProvider>
    </StrictMode>,
  );
}
