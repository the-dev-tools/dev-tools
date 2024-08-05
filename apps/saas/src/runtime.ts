import { BrowserKeyValueStore } from '@effect/platform-browser';
import { ConfigProvider, Layer, Logger, LogLevel, ManagedRuntime, pipe } from 'effect';

import { MagicClientLive } from '@the-dev-tools/api/auth';
import { ApiClientLive } from '@the-dev-tools/api/client';
import { ApiTransportDev } from '@the-dev-tools/api/transport';

// Does not work without specifying env variables manually
// https://rsbuild.dev/guide/advanced/env-vars#identifiers-matching
const configProvider = ConfigProvider.fromMap(new Map([['PUBLIC_MAGIC_KEY', process.env.PUBLIC_MAGIC_KEY]]));

const layer = pipe(
  Layer.empty,
  Layer.provideMerge(MagicClientLive),
  Layer.provideMerge(ApiTransportDev),
  Layer.provideMerge(ApiClientLive),
  Layer.provideMerge(Logger.pretty),
  Layer.provideMerge(Layer.setConfigProvider(configProvider)),
  Layer.provideMerge(Logger.minimumLogLevel(LogLevel.Debug)),
  Layer.provideMerge(BrowserKeyValueStore.layerLocalStorage),
);

export const Runtime = ManagedRuntime.make(layer);
