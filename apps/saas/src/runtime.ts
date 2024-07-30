import { BrowserKeyValueStore } from '@effect/platform-browser';
import { Layer, Logger, LogLevel, ManagedRuntime, pipe } from 'effect';

import { ApiClientLive } from '@the-dev-tools/api/client';
import { ApiTransportMock } from '@the-dev-tools/api/transport';

const layer = pipe(
  ApiTransportMock,
  Layer.provideMerge(ApiClientLive),
  Layer.provideMerge(Logger.pretty),
  Layer.provideMerge(Logger.minimumLogLevel(LogLevel.Debug)),
  Layer.provideMerge(BrowserKeyValueStore.layerLocalStorage),
);

export const Runtime = ManagedRuntime.make(layer);
