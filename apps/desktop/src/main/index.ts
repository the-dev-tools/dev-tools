import { Command, FetchHttpClient, Path, Url } from '@effect/platform';
import { NodeContext, NodeRuntime } from '@effect/platform-node';
import { CustomPublishOptions } from 'builder-util-runtime';
import { Console, Effect, pipe, Runtime, String } from 'effect';
import { app, BrowserWindow } from 'electron';
import { autoUpdater } from 'electron-updater';

import { CustomUpdateProvider } from './update';
// eslint-disable-next-line import-x/default
import workerPath from './worker?modulePath';

const createWindow = Effect.gen(function* () {
  const path = yield* Path.Path;

  // Create the browser window.
  const mainWindow = new BrowserWindow({
    title: 'DevTools',
    icon: yield* pipe(
      import.meta.resolve('@the-dev-tools/core/assets/favicon/favicon.ico'),
      Url.fromString,
      Effect.flatMap(path.fromFileUrl),
    ),
    width: 800,
    height: 600,
    webPreferences: {
      preload: path.join(import.meta.dirname, '../preload/index.mjs'),
    },
  });

  // and load the index.html of the app.
  if (import.meta.env.DEV && process.env.ELECTRON_RENDERER_URL) {
    void mainWindow.loadURL(process.env.ELECTRON_RENDERER_URL);

    // Open the DevTools.
    mainWindow.webContents.openDevTools();
  } else {
    void mainWindow.loadFile(path.resolve(import.meta.dirname, '../renderer/index.html'));
  }
});

const server = pipe(
  Effect.gen(function* () {
    const path = yield* Path.Path;

    const dist = yield* pipe(
      import.meta.resolve('@the-dev-tools/backend'),
      Url.fromString,
      Effect.flatMap(path.fromFileUrl),
    );

    return yield* pipe(
      path.join(dist, 'backend'),
      String.replaceAll('app.asar', 'app.asar.unpacked'),
      Command.make,
      Command.env({
        DB_MODE: 'local',
        DB_PATH: app.getPath('userData'),
        DB_NAME: 'state',
        // TODO: we probably shouldn't encrypt local database
        DB_ENCRYPTION_KEY: 'secret',
        HMAC_SECRET: 'secret',
      }),
      Command.stdout('inherit'),
      Command.stderr('inherit'),
      Command.start,
    );
  }),
  Effect.ensuring(Console.log('Server exited')),
);

const worker = pipe(
  Command.make(process.execPath, '--experimental-vm-modules', '--disable-warning=ExperimentalWarning', workerPath),
  Command.env({ ELECTRON_RUN_AS_NODE: '1' }),
  Command.stdout('inherit'),
  Command.stderr('inherit'),
  Command.start,
);

const onReady = Effect.gen(function* () {
  autoUpdater.setFeedURL({
    provider: 'custom',
    updateProvider: CustomUpdateProvider,
    runtime: yield* Effect.runtime<Runtime.Runtime.Context<CustomPublishOptions['runtime']>>(),
    repo: 'the-dev-tools/dev-tools',
    project: { name: 'desktop', path: 'apps/desktop' },
  });
  yield* Effect.tryPromise(() => autoUpdater.checkForUpdatesAndNotify());

  yield* createWindow;
});

const onActivate = Effect.gen(function* () {
  if (BrowserWindow.getAllWindows().length > 0) return;
  yield* createWindow;
});

const client = pipe(
  Effect.fn(function* (callback: (_: typeof Effect.void) => void) {
    const runtime = yield* Effect.runtime<
      Effect.Effect.Context<typeof onReady> | Effect.Effect.Context<typeof onActivate>
    >();

    // This method will be called when Electron has finished
    // initialization and is ready to create browser windows.
    // Some APIs can only be used after this event occurs.
    app.on('ready', () => void Runtime.runPromise(runtime)(onReady));

    // Quit when all windows are closed, except on macOS. There, it's common
    // for applications and their menu bar to stay active until the user quits
    // explicitly with Cmd + Q.
    app.on('window-all-closed', () => {
      if (process.platform === 'darwin') return;
      // callback(Scope.close(scope, Exit.void));
      callback(Effect.interrupt);
    });

    // On OS X it's common to re-create a window in the app when the
    // dock icon is clicked and there are no other windows open.
    app.on('activate', () => void Runtime.runPromise(runtime)(onActivate));

    return Effect.void;
  }),
  Effect.asyncEffect,
  Effect.ensuring(Console.log('Client exited')),
);

// In this file you can include the rest of your app's specific main process
// code. You can also put them in separate files and import them here.

pipe(
  Effect.all([import.meta.env.DEV ? Effect.void : server, client, worker], { concurrency: 'unbounded' }),
  Effect.ensuring(
    Effect.gen(function* () {
      yield* Console.log('Program exited');
      yield* Effect.sync(() => void app.quit());
    }),
  ),
  Effect.scoped,
  Effect.provide(NodeContext.layer),
  Effect.provide(FetchHttpClient.layer),
  NodeRuntime.runMain,
);
