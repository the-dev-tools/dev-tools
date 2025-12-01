import { Transport } from '@connectrpc/connect';
import { Registry } from '@effect-atom/atom-react';
import { KeyValueStore } from '@effect/platform/KeyValueStore';
import { QueryClient } from '@tanstack/react-query';
import { Runtime } from 'effect';
import { ApiCollections } from '~api';
import { ApiTransport } from '~api/transport';

export interface RouterContext {
  queryClient: QueryClient;
  runtime: Runtime.Runtime<ApiCollections | ApiTransport | KeyValueStore | Registry.AtomRegistry>;
  transport: Transport;
}
