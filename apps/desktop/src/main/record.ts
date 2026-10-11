/**
 * Recording with the bundled `stresseur` binary:
 * `record --browser <url> --dir <tmp> --json` streams NDJSON events, closing its
 * stdin stops it, then `generate --dir <tmp> --out <tmp>/api-tests --no-verify
 * --no-upload --json` writes the flows, which are returned as text. The
 * temporary folder is removed afterwards. One recording at a time; the process
 * tree is killed when Studio quits.
 */
import { type ChildProcess, execFileSync, spawn } from 'node:child_process';
import { existsSync, promises as fs, rmSync, statSync } from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { createInterface } from 'node:readline';

export type RecordEvent =
  | { calls: number; endpoints: number; sessions: number; type: 'progress' }
  | { calls: number; endpoints?: number; sessions: number; type: 'stopped' }
  | { message: string; type: 'error' }
  | { proxy_url: string; type: 'started' }
  | { type: 'generating' };

export interface GeneratedFile {
  flows: string[];
  /** Relative to the generated api-tests folder. */
  path: string;
  text: string;
}

export type RecordResult<T> = { data: T; ok: true } | { message: string; ok: false };

interface BinaryOptions {
  env?: NodeJS.ProcessEnv;
  isPackaged: boolean;
  platform?: NodeJS.Platform;
  resourcesPath?: string;
}

const isExecutableFile = (file: string) => {
  try {
    return statSync(file).isFile();
  } catch {
    return false;
  }
};

/**
 * Packaged: `<resources>/stresseur(.exe)`. Dev: $STRESSEUR_BIN, else `stresseur` on PATH.
 * Undefined when none is found.
 */
export const resolveStresseurBinary = ({
  env = process.env,
  isPackaged,
  platform = process.platform,
  resourcesPath = process.resourcesPath,
}: BinaryOptions): string | undefined => {
  const exe = platform === 'win32' ? 'stresseur.exe' : 'stresseur';
  if (isPackaged) {
    const bundled = path.join(resourcesPath, exe);
    return isExecutableFile(bundled) ? bundled : undefined;
  }
  const fromEnv = env['STRESSEUR_BIN'];
  if (fromEnv) return isExecutableFile(fromEnv) ? fromEnv : undefined;
  const pathVar = env['PATH'] ?? env['Path'] ?? '';
  for (const dir of pathVar.split(path.delimiter)) {
    if (!dir) continue;
    const candidate = path.join(dir, exe);
    if (isExecutableFile(candidate)) return candidate;
  }
  return undefined;
};

const isRecordEvent = (value: unknown): value is RecordEvent => {
  if (!value || typeof value !== 'object') return false;
  const event = value as Record<string, unknown>;
  switch (event['type']) {
    case 'error':
      return typeof event['message'] === 'string';
    case 'progress':
      return typeof event['calls'] === 'number';
    case 'started':
      return typeof event['proxy_url'] === 'string';
    case 'stopped':
      return typeof event['calls'] === 'number';
    default:
      return false;
  }
};

/** Keeps only the fields of the contract, so nothing else reaches the renderer. */
const pickEvent = (event: RecordEvent): RecordEvent => {
  const number = (value: unknown) => (typeof value === 'number' ? value : 0);
  switch (event.type) {
    case 'error':
      return { message: event.message, type: 'error' };
    case 'generating':
      return { type: 'generating' };
    case 'progress':
      return {
        calls: number(event.calls),
        endpoints: number(event.endpoints),
        sessions: number(event.sessions),
        type: 'progress',
      };
    case 'started':
      return { proxy_url: event.proxy_url, type: 'started' };
    case 'stopped':
      return {
        calls: number(event.calls),
        ...(typeof event.endpoints === 'number' ? { endpoints: event.endpoints } : {}),
        sessions: number(event.sessions),
        type: 'stopped',
      };
  }
};

const parseLines = (child: ChildProcess, onJson: (value: unknown) => void) => {
  if (!child.stdout) return;
  createInterface({ crlfDelay: Infinity, input: child.stdout }).on('line', (line) => {
    const trimmed = line.trim();
    if (!trimmed.startsWith('{')) return;
    try {
      onJson(JSON.parse(trimmed));
    } catch {
      // Not an event.
    }
  });
};

const tail = (chunks: string[]) => chunks.join('').trim().split('\n').slice(-3).join('\n');

/** Kills a process and its children: taskkill /T on Windows, the process group elsewhere. */
export const killTree = (child: ChildProcess, signal: NodeJS.Signals = 'SIGKILL') => {
  const { pid } = child;
  if (pid === undefined || child.exitCode !== null || child.signalCode !== null) return;
  try {
    if (process.platform === 'win32') {
      execFileSync('taskkill', ['/pid', String(pid), '/T', '/F'], { stdio: 'ignore', windowsHide: true });
    } else {
      process.kill(-pid, signal);
    }
  } catch {
    try {
      child.kill(signal);
    } catch {
      // Already gone.
    }
  }
};

const exited = (child: ChildProcess) =>
  new Promise<{ code: null | number; signal: NodeJS.Signals | null }>((resolve) => {
    if (child.exitCode !== null || child.signalCode !== null) {
      resolve({ code: child.exitCode, signal: child.signalCode });
      return;
    }
    child.once('exit', (code, signal) => void resolve({ code, signal }));
  });

const withTimeout = <T>(promise: Promise<T>, ms: number) =>
  new Promise<T | undefined>((resolve) => {
    const timer = setTimeout(() => void resolve(undefined), ms);
    void promise.then((value) => {
      clearTimeout(timer);
      resolve(value);
    });
  });

export interface RecorderOptions {
  /** Resolved lazily, so a missing binary only fails the Record action. */
  binary: () => string | undefined;
  onEvent: (event: RecordEvent) => void;
  /** How long `start` waits for the `started` event. */
  startTimeoutMs?: number;
  /** How long `stop` waits for `record` to exit after stdin closes, before killing it. */
  stopTimeoutMs?: number;
  /** Parent of the temporary recording folders (default: the OS temp folder). */
  tmpRoot?: string;
}

interface Recording {
  child: ChildProcess;
  dir: string;
  exit: Promise<{ code: null | number; signal: NodeJS.Signals | null }>;
  stderr: string[];
}

export const createRecorder = ({
  binary,
  onEvent,
  startTimeoutMs = 60_000,
  stopTimeoutMs = 15_000,
  tmpRoot = os.tmpdir(),
}: RecorderOptions) => {
  let recording: Recording | undefined;
  /** Every process this recorder started and that may still run. */
  const children = new Set<ChildProcess>();
  let busy = false;

  const launch = (bin: string, args: string[]) => {
    const child = spawn(bin, args, {
      // A process group of its own, so the browser it opens can be killed with it.
      detached: process.platform !== 'win32',
      stdio: ['pipe', 'pipe', 'pipe'],
      windowsHide: true,
    });
    children.add(child);
    child.once('exit', () => void children.delete(child));
    child.once('error', () => void children.delete(child));
    const stderr: string[] = [];
    child.stderr.setEncoding('utf8').on('data', (chunk: string) => {
      stderr.push(chunk);
      if (stderr.length > 50) stderr.shift();
    });
    child.stdin.on('error', () => undefined);
    return { child, stderr };
  };

  const cleanup = async (dir: string) => {
    await fs.rm(dir, { force: true, recursive: true }).catch(() => undefined);
  };

  const start = async (url: unknown): Promise<RecordResult<null>> => {
    if (recording || busy) return { message: 'A recording is already running.', ok: false };
    let parsed: undefined | URL;
    try {
      parsed = typeof url === 'string' ? new URL(url) : undefined;
    } catch {
      parsed = undefined;
    }
    if (!parsed || (parsed.protocol !== 'http:' && parsed.protocol !== 'https:'))
      return { message: 'Enter the URL of your app, starting with http:// or https://.', ok: false };

    const bin = binary();
    if (!bin) return { message: 'The stresseur binary was not found.', ok: false };

    busy = true;
    try {
      const dir = await fs.mkdtemp(path.join(tmpRoot, 'stresseur-record-'));
      const { child, stderr } = launch(bin, ['record', '--browser', parsed.href, '--dir', dir, '--json']);
      const exit = exited(child);
      const current: Recording = { child, dir, exit, stderr };
      recording = current;

      let lastError: string | undefined;
      let resolveStarted: (started: boolean) => void = () => undefined;
      const started = new Promise<boolean>((resolve) => (resolveStarted = resolve));
      parseLines(child, (value) => {
        if (!isRecordEvent(value)) return;
        if (value.type === 'error') lastError = value.message;
        if (value.type === 'started') resolveStarted(true);
        onEvent(pickEvent(value));
      });
      const spawnError = new Promise<string>((resolve) => child.once('error', (error) => void resolve(error.message)));

      let timer: NodeJS.Timeout | undefined;
      const outcome = await Promise.race([
        started.then(() => ({ kind: 'started' as const })),
        exit.then(({ code }) => ({ code, kind: 'exited' as const })),
        spawnError.then((message) => ({ kind: 'spawn-error' as const, message })),
        new Promise<{ kind: 'timeout' }>((resolve) => {
          timer = setTimeout(() => void resolve({ kind: 'timeout' }), startTimeoutMs);
        }),
      ]);
      clearTimeout(timer);
      if (outcome.kind === 'started') {
        // Report an unexpected exit (crash, browser closed) to the renderer.
        void exit.then(({ code, signal }) => {
          if (recording !== current || code === 0 || signal !== null) return;
          onEvent({
            message: lastError ?? (tail(stderr) || `Recording exited with code ${String(code)}.`),
            type: 'error',
          });
        });
        return { data: null, ok: true };
      }

      killTree(child);
      recording = undefined;
      await withTimeout(exit, 5_000);
      await cleanup(dir);
      if (outcome.kind === 'spawn-error') return { message: `Could not run stresseur: ${outcome.message}`, ok: false };
      if (outcome.kind === 'timeout') return { message: 'The recording did not start.', ok: false };
      return {
        message: lastError ?? (tail(stderr) || `stresseur record exited with code ${String(outcome.code)}.`),
        ok: false,
      };
    } finally {
      busy = false;
    }
  };

  const generate = async (dir: string): Promise<RecordResult<GeneratedFile[]>> => {
    const bin = binary();
    if (!bin) return { message: 'The stresseur binary was not found.', ok: false };
    const out = path.join(dir, 'api-tests');
    const { child, stderr } = launch(bin, [
      'generate',
      '--dir',
      dir,
      '--out',
      out,
      '--no-verify',
      '--no-upload',
      '--json',
    ]);
    child.stdin.end();

    let generated: undefined | { files?: unknown };
    let lastError: string | undefined;
    parseLines(child, (value) => {
      if (!value || typeof value !== 'object') return;
      const event = value as Record<string, unknown>;
      if (event['type'] === 'generated') generated = event;
      if (event['type'] === 'error' && typeof event['message'] === 'string') lastError = event['message'];
    });
    const spawnError = new Promise<string>((resolve) => child.once('error', (error) => void resolve(error.message)));
    const result = await Promise.race([exited(child), spawnError]);
    if (typeof result === 'string') return { message: `Could not run stresseur: ${result}`, ok: false };
    if (result.code !== 0 || !generated)
      return {
        message: lastError ?? (tail(stderr) || `stresseur generate exited with code ${String(result.code)}.`),
        ok: false,
      };

    const outReal = await fs.realpath(out).catch(() => out);
    const files: GeneratedFile[] = [];
    for (const entry of Array.isArray(generated.files) ? (generated.files as unknown[]) : []) {
      const { flows, path: filePath } = (entry ?? {}) as Record<string, unknown>;
      if (typeof filePath !== 'string') continue;
      const real = await fs.realpath(path.resolve(out, filePath)).catch(() => undefined);
      if (!real?.startsWith(outReal + path.sep)) continue;
      files.push({
        flows: Array.isArray(flows) ? flows.filter((_): _ is string => typeof _ === 'string') : [],
        path: path.relative(outReal, real).split(path.sep).join('/'),
        text: await fs.readFile(real, 'utf8'),
      });
    }
    return { data: files, ok: true };
  };

  const stop = async (): Promise<RecordResult<GeneratedFile[]>> => {
    const current = recording;
    if (!current || busy) return { message: 'No recording is running.', ok: false };
    busy = true;
    try {
      current.child.stdin?.end();
      if ((await withTimeout(current.exit, stopTimeoutMs)) === undefined) {
        killTree(current.child);
        await withTimeout(current.exit, 5_000);
      }
      if (recording !== current) return { message: 'The recording was cancelled.', ok: false };
      onEvent({ type: 'generating' });
      return await generate(current.dir);
    } finally {
      if (recording === current) recording = undefined;
      busy = false;
      await cleanup(current.dir);
    }
  };

  /** Stops everything without generating, and removes the recording. */
  const cancel = async () => {
    const current = recording;
    recording = undefined;
    const running = [...children];
    for (const child of running) killTree(child);
    await Promise.all(running.map((_) => withTimeout(exited(_), 5_000)));
    if (current) await cleanup(current.dir);
  };

  /** Synchronous kill for app quit. */
  const dispose = () => {
    const current = recording;
    recording = undefined;
    for (const child of children) killTree(child);
    if (current && existsSync(current.dir)) rmSync(current.dir, { force: true, recursive: true });
  };

  return { cancel, dispose, isRecording: () => recording !== undefined, start, stop };
};

export type Recorder = ReturnType<typeof createRecorder>;
