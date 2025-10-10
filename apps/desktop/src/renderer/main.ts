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

import { Registry } from '@effect-atom/atom-react';
import { BrowserKeyValueStore } from '@effect/platform-browser';
import { ConfigProvider, Layer, Logger, LogLevel, ManagedRuntime, pipe, Record } from 'effect';
import { ApiLayer } from '@the-dev-tools/client/api/layer';
import { ApiErrorHandlerLive, app } from '@the-dev-tools/client/index';
import packageJson from '../../package.json';

const ConfigLive = pipe(
  {
    ...import.meta.env,
    PUBLIC_LOCAL_MODE: true,
    VERSION: packageJson.version,
  },
  Record.mapKeys((_) => _.replaceAll('__', '.')),
  Record.toEntries,
  (_) => new Map(_ as [string, string][]),
  ConfigProvider.fromMap,
  Layer.setConfigProvider,
);

const layer = pipe(
  ApiLayer,
  Layer.provideMerge(Registry.layer),
  Layer.provideMerge(ApiErrorHandlerLive),
  Layer.provideMerge(ConfigLive),
  Layer.provideMerge(Logger.pretty),
  Layer.provideMerge(Logger.minimumLogLevel(LogLevel.Debug)),
  Layer.provideMerge(BrowserKeyValueStore.layerLocalStorage),
);

const Runtime = ManagedRuntime.make(layer);

void Runtime.runPromise(app);
