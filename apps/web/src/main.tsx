import { BrowserKeyValueStore } from '@effect/platform-browser';
import { Config, ConfigProvider, Effect, Layer, Logger, LogLevel, ManagedRuntime, pipe } from 'effect';

import { ApiLive } from '@the-dev-tools/api/live';
import { ApiTest } from '@the-dev-tools/api/test';
import { app } from '@the-dev-tools/core/index';

const ConfigLive = pipe(import.meta.env, ConfigProvider.fromJson, Layer.setConfigProvider);

const Environment = Config.literal('production', 'development', 'test')('MODE');

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

const Runtime = ManagedRuntime.make(layer);

void Runtime.runPromise(app);
