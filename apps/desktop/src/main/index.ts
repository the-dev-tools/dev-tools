import { Command, FetchHttpClient, Path, Url } from '@effect/platform';
import * as NodeContext from '@effect/platform-node/NodeContext';
import * as NodeRuntime from '@effect/platform-node/NodeRuntime';
import { Config, Console, Effect, pipe, Runtime, String } from 'effect';
import {
  app,
  BrowserWindow,
  dialog,
  Dialog,
  globalShortcut,
  ipcMain,
  Menu,
  nativeTheme,
  protocol,
  shell,
} from 'electron';
import { autoUpdater } from 'electron-updater';
import { execFileSync } from 'node:child_process';
import { existsSync, unlinkSync } from 'node:fs';
import fs from 'node:fs';
import os from 'node:os';
import nodePath from 'node:path';
import { Agent } from 'undici';
import icon from '../../build/icon.ico?asset';
import { legacyDataDirs, migrateDataDir, type MigrationResult } from './migrate-data-dir';
import { dismissRenameNotice, initRenameNotice, RENAME_NOTICE_FILE } from './rename-notice';
import { CustomUpdateProvider, UpdateOptions } from './update';

// TODO(rename): replace build/icon.* with the Stresseur Studio icon once it exists. Keep the current icon until then.

/** Display name. Identifiers (appId, executable, user-data legacy names, sockets) keep their devtools names on purpose. */
const PRODUCT_NAME = 'Stresseur Studio';

/** Links to Stresseur carry UTM tags, since a desktop app sends no referrer. */
const stresseurUrl = (campaign: string) =>
  `https://stresseur.com/?utm_source=studio&utm_medium=app&utm_campaign=${campaign}`;

/**
 * Bring user data over from the pre-rename folder ("DevTools-Studio", and older).
 *
 * This must stay at the top of the main process: it runs before Chromium writes
 * into the user-data folder, before the Go server opens `state.db`, and before
 * anything else reads or writes files there. Dev builds (unpackaged) use their
 * own folder and never touch an installed app's data.
 */
const userDataMigration: MigrationResult = app.isPackaged
  ? migrateDataDir({
      carryFiles: [RENAME_NOTICE_FILE],
      legacyDirs: legacyDataDirs(app.getPath('appData')),
      log: (_) => void console.log(_),
      userDataDir: app.getPath('userData'),
    })
  : { kind: 'current' };

// If the migration could not complete, keep using the intact legacy folder for
// this session so the user still sees their data; the next launch retries.
if (userDataMigration.kind === 'failed') {
  app.setPath('userData', userDataMigration.from);
  app.setPath('sessionData', userDataMigration.from);
}

const showRenameNotice = initRenameNotice(app.getPath('userData'), userDataMigration);

/**
 * On macOS, detect whether the current process is running under Rosetta 2
 * translation (i.e. the x64 build of the app was installed on an Apple
 * Silicon Mac). Rosetta imposes a significant performance penalty — every
 * JS-heavy Electron operation and every server-side Go cycle runs through
 * x86→arm64 translation. Warn the user so they can download the native
 * Apple Silicon build.
 *
 * Apple's documented detection: `sysctl.proc_translated` returns "1" when
 * the calling process is translated. Key is missing / "0" on Intel-native
 * or arm64-native runs.
 */
const warnOnArchitectureMismatch = () => {
  if (os.platform() !== 'darwin') return;
  let translated = '0';
  try {
    translated = execFileSync('sysctl', ['-in', 'sysctl.proc_translated'], { encoding: 'utf8' }).trim();
  } catch {
    return;
  }
  if (translated !== '1') return;

  const choice = dialog.showMessageBoxSync({
    buttons: ['Download Apple Silicon build', 'Continue anyway'],
    cancelId: 1,
    defaultId: 0,
    detail:
      `The x64 (Intel) build of ${PRODUCT_NAME} is running under Rosetta 2 on an Apple Silicon Mac. ` +
      'This makes the window slow to open and the UI sluggish. Install the arm64 (Apple Silicon) build for native performance.',
    message: 'Wrong architecture installed',
    type: 'warning',
  });
  if (choice === 0) void shell.openExternal('https://dev.tools/download');
};

// Workaround to allow unlimited concurrent HTTP/1.1 connections
// https://medium.com/@hnasr/chromes-6-tcp-connections-limit-c199fe550af6
// https://www.electronjs.org/docs/latest/api/command-line-switches#--ignore-connections-limitdomains
app.commandLine.appendSwitch('ignore-connections-limit', 'localhost');

// Register a custom protocol for server IPC
// https://www.electronjs.org/docs/latest/api/protocol
protocol.registerSchemesAsPrivileged([
  {
    privileges: {
      corsEnabled: true,
      supportFetchAPI: true,
    },
    scheme: 'server',
  },
]);

const createWindow = Effect.gen(function* () {
  const path = yield* Path.Path;

  // Create the browser window.
  const mainWindow = new BrowserWindow({
    backgroundColor: nativeTheme.shouldUseDarkColors ? '#18181b' : 'white',
    height: 600,
    icon,
    title: PRODUCT_NAME,
    webPreferences: {
      preload: path.join(import.meta.dirname, '../preload/index.cjs'),
    },
    width: 800,
  });

  // Open external URLs in a browser
  mainWindow.webContents.setWindowOpenHandler((details) => {
    void shell.openExternal(details.url);
    return { action: 'deny' };
  });

  // Never navigate the app window to a web page; open it in the browser
  mainWindow.webContents.on('will-navigate', (event, url) => {
    if (!/^https?:/.test(url) || url.startsWith(process.env.ELECTRON_RENDERER_URL ?? '\0')) return;
    event.preventDefault();
    void shell.openExternal(url);
  });

  // Run cleanup in window
  let canClose = false;
  mainWindow.on('close', (event) => {
    if (canClose) return;
    event.preventDefault();
    mainWindow.webContents.send('on-close');
  });

  ipcMain.on('on-close-done', () => {
    canClose = true;
    mainWindow.close();
  });

  // and load the index.html of the app.
  if (import.meta.env.DEV && process.env.ELECTRON_RENDERER_URL) {
    // Install dev extensions
    const { installExtension, REACT_DEVELOPER_TOOLS } = yield* Effect.tryPromise(
      () => import('electron-devtools-installer'),
    );
    yield* Effect.tryPromise(() =>
      installExtension([REACT_DEVELOPER_TOOLS], { loadExtensionOptions: { allowFileAccess: true } }),
    );

    void mainWindow.loadURL(process.env.ELECTRON_RENDERER_URL);

    // Open the DevTools.
    mainWindow.webContents.openDevTools();
  } else {
    // TODO: re-disable once app is more stable
    // Disable page reload shortcuts
    // globalShortcut.registerAll(['CommandOrControl+R', 'CommandOrControl+Shift+R', 'F5'], () => void {});
    globalShortcut.unregisterAll();

    // Disable toolbar
    mainWindow.setMenu(null);

    void mainWindow.loadFile(path.resolve(import.meta.dirname, '../renderer/index.html'));

    // TODO: remove once app is more stable
    if (
      yield* pipe(
        Config.boolean('OPEN_DEV_TOOLS'),
        Config.orElse(() => Config.succeed(false)),
      )
    )
      mainWindow.webContents.openDevTools();
  }

  return mainWindow;
});

const server = pipe(
  Effect.gen(function* () {
    const path = yield* Path.Path;

    const dist = yield* pipe(
      import.meta.resolve('@the-dev-tools/server'),
      Url.fromString,
      Effect.flatMap(path.fromFileUrl),
    );

    yield* pipe(
      path.join(dist, os.platform() === 'win32' ? 'server.exe' : 'server'),
      String.replaceAll('app.asar', 'app.asar.unpacked'),
      Command.make,
      Command.env({
        // TODO: we probably shouldn't encrypt local database
        DB_ENCRYPTION_KEY: 'secret',
        DB_MODE: 'local',
        DB_NAME: 'state',
        DB_PATH: app.getPath('userData'),
        DEVTOOLS_MODE: 'server',
        HMAC_SECRET: 'secret',
      }),
      Command.stdout('inherit'),
      Command.stderr('inherit'),
      Command.exitCode,
    );

    yield* Effect.interrupt;
  }),
  Effect.ensuring(Console.log('Server exited')),
);

const worker = pipe(
  Effect.gen(function* () {
    const path = yield* Path.Path;

    const bundle = yield* pipe(
      import.meta.resolve('@the-dev-tools/worker-js'),
      Url.fromString,
      Effect.flatMap(path.fromFileUrl),
    );

    yield* pipe(
      Command.make(process.execPath, '--experimental-vm-modules', '--disable-warning=ExperimentalWarning', bundle),
      Command.env({ ELECTRON_RUN_AS_NODE: '1' }),
      Command.stdout('inherit'),
      Command.stderr('inherit'),
      Command.exitCode,
    );

    yield* Effect.interrupt;
  }),
  Effect.ensuring(Console.log('Worker exited')),
);

/**
 * macOS application menu: Electron's default menu, with an About panel and a
 * Help menu for the new name. Windows and Linux hide the menu bar in
 * production (`setMenu(null)`), so they get the same links in the app itself.
 */
const createMacMenu = () => {
  app.setAboutPanelOptions({
    applicationName: PRODUCT_NAME,
    applicationVersion: app.getVersion(),
    credits: 'Formerly DevTools Studio\nhttps://dev.tools',
  });

  return Menu.buildFromTemplate([
    { role: 'appMenu' },
    { role: 'fileMenu' },
    { role: 'editMenu' },
    { role: 'viewMenu' },
    { role: 'windowMenu' },
    {
      role: 'help',
      submenu: [
        { click: () => void shell.openExternal(stresseurUrl('help_menu')), label: 'Stresseur: AI test engineer' },
        { type: 'separator' },
        { click: () => void shell.openExternal('https://dev.tools'), label: 'DevTools (dev.tools)' },
      ],
    },
  ]);
};

const onReady = Effect.gen(function* () {
  const path = yield* Path.Path;

  // Warn (and offer a download link) if the x64 build is running under Rosetta
  // on Apple Silicon — one of the common "why is it so slow?" footguns.
  yield* Effect.sync(warnOnArchitectureMismatch);

  autoUpdater.autoDownload = false;
  autoUpdater.setFeedURL({
    provider: 'custom',
    update: {
      project: { name: 'desktop', path: 'apps/desktop' },
      repo: 'the-dev-tools/dev-tools',
      runtime: yield* Effect.runtime<Runtime.Runtime.Context<UpdateOptions['runtime']>>(),
    },
    updateProvider: CustomUpdateProvider,
  });

  let socketPath = path.join(os.tmpdir(), 'the-dev-tools', 'server.socket');
  if (os.platform() === 'win32') socketPath = '\\\\.\\pipe\\the-dev-tools_server.socket';

  // Redirect server IPC into a UDS
  // https://nodejs.org/api/globals.html#custom-dispatcher
  // https://undici.nodejs.org/#/docs/api/Client?id=parameter-connectoptions
  const dispatcher = new Agent({
    socketPath,

    // Disable timeout for sync streams
    bodyTimeout: 0,
    headersTimeout: 0,
  });
  protocol.handle('server', (rawRequest) => {
    const url = rawRequest.url.replace('server://', 'http://the-dev-tools:0/');
    let request = new Request(url, rawRequest);
    request = new Request(request, { dispatcher } as never);
    return fetch(request).catch(() => new Response(null, { status: 503 }));
  });

  if (os.platform() === 'darwin') Menu.setApplicationMenu(createMacMenu());

  const mainWindow = yield* createWindow;

  ipcMain.handle('dialog', <T extends keyof Dialog>(_event: unknown, method: T, ...options: Parameters<Dialog[T]>) => {
    const methodFunction = dialog[method] as (...options: Parameters<Dialog[T]>) => ReturnType<Dialog[T]>;
    return methodFunction(...options);
  });

  ipcMain.handle('update:check', () =>
    autoUpdater.checkForUpdates().then((_) => (_?.isUpdateAvailable ? _.updateInfo.version : null)),
  );
  ipcMain.on('update:start', () => void autoUpdater.downloadUpdate());
  autoUpdater.on('download-progress', (_) => void mainWindow.webContents.send('update:progress', _));
  autoUpdater.on('update-downloaded', () => void autoUpdater.quitAndInstall());

  ipcMain.handle('rename-notice:get', () => showRenameNotice);
  ipcMain.on('rename-notice:dismiss', () => void dismissRenameNotice(app.getPath('userData')));

  ipcMain.handle('server:wipe-and-restart', () => {
    const dbDir = app.getPath('userData');
    for (const suffix of ['', '-wal', '-shm']) {
      const file = nodePath.join(dbDir, `state.db${suffix}`);
      try {
        if (existsSync(file)) unlinkSync(file);
      } catch (e) {
        console.error(`Failed to delete ${file}:`, e);
      }
    }
    app.relaunch();
    app.exit(0);
  });

  // Agent logging
  const logDir = path.join(app.getPath('userData'), 'logs', 'agent');
  fs.mkdirSync(logDir, { recursive: true });

  ipcMain.on('agent-log:write', (_event, fileName: string, jsonLine: string) => {
    const filePath = path.join(logDir, path.basename(fileName));
    void fs.promises.appendFile(filePath, jsonLine);
  });

  ipcMain.on('agent-log:cleanup', () => {
    const maxAge = 7 * 24 * 60 * 60 * 1000;
    fs.readdir(logDir, (err, files) => {
      if (err) return;
      const now = Date.now();
      for (const file of files) {
        const filePath = path.join(logDir, file);
        fs.stat(filePath, (err, stats) => {
          if (err) return;
          if (now - stats.mtimeMs > maxAge) void fs.promises.unlink(filePath).catch(() => undefined);
        });
      }
    });
  });
});

const onActivate = Effect.gen(function* () {
  if (BrowserWindow.getAllWindows().length > 0) return;
  yield* createWindow;
});

let canQuit = false;
const client = pipe(
  Effect.fn(function* (callback: (_: typeof Effect.void) => void) {
    const runtime = yield* Effect.runtime<
      Effect.Effect.Context<typeof onActivate> | Effect.Effect.Context<typeof onReady>
    >();

    // This method will be called when Electron has finished
    // initialization and is ready to create browser windows.
    // Some APIs can only be used after this event occurs.
    app.on('ready', () => void Runtime.runPromise(runtime)(onReady));

    // Quit when all windows are closed, except on macOS. There, it's common
    // for applications and their menu bar to stay active until the user quits
    // explicitly with Cmd + Q.
    app.on('window-all-closed', () => {
      // TODO: re-enable with improved instanc management
      // if (process.platform === 'darwin') return;
      app.quit();
    });

    app.on('before-quit', (event) => {
      if (canQuit) return;
      event.preventDefault();
      callback(Effect.interrupt);
      canQuit = true;
    });

    // On OS X it's common to re-create a window in the app when the
    // dock icon is clicked and there are no other windows open.
    app.on('activate', () => void Runtime.runPromise(runtime)(onActivate));

    return Effect.interrupt;
  }),
  Effect.asyncEffect,
  Effect.ensuring(Console.log('Client exited')),
);

const desktop = pipe(
  Effect.all([import.meta.env.DEV ? Effect.void : server, client, worker], { concurrency: 'unbounded' }),
  Effect.ensuring(Console.log('Program exited')),
  Effect.ensuring(
    Effect.sync(() => {
      canQuit = true;
      app.quit();
    }),
  ),
  Effect.scoped,
);

const args = process.argv.slice(process.defaultApp ? 2 : 1);
const cli = pipe(
  Effect.gen(function* () {
    const path = yield* Path.Path;

    const dist = yield* pipe(
      import.meta.resolve('@the-dev-tools/server'),
      Url.fromString,
      Effect.flatMap(path.fromFileUrl),
    );

    const bin = pipe(
      path.join(dist, os.platform() === 'win32' ? 'server.exe' : 'server'),
      String.replaceAll('app.asar', 'app.asar.unpacked'),
    );

    yield* pipe(
      Command.make(bin, ...args),
      Command.env({ DEVTOOLS_MODE: 'cli' }),
      Command.stdout('inherit'),
      Command.stderr('inherit'),
      Command.exitCode,
    );

    app.quit();
  }),
);

const main = args.length > 0 ? cli : desktop;

pipe(main, Effect.provide(NodeContext.layer), Effect.provide(FetchHttpClient.layer), NodeRuntime.runMain);
