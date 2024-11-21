import path from 'node:path';
import { Command } from '@effect/platform';
import { NodeRuntime } from '@effect/platform-node';
import { Config, Console, Effect, Exit, pipe, Scope } from 'effect';
import { app, BrowserWindow } from 'electron';

import { layer } from './runtime.ts';

declare const MAIN_WINDOW_VITE_DEV_SERVER_URL: string;
declare const MAIN_WINDOW_VITE_NAME: string;

const createWindow = () => {
  // Create the browser window.
  const mainWindow = new BrowserWindow({
    width: 800,
    height: 600,
    webPreferences: {
      preload: path.join(import.meta.dirname, 'preload.js'),
    },
  });

  // and load the index.html of the app.
  if (MAIN_WINDOW_VITE_DEV_SERVER_URL) {
    void mainWindow.loadURL(MAIN_WINDOW_VITE_DEV_SERVER_URL);
  } else {
    void mainWindow.loadFile(path.join(import.meta.dirname, `../renderer/${MAIN_WINDOW_VITE_NAME}/index.html`));
  }

  // Open the DevTools.
  mainWindow.webContents.openDevTools();
};

const server = Effect.gen(function* () {
  const dir = yield* Config.string('SERVER_BINARY');

  const server = yield* pipe(
    Command.make(`${dir}-${process.platform}-${process.arch}`),
    Command.env({
      DB_PATH: './',
      DB_MODE: 'local',
      DB_NAME: 'dev-tools',
      DB_ENCRYPTION_KEY: 'some_key',
      HMAC_SECRET: 'secret',
      MASTER_NODE_ENDPOINT: 'h2c://localhost:8090',
      PORT: '8080',
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

// eslint-disable-next-line @typescript-eslint/no-invalid-void-type
const client = Effect.asyncEffect<void, never, never, never, never, Scope.Scope>((callback) =>
  Effect.gen(function* () {
    const scope = yield* Scope.make();

    // This method will be called when Electron has finished
    // initialization and is ready to create browser windows.
    // Some APIs can only be used after this event occurs.
    app.on('ready', createWindow);

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
      createWindow();
    });

    yield* Effect.addFinalizer((exit) => Console.info(`Client exited with status "${exit._tag}"`));
  }),
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

  yield* server;
  yield* client;
});

pipe(program, Effect.scoped, Effect.provide(layer), NodeRuntime.runMain);
