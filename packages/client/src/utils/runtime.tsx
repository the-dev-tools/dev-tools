import { Atom, Registry } from '@effect-atom/atom-react';
import { BrowserKeyValueStore } from '@effect/platform-browser';
import { Layer, Logger, LogLevel, pipe } from 'effect';
import { ApiCollections } from '~/api-new';
import { ApiTransport } from '~/api/transport';

export const layer = pipe(
  ApiCollections.Default,
  Layer.provideMerge(ApiTransport.Default),
  Layer.provideMerge(Registry.layer),
  Layer.provideMerge(Logger.pretty),
  Layer.provideMerge(Logger.minimumLogLevel(LogLevel.Debug)),
  Layer.provideMerge(BrowserKeyValueStore.layerLocalStorage),
);

export const atomRuntime = Atom.runtime(layer);
