import { BrowserKeyValueStore } from '@effect/platform-browser';
import { ConfigProvider, Layer, Logger, LogLevel, ManagedRuntime, pipe } from 'effect';

import { ApiLayer } from '@the-dev-tools/api/layer';
import { app } from '@the-dev-tools/core/index';

const ConfigLive = pipe(import.meta.env, ConfigProvider.fromJson, Layer.setConfigProvider);

const layer = pipe(
  ApiLayer,
  Layer.provideMerge(ConfigLive),
  Layer.provideMerge(Logger.pretty),
  Layer.provideMerge(Logger.minimumLogLevel(LogLevel.Debug)),
  Layer.provideMerge(BrowserKeyValueStore.layerLocalStorage),
);

const Runtime = ManagedRuntime.make(layer);

void Runtime.runPromise(app);
