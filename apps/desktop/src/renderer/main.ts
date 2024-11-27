/**
 * This file will automatically be loaded by vite and run in the "renderer" context.
 * To learn more about the differences between the "main" and the "renderer" context in
 * Electron, visit:
 *
 * https://electronjs.org/docs/tutorial/application-architecture#main-and-renderer-processes
 *
 * By default, Node.js integration in this file is disabled. When enabling Node.js integration
 * in a renderer process, please be aware of potential security implications. You can read
 * more about security risks here:
 *
 * https://electronjs.org/docs/tutorial/security
 *
 * To enable Node.js integration in this file, open up `main.ts` and enable the `nodeIntegration`
 * flag:
 *
 * ```
 *  // Create the browser window.
 *  mainWindow = new BrowserWindow({
 *    width: 800,
 *    height: 600,
 *    webPreferences: {
 *      nodeIntegration: true
 *    }
 *  });
 * ```
 */

import { BrowserKeyValueStore } from '@effect/platform-browser';
import { Config, ConfigProvider, Effect, Layer, Logger, LogLevel, ManagedRuntime, pipe } from 'effect';

import { ApiLive } from '@the-dev-tools/api/live';
import { ApiTest } from '@the-dev-tools/api/test';
import { app } from '@the-dev-tools/core/index';

const ConfigLive = pipe(
  {
    ...import.meta.env,
    PUBLIC_API_URL: 'http://localhost:8080',
    LOCAL_MODE: true,
  },
  ConfigProvider.fromJson,
  Layer.setConfigProvider,
);

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
