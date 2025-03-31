import { Transport } from '@connectrpc/connect';
import { KeyValueStore } from '@effect/platform/KeyValueStore';
import { QueryClient } from '@tanstack/react-query';
import { createRootRouteWithContext } from '@tanstack/react-router';
import { Runtime } from 'effect';

import { AuthTransport, MagicClient } from '~/api/auth';

import { ErrorComponent } from './error';

export interface RouterContext {
  queryClient: QueryClient;
  runtime: Runtime.Runtime<AuthTransport | KeyValueStore | MagicClient>;
  transport: Transport;
}

export const Route = createRootRouteWithContext<RouterContext>()({
  errorComponent: ErrorComponent,
});
