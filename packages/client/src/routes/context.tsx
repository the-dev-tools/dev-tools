import { Transport } from '@connectrpc/connect';
import { Registry } from '@effect-atom/atom-react';
import { KeyValueStore } from '@effect/platform/KeyValueStore';
import { QueryClient } from '@tanstack/react-query';
import { Runtime } from 'effect';
import { DataClient } from '~data-client';

export interface RouterContext {
  dataClient: DataClient;
  queryClient: QueryClient;
  runtime: Runtime.Runtime<KeyValueStore | Registry.AtomRegistry>;
  transport: Transport;
}
