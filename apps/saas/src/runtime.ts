import { BrowserKeyValueStore } from '@effect/platform-browser';
import { QueryClient } from '@tanstack/react-query';
import { Config, ConfigProvider, Effect, Layer, Logger, LogLevel, ManagedRuntime, pipe } from 'effect';

import { ApiLive } from '@the-dev-tools/api/live';
import { ApiTest } from '@the-dev-tools/api/test';
import { ApiTransport } from '@the-dev-tools/api/transport';

const ConfigLive = pipe(PUBLIC_ENV, ConfigProvider.fromJson, Layer.setConfigProvider);

const Environment = Config.literal('production', 'development', 'test')('NODE_ENV');

const ApiLayer = Effect.gen(function* () {
  const environment = yield* Environment;
  if (environment === 'test') return ApiTest;
  return ApiLive;
}).pipe(Layer.unwrapEffect);

const layer = pipe(
  ApiLayer,
  Layer.provideMerge(ConfigLive),
  Layer.provideMerge(Logger.pretty),
  Layer.provideMerge(Logger.minimumLogLevel(LogLevel.Debug)),
  Layer.provideMerge(BrowserKeyValueStore.layerLocalStorage),
);

export const Runtime = ManagedRuntime.make(layer);

export const transport = Runtime.runSync(ApiTransport);

export const queryClient = new QueryClient();
