import { BrowserKeyValueStore } from '@effect/platform-browser';
import { QueryClient } from '@tanstack/react-query';
import { Config, ConfigProvider, Effect, Layer, Logger, LogLevel, ManagedRuntime, pipe } from 'effect';

import { MagicClientLive, MagicClientTest } from '@the-dev-tools/api/auth';
import { ApiClientLive } from '@the-dev-tools/api/client';
import { ApiTransport } from '@the-dev-tools/api/transport';
import { ApiTransportLive } from '@the-dev-tools/api/transport.live';
import { ApiTransportTest } from '@the-dev-tools/api/transport.test';
import { FakerLive } from '@the-dev-tools/utils/faker';

const ConfigLive = pipe(PUBLIC_ENV, ConfigProvider.fromJson, Layer.setConfigProvider);

const Environment = Config.literal('production', 'development', 'test')('NODE_ENV');

const ApiLayer = Effect.gen(function* () {
  const environment = yield* Environment;

  if (environment === 'test') {
    return pipe(
      Layer.empty,
      Layer.provideMerge(MagicClientTest),
      Layer.provideMerge(ApiTransportTest),
      Layer.provideMerge(FakerLive),
    );
  }

  return pipe(Layer.empty, Layer.provideMerge(MagicClientLive), Layer.provideMerge(ApiTransportLive));
}).pipe(Layer.unwrapEffect);

const layer = pipe(
  Layer.empty,
  Layer.provideMerge(ApiLayer),
  Layer.provideMerge(ApiClientLive),
  Layer.provideMerge(ConfigLive),
  Layer.provideMerge(Logger.pretty),
  Layer.provideMerge(Logger.minimumLogLevel(LogLevel.Debug)),
  Layer.provideMerge(BrowserKeyValueStore.layerLocalStorage),
);

export const Runtime = ManagedRuntime.make(layer);

export const transport = Runtime.runSync(ApiTransport);

export const queryClient = new QueryClient();
