import { Command, Path, Url } from '@effect/platform';
import { NodeContext, NodeRuntime } from '@effect/platform-node';
import { Console, Effect, Exit, pipe, Runtime, Scope, String } from 'effect';
import { app, BrowserWindow } from 'electron';

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
  if (import.meta.env.DEV && process.env['ELECTRON_RENDERER_URL']) {
    void mainWindow.loadURL(process.env['ELECTRON_RENDERER_URL']);

    // Open the DevTools.
    mainWindow.webContents.openDevTools();
  } else {
    void mainWindow.loadFile(path.resolve(import.meta.dirname, '../renderer/index.html'));
  }
});

const server = Effect.gen(function* () {
  const path = yield* Path.Path;

  const dist = yield* pipe(
    import.meta.resolve('@the-dev-tools/backend'),
    Url.fromString,
    Effect.flatMap(path.fromFileUrl),
  );

  const server = yield* pipe(
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

  yield* Effect.addFinalizer((exit) =>
    Effect.gen(function* () {
      yield* server.kill();
      yield* Console.info(`Server exited with status "${exit._tag}" and code "${yield* server.exitCode}"`);
    }).pipe(Effect.ignore),
  );

  return server;
});

const client = pipe(
  Effect.fn(function* (callback: (_: typeof Effect.void) => void) {
    const scope = yield* Scope.make();
    const runtime = yield* Effect.runtime<Effect.Effect.Context<typeof createWindow>>();

    // This method will be called when Electron has finished
    // initialization and is ready to create browser windows.
    // Some APIs can only be used after this event occurs.
    app.on('ready', () => void Runtime.runSync(runtime)(createWindow));

    // Quit when all windows are closed, except on macOS. There, it's common
    // for applications and their menu bar to stay active until the user quits
    // explicitly with Cmd + Q.
    app.on('window-all-closed', () => {
      if (process.platform === 'darwin') return;
      callback(Scope.close(scope, Exit.void));
    });

    // On OS X it's common to re-create a window in the app when the
    // dock icon is clicked and there are no other windows open.
    app.on('activate', () => {
      if (BrowserWindow.getAllWindows().length > 0) return;
      Runtime.runSync(runtime)(createWindow);
    });

    yield* Effect.addFinalizer((exit) => Console.info(`Client exited with status "${exit._tag}"`));

    return Effect.void;
  }),
  Effect.asyncEffect,
);

// In this file you can include the rest of your app's specific main process
// code. You can also put them in separate files and import them here.
const program = Effect.gen(function* () {
  yield* Effect.addFinalizer((exit) =>
    Effect.gen(function* () {
      yield* Console.info(`Program exited with status "${exit._tag}"`);
      yield* Effect.sync(() => void app.quit());
    }),
  );

  if (!import.meta.env.DEV) yield* server;
  yield* client;
});

pipe(program, Effect.scoped, Effect.provide(NodeContext.layer), NodeRuntime.runMain);
