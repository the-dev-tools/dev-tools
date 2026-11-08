/**
 * The sample shop `init --demo` records, for running its flows again in Studio. `stresseur
 * demo --json` starts on the first local run of a demo flow and keeps serving until Studio
 * quits; its `app_ready` URL becomes the run's APP_URL. Flows imported from a demo run are
 * remembered by id together with that run's .env values, which a run passes as #env:
 * overrides, so the demo never touches the Global Environment (APP_URL there is the user's
 * own app).
 */
import { type ChildProcess, spawn } from 'node:child_process';
import { promises as fs } from 'node:fs';
import path from 'node:path';
import { createInterface } from 'node:readline';
import { exited, killTree, tail } from './record.ts';

export type DemoResult<T> = { data: T; ok: true } | { message: string; ok: false };

export interface DemoAppOptions {
  /** Resolved lazily, so a missing binary only fails a demo run. */
  binary: () => string | undefined;
  /** How long `ensure` waits for `app_ready`. */
  startTimeoutMs?: number;
}

/** `stresseur demo --json`, one process for the whole session. */
export const createDemoApp = ({ binary, startTimeoutMs = 30_000 }: DemoAppOptions) => {
  let child: ChildProcess | undefined;
  let ready: Promise<DemoResult<{ url: string }>> | undefined;

  const start = (bin: string) =>
    new Promise<DemoResult<{ url: string }>>((resolve) => {
      const current = spawn(bin, ['demo', '--json'], {
        // A process group of its own, so whatever it starts goes with it.
        detached: process.platform !== 'win32',
        stdio: ['pipe', 'pipe', 'pipe'],
        windowsHide: true,
      });
      child = current;
      current.stdin.on('error', () => undefined);
      const stderr: string[] = [];
      current.stderr.setEncoding('utf8').on('data', (chunk: string) => {
        stderr.push(chunk);
        if (stderr.length > 50) stderr.shift();
      });

      let settled = false;
      const settle = (result: DemoResult<{ url: string }>) => {
        if (settled) return;
        settled = true;
        clearTimeout(timer);
        if (!result.ok) {
          killTree(current);
          if (child === current) child = undefined;
        }
        resolve(result);
      };
      const timer = setTimeout(
        () => void settle({ message: 'The sample shop did not start.', ok: false }),
        startTimeoutMs,
      );

      createInterface({ crlfDelay: Infinity, input: current.stdout }).on('line', (line) => {
        try {
          const event = JSON.parse(line.trim()) as { type?: unknown; url?: unknown };
          if (event.type === 'app_ready' && typeof event.url === 'string')
            settle({ data: { url: event.url }, ok: true });
        } catch {
          // Not an event.
        }
      });
      current.once(
        'error',
        (error) => void settle({ message: `Could not run stresseur: ${error.message}`, ok: false }),
      );
      void exited(current).then(({ code }) => {
        // Gone: the next run starts it again.
        if (child === current) {
          child = undefined;
          ready = undefined;
        }
        settle({ message: tail(stderr) || `stresseur demo exited with code ${String(code)}.`, ok: false });
      });
    });

  /** The running sample shop's URL, starting it when needed. */
  const ensure = (): Promise<DemoResult<{ url: string }>> => {
    if (ready) return ready;
    const bin = binary();
    if (!bin) return Promise.resolve({ message: 'The stresseur binary was not found.', ok: false });
    const pending = start(bin).then((result) => {
      if (!result.ok && ready === pending) ready = undefined;
      return result;
    });
    ready = pending;
    return pending;
  };

  /** Stops it (app quit). */
  const dispose = () => {
    const current = child;
    child = undefined;
    ready = undefined;
    if (!current) return;
    current.stdin?.write('done\n');
    killTree(current);
  };

  return { dispose, ensure, isRunning: () => child !== undefined };
};

export type DemoApp = ReturnType<typeof createDemoApp>;

// Flows imported from a demo run

interface DemoStore {
  /** The demo run's .env values (sample-shop login), passed as #env: overrides. */
  env: Record<string, string>;
  /** Canonical flow ids. */
  flowIds: string[];
}

const isStringRecord = (value: unknown): value is Record<string, string> =>
  !!value &&
  typeof value === 'object' &&
  !Array.isArray(value) &&
  Object.values(value as Record<string, unknown>).every((_) => typeof _ === 'string');

const ENV_NAME = /^[A-Za-z_][A-Za-z0-9_]*$/;

export const createDemoFlows = ({ storeFile }: { storeFile: string }) => {
  const read = async (): Promise<DemoStore> => {
    try {
      const parsed = JSON.parse(await fs.readFile(storeFile, 'utf8')) as Partial<DemoStore>;
      return {
        env: isStringRecord(parsed.env) ? parsed.env : {},
        flowIds: Array.isArray(parsed.flowIds) ? parsed.flowIds.filter((_) => typeof _ === 'string') : [],
      };
    } catch {
      return { env: {}, flowIds: [] };
    }
  };

  const write = async (store: DemoStore) => {
    await fs.mkdir(path.dirname(storeFile), { recursive: true });
    const tmp = `${storeFile}.tmp`;
    await fs.writeFile(tmp, JSON.stringify(store), { mode: 0o600 });
    await fs.rename(tmp, storeFile);
  };

  /** Remembers flows imported from a demo run; its .env values replace earlier ones. */
  const mark = async (flowIds: unknown, env: unknown): Promise<DemoResult<null>> => {
    if (!Array.isArray(flowIds) || !flowIds.every((_) => typeof _ === 'string' && _.length > 0))
      return { message: 'Flow ids must be strings.', ok: false };
    const values = isStringRecord(env)
      ? Object.fromEntries(Object.entries(env).filter(([key]) => ENV_NAME.test(key)))
      : {};
    const store = await read();
    await write({
      env: { ...store.env, ...values },
      flowIds: [...new Set([...store.flowIds, ...(flowIds as string[])])],
    });
    return { data: null, ok: true };
  };

  /** The demo run's .env values for a demo flow; undefined for any other flow. */
  const envFor = async (flowId: unknown) => {
    if (typeof flowId !== 'string') return undefined;
    const store = await read();
    return store.flowIds.includes(flowId) ? store.env : undefined;
  };

  return { envFor, mark };
};

export type DemoFlows = ReturnType<typeof createDemoFlows>;

/**
 * #env: overrides for a local run: none for an ordinary flow; for a demo flow the demo's
 * .env values and the running sample shop's URL as APP_URL.
 */
export const demoRunEnv = async (
  flows: DemoFlows,
  app: Pick<DemoApp, 'ensure'>,
  flowId: unknown,
): Promise<DemoResult<null | Record<string, string>>> => {
  const env = await flows.envFor(flowId);
  if (!env) return { data: null, ok: true };
  const started = await app.ensure();
  if (!started.ok) return started;
  return { data: { ...env, APP_URL: started.data.url }, ok: true };
};
