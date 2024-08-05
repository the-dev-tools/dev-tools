import { BrowserKeyValueStore } from '@effect/platform-browser';
import { ConfigProvider, Layer, Logger, LogLevel, ManagedRuntime, pipe } from 'effect';

import { MagicClientLive } from '@the-dev-tools/api/auth';
import { ApiClientLive } from '@the-dev-tools/api/client';
import { ApiTransportDev } from '@the-dev-tools/api/transport';

const ConfigLive = pipe(PUBLIC_ENV, ConfigProvider.fromJson, Layer.setConfigProvider);

const layer = pipe(
  Layer.empty,
  Layer.provideMerge(MagicClientLive),
  Layer.provideMerge(ApiTransportDev),
  Layer.provideMerge(ApiClientLive),
  Layer.provideMerge(Logger.pretty),
  Layer.provideMerge(ConfigLive),
  Layer.provideMerge(Logger.minimumLogLevel(LogLevel.Debug)),
  Layer.provideMerge(BrowserKeyValueStore.layerLocalStorage),
);

export const Runtime = ManagedRuntime.make(layer);
