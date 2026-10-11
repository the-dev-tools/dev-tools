/**
 * Recording with the bundled `stresseur` binary: `init --json --stop-on-stdin-eof
 * --no-ci --no-pr --dir <dir> (--url <url> | --demo)` detects the app, records it in a
 * fresh browser, generates flows and runs them twice, streaming NDJSON events that go
 * to the renderer as they come. A `done` line on stdin ends the recording. When it
 * exits, a `finished` event carries the generated files (as text) and, for a
 * temporary folder, the recorded .env values, and the temporary folder is removed
 * (kept when verification failed, so a coding agent can fix it there). One run at a
 * time; the process tree is killed when Studio quits.
 */
import { type ChildProcess, execFileSync, spawn } from 'node:child_process';
import { existsSync, promises as fs, rmSync, statSync } from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { createInterface } from 'node:readline';

export interface InitFailure {
  flow: string;
  hint?: string;
  reason: string;
  step: string;
}

/** The `init --json` events (see the init contract). */
export type InitEvent =
  | { browser: string; proxy_url: string; type: 'recording' }
  | { calls: number; endpoints: number; type: 'progress' }
  | { calls: number; sessions: number; type: 'recorded' }
  | { command: string; type: 'starting_app' }
  | { command?: string; source: string; type: 'detected'; url: string }
  | { commands?: string[]; type: 'pr'; url?: string }
  | { env?: { names: string[]; path: string }; files: InitFile[]; type: 'generated' }
  | { failures: InitFailure[]; passed: number; run: number; total: number; type: 'verified' }
  | { green: boolean; type: 'done' }
  | { hint?: string; message: string; type: 'error' }
  | { path: string; type: 'ci'; warnings: string[] }
  | { prompt: string; type: 'fix_prompt' }
  | { type: 'app_ready'; url: string };

export interface InitFile {
  flows: string[];
  path: string;
  steps: number;
}

export interface GeneratedFile {
  flows: string[];
  /** Relative to the api-tests folder (or the run folder when outside it). */
  path: string;
  text: string;
}

/** Studio's own event, after the process exits. */
export interface FinishedEvent {
  code: null | number;
  /** The run folder: the repo, or a temporary folder (kept only when verification failed). */
  dir: string;
  /** Values of the recorded .env names, read from a temporary folder only. */
  env: Record<string, string>;
  files: GeneratedFile[];
  temporary: boolean;
  type: 'finished';
}

export type RecordEvent = FinishedEvent | InitEvent;

export type RecordResult<T> = { data: T; ok: true } | { message: string; ok: false };

export interface StartOptions {
  demo?: boolean;
  /** Repo folder to write flows into; a temporary folder when missing. */
  dir?: string;
  url?: string;
}

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

// Event parsing: only the contract's fields reach the renderer.

const str = (value: unknown) => (typeof value === 'string' ? value : undefined);
const num = (value: unknown) => (typeof value === 'number' && Number.isFinite(value) ? value : 0);
const strings = (value: unknown) =>
  Array.isArray(value) ? value.filter((_): _ is string => typeof _ === 'string') : [];
const record = (value: unknown) =>
  value && typeof value === 'object' && !Array.isArray(value) ? (value as Record<string, unknown>) : undefined;
const optional = <K extends string>(key: K, value: string | undefined) =>
  (value === undefined ? {} : { [key]: value }) as Partial<Record<K, string>>;

export const parseInitEvent = (value: unknown): InitEvent | undefined => {
  const event = record(value);
  if (!event) return undefined;
  switch (event['type']) {
    case 'app_ready': {
      const url = str(event['url']);
      return url === undefined ? undefined : { type: 'app_ready', url };
    }
    case 'ci': {
      const ciPath = str(event['path']);
      return ciPath === undefined ? undefined : { path: ciPath, type: 'ci', warnings: strings(event['warnings']) };
    }
    case 'detected': {
      const url = str(event['url']);
      if (url === undefined) return undefined;
      return {
        ...optional('command', str(event['command'])),
        source: str(event['source']) ?? 'flag',
        type: 'detected',
        url,
      };
    }
    case 'done':
      return { green: event['green'] === true, type: 'done' };
    case 'error': {
      const message = str(event['message']);
      return message === undefined ? undefined : { ...optional('hint', str(event['hint'])), message, type: 'error' };
    }
    case 'fix_prompt': {
      const prompt = str(event['prompt']);
      return prompt === undefined ? undefined : { prompt, type: 'fix_prompt' };
    }
    case 'generated': {
      const files = (Array.isArray(event['files']) ? (event['files'] as unknown[]) : []).flatMap((entry) => {
        const file = record(entry);
        const filePath = str(file?.['path']);
        return filePath === undefined
          ? []
          : [{ flows: strings(file?.['flows']), path: filePath, steps: num(file?.['steps']) }];
      });
      const env = record(event['env']);
      const envPath = str(env?.['path']);
      return {
        ...(envPath !== undefined && { env: { names: strings(env?.['names']), path: envPath } }),
        files,
        type: 'generated',
      };
    }
    case 'pr': {
      const commands = Array.isArray(event['commands']) ? strings(event['commands']) : undefined;
      return { ...(commands && { commands }), ...optional('url', str(event['url'])), type: 'pr' };
    }
    case 'progress':
      return { calls: num(event['calls']), endpoints: num(event['endpoints']), type: 'progress' };
    case 'recorded':
      return { calls: num(event['calls']), sessions: num(event['sessions']), type: 'recorded' };
    case 'recording': {
      const proxy = str(event['proxy_url']);
      return proxy === undefined
        ? undefined
        : { browser: str(event['browser']) ?? 'default', proxy_url: proxy, type: 'recording' };
    }
    case 'starting_app': {
      const command = str(event['command']);
      return command === undefined ? undefined : { command, type: 'starting_app' };
    }
    case 'verified': {
      const failures = (Array.isArray(event['failures']) ? (event['failures'] as unknown[]) : []).flatMap((entry) => {
        const failure = record(entry);
        if (!failure) return [];
        return [
          {
            flow: str(failure['flow']) ?? '',
            ...optional('hint', str(failure['hint'])),
            reason: str(failure['reason']) ?? '',
            step: str(failure['step']) ?? '',
          },
        ];
      });
      return {
        failures,
        passed: num(event['passed']),
        run: num(event['run']),
        total: num(event['total']),
        type: 'verified',
      };
    }
    default:
      return undefined;
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

export const tail = (chunks: string[]) => chunks.join('').trim().split('\n').slice(-3).join('\n');

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

export const exited = (child: ChildProcess) =>
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

/** KEY=VALUE lines, optional `export ` and matching quotes; only the given names. */
export const readDotEnv = (text: string, names: string[]) => {
  const wanted = new Set(names);
  const values: Record<string, string> = {};
  for (const raw of text.split(/\r?\n/)) {
    const match = /^\s*(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)$/.exec(raw);
    if (!match?.[1] || !wanted.has(match[1]) || match[1] in values) continue;
    let value = (match[2] ?? '').trim();
    const quote = value.startsWith('"') ? '"' : value.startsWith("'") ? "'" : undefined;
    if (value.length >= 2 && quote && value.endsWith(quote)) value = value.slice(1, -1);
    values[match[1]] = value;
  }
  return values;
};

export interface RecorderOptions {
  /** Resolved lazily, so a missing binary only fails the Record action. */
  binary: () => string | undefined;
  onEvent: (event: RecordEvent) => void;
  /** How long `start` waits for the first event. */
  startTimeoutMs?: number;
  /** How long a stopped recording may take to report `recorded` before it is killed. */
  stopTimeoutMs?: number;
  /** Parent of the temporary folders (default: the OS temp folder). */
  tmpRoot?: string;
}

interface Run {
  child: ChildProcess;
  dir: string;
  exit: Promise<{ code: null | number; signal: NodeJS.Signals | null }>;
  recorded: boolean;
  stopTimer?: NodeJS.Timeout;
  temporary: boolean;
}

const isHttpUrl = (url: unknown) => {
  try {
    const parsed = typeof url === 'string' ? new URL(url) : undefined;
    return parsed?.protocol === 'http:' || parsed?.protocol === 'https:' ? parsed : undefined;
  } catch {
    return undefined;
  }
};

const isDirectory = async (dir: string) => (await fs.stat(dir).catch(() => undefined))?.isDirectory() === true;

/** The CLI equivalent of a run, for "Show command". */
export const initArgs = ({ demo, dir, url }: { demo: boolean; dir: string; url?: string | undefined }) => [
  'init',
  '--json',
  ...(demo ? ['--demo'] : ['--url', url ?? '']),
  '--dir',
  dir,
  '--stop-on-stdin-eof',
  '--no-ci',
  '--no-pr',
];

export const createRecorder = ({
  binary,
  onEvent,
  startTimeoutMs = 60_000,
  stopTimeoutMs = 15_000,
  tmpRoot = os.tmpdir(),
}: RecorderOptions) => {
  let run: Run | undefined;
  let busy = false;

  const cleanup = async (dir: string) => {
    await fs.rm(dir, { force: true, recursive: true }).catch(() => undefined);
  };

  /** Generated files under the run folder, as text. */
  const readFiles = async (dir: string, files: InitFile[]) => {
    const dirReal = await fs.realpath(dir).catch(() => dir);
    const testsReal = path.join(dirReal, 'api-tests');
    const out: GeneratedFile[] = [];
    for (const file of files) {
      const real = await fs.realpath(path.resolve(dirReal, file.path)).catch(() => undefined);
      if (!real?.startsWith(dirReal + path.sep)) continue;
      const base = real.startsWith(testsReal + path.sep) ? testsReal : dirReal;
      out.push({
        flows: file.flows,
        path: path.relative(base, real).split(path.sep).join('/'),
        text: await fs.readFile(real, 'utf8'),
      });
    }
    return out;
  };

  const readEnv = async (dir: string, env: undefined | { names: string[]; path: string }) => {
    if (!env || env.names.length === 0) return {};
    const dirReal = await fs.realpath(dir).catch(() => dir);
    const real = await fs.realpath(path.resolve(dirReal, env.path)).catch(() => undefined);
    if (!real?.startsWith(dirReal + path.sep)) return {};
    return readDotEnv(await fs.readFile(real, 'utf8').catch(() => ''), env.names);
  };

  const start = async (options: unknown): Promise<RecordResult<{ args: string[]; dir: string }>> => {
    if (run || busy) return { message: 'A recording is already running.', ok: false };
    const { demo, dir, url } = record(options) ?? {};
    const isDemo = demo === true;
    const parsed = isHttpUrl(url);
    if (!isDemo && !parsed)
      return { message: 'Enter the URL of your app, starting with http:// or https://.', ok: false };
    if (dir !== undefined && (typeof dir !== 'string' || !path.isAbsolute(dir) || !(await isDirectory(dir))))
      return { message: 'Pick a folder that exists.', ok: false };

    const bin = binary();
    if (!bin) return { message: 'The stresseur binary was not found.', ok: false };

    busy = true;
    try {
      const temporary = dir === undefined;
      const runDir = typeof dir === 'string' ? dir : await fs.mkdtemp(path.join(tmpRoot, 'stresseur-init-'));
      const args = initArgs({ demo: isDemo, dir: runDir, url: parsed?.href });
      const child = spawn(bin, args, {
        cwd: runDir,
        // A process group of its own, so the app and browser it starts can be killed with it.
        detached: process.platform !== 'win32',
        stdio: ['pipe', 'pipe', 'pipe'],
        windowsHide: true,
      });
      child.stdin.on('error', () => undefined);
      const stderr: string[] = [];
      child.stderr.setEncoding('utf8').on('data', (chunk: string) => {
        stderr.push(chunk);
        if (stderr.length > 50) stderr.shift();
      });
      const exit = exited(child);
      const current: Run = { child, dir: runDir, exit, recorded: false, temporary };
      run = current;

      let generated: Extract<InitEvent, { type: 'generated' }> | undefined;
      let green = false;
      let lastError: string | undefined;
      let resolveFirst: () => void = () => undefined;
      const first = new Promise<void>((resolve) => (resolveFirst = resolve));
      parseLines(child, (value) => {
        const event = parseInitEvent(value);
        if (!event) return;
        // An error before anything else fails `start` instead.
        if (event.type !== 'error') resolveFirst();
        if (event.type === 'recorded') {
          current.recorded = true;
          clearTimeout(current.stopTimer);
        }
        if (event.type === 'generated') generated = event;
        if (event.type === 'done') green = event.green;
        if (event.type === 'error') lastError = event.hint ? `${event.message} ${event.hint}` : event.message;
        if (run === current) onEvent(event);
      });
      const spawnError = new Promise<string>((resolve) => child.once('error', (error) => void resolve(error.message)));

      let timer: NodeJS.Timeout | undefined;
      const outcome = await Promise.race([
        first.then(() => ({ kind: 'started' as const })),
        exit.then(({ code }) => ({ code, kind: 'exited' as const })),
        spawnError.then((message) => ({ kind: 'spawn-error' as const, message })),
        new Promise<{ kind: 'timeout' }>((resolve) => {
          timer = setTimeout(() => void resolve({ kind: 'timeout' }), startTimeoutMs);
        }),
      ]);
      clearTimeout(timer);

      if (outcome.kind !== 'started') {
        killTree(child);
        run = undefined;
        await withTimeout(exit, 5_000);
        if (temporary) await cleanup(runDir);
        if (outcome.kind === 'spawn-error')
          return { message: `Could not run stresseur: ${outcome.message}`, ok: false };
        if (outcome.kind === 'timeout') return { message: 'stresseur init did not start.', ok: false };
        return {
          message: lastError ?? (tail(stderr) || `stresseur init exited with code ${String(outcome.code)}.`),
          ok: false,
        };
      }

      void exit.then(async ({ code, signal }) => {
        clearTimeout(current.stopTimer);
        if (run !== current) return;
        if (code !== 0 && code !== 1 && !lastError)
          onEvent({
            message:
              tail(stderr) ||
              (signal ? `stresseur init was stopped (${signal}).` : `stresseur init exited with code ${String(code)}.`),
            type: 'error',
          });
        const files = generated ? await readFiles(runDir, generated.files).catch(() => []) : [];
        const env = temporary ? await readEnv(runDir, generated?.env).catch(() => ({})) : {};
        // Keep a failed run's folder for the coding agent; a green or empty one goes.
        const keep = temporary && !green && files.length > 0;
        if (temporary && !keep) await cleanup(runDir);
        if (run !== current) return;
        run = undefined;
        onEvent({ code, dir: runDir, env, files, temporary, type: 'finished' });
      });

      return { data: { args, dir: runDir }, ok: true };
    } finally {
      busy = false;
    }
  };

  /** Ends the recording: generating and verifying go on, reported as events. */
  const stop = (): RecordResult<null> => {
    const current = run;
    if (!current) return { message: 'No recording is running.', ok: false };
    if (current.recorded) return { data: null, ok: true };
    current.child.stdin?.write('done\n');
    clearTimeout(current.stopTimer);
    current.stopTimer = setTimeout(() => {
      if (run !== current || current.recorded) return;
      onEvent({ message: 'The recording did not stop.', type: 'error' });
      killTree(current.child);
    }, stopTimeoutMs);
    return { data: null, ok: true };
  };

  /** Stops everything and removes a temporary folder. */
  const cancel = async () => {
    const current = run;
    run = undefined;
    if (!current) return;
    clearTimeout(current.stopTimer);
    killTree(current.child);
    await withTimeout(current.exit, 5_000);
    if (current.temporary) await cleanup(current.dir);
  };

  /** Synchronous kill for app quit. */
  const dispose = () => {
    const current = run;
    run = undefined;
    if (!current) return;
    clearTimeout(current.stopTimer);
    killTree(current.child);
    if (current.temporary && existsSync(current.dir)) rmSync(current.dir, { force: true, recursive: true });
  };

  return { cancel, dispose, isRunning: () => run !== undefined, start, stop };
};

export type Recorder = ReturnType<typeof createRecorder>;
