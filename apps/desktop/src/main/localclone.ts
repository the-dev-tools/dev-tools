/**
 * Linked local clones: repo slug → folder of a git clone of that repo.
 *
 * The renderer can only read and write `<clone>/api-tests/**\/*.yamlflow.yaml`,
 * never through a symlink, and can read the values of named variables from
 * `<clone>/.env` (never written). Git runs through execFile, without a shell.
 */
import { execFile } from 'node:child_process';
import { constants, promises as fs } from 'node:fs';
import path from 'node:path';
import { promisify } from 'node:util';
import { isRepoFlowPath } from './cloud.ts';

export type LocalResult<T> = { data: T; ok: true } | { message: string; ok: false };

class LocalCloneError extends Error {}

const fail = (message: string): never => {
  throw new LocalCloneError(message);
};

const toResult = async <T>(run: () => Promise<T>): Promise<LocalResult<T>> => {
  try {
    return { data: await run(), ok: true };
  } catch (error) {
    if (error instanceof LocalCloneError) return { message: error.message, ok: false };
    return { message: error instanceof Error ? error.message : String(error), ok: false };
  }
};

const isMissing = (error: unknown) =>
  error instanceof Error && 'code' in error && (error.code === 'ENOENT' || error.code === 'ENOTDIR');

/** `owner/repo` (lowercase) from an https, ssh or scp-style git remote URL. */
export const remoteRepoName = (remote: string): string | undefined => {
  const value = remote.trim();
  let pathname: string;
  const scp = /^[^@/\s]+@[^:/\s]+:(.+)$/.exec(value);
  if (scp?.[1] && !value.includes('://')) {
    pathname = scp[1];
  } else {
    try {
      const url = new URL(value);
      if (!['git:', 'http:', 'https:', 'ssh:'].includes(url.protocol)) return undefined;
      pathname = decodeURIComponent(url.pathname);
    } catch {
      return undefined;
    }
  }
  const parts = pathname
    .replace(/^\/+|\/+$/g, '')
    .replace(/\.git$/i, '')
    .split('/');
  if (parts.length !== 2 || parts.some((_) => !_)) return undefined;
  return parts.join('/').toLowerCase();
};

export const remoteMatches = (remote: string, fullName: string) =>
  remoteRepoName(remote) !== undefined && remoteRepoName(remote) === fullName.trim().toLowerCase();

/**
 * A small .env parser: `KEY=VALUE`, optional `export ` prefix, `#` comments,
 * single quotes (literal), double quotes (escapes, may span lines).
 */
export const parseDotenv = (text: string) => {
  const values = new Map<string, string>();
  const lines = text.replace(/\r\n?/g, '\n').split('\n');
  for (let index = 0; index < lines.length; index++) {
    const line = (lines[index] ?? '').trim();
    if (!line || line.startsWith('#')) continue;
    const match = /^(?:export\s+)?([A-Za-z_][A-Za-z0-9_.-]*)\s*=\s*(.*)$/.exec(line);
    if (!match?.[1]) continue;
    const key = match[1];
    let raw = match[2] ?? '';

    if (raw.startsWith('"')) {
      // Join lines until the closing quote.
      while (!/^"(?:[^"\\]|\\.)*"/s.test(raw) && index + 1 < lines.length) raw += '\n' + (lines[++index] ?? '');
      const quoted = /^"((?:[^"\\]|\\.)*)"/s.exec(raw)?.[1] ?? raw.slice(1);
      values.set(
        key,
        quoted.replace(/\\(.)/gs, (_, char: string) => ({ n: '\n', r: '\r', t: '\t' })[char] ?? char),
      );
    } else if (raw.startsWith("'")) {
      const end = raw.indexOf("'", 1);
      values.set(key, end === -1 ? raw.slice(1) : raw.slice(1, end));
    } else if (raw.startsWith('`')) {
      const end = raw.indexOf('`', 1);
      values.set(key, end === -1 ? raw.slice(1) : raw.slice(1, end));
    } else {
      values.set(key, raw.replace(/\s+#.*$/, '').trim());
    }
  }
  return values;
};

const ENV_NAME = /^[A-Za-z_][A-Za-z0-9_]*$/;
const MAX_FILE_BYTES = 5 * 1024 * 1024;
const NO_FOLLOW = process.platform === 'win32' ? 0 : constants.O_NOFOLLOW;

const isInside = (root: string, target: string) => target.startsWith(root.endsWith(path.sep) ? root : root + path.sep);

export interface LocalClonesOptions {
  /** Git executable (default `git` on PATH). */
  git?: string;
  /** Shows a folder picker; resolves null when cancelled. */
  pickDirectory: (title: string) => Promise<null | string>;
  /** JSON file for the slug → folder map, e.g. userData/local-clones.json. */
  storeFile: string;
}

export const createLocalClones = ({ git = 'git', pickDirectory, storeFile }: LocalClonesOptions) => {
  const run = promisify(execFile);
  let writes = Promise.resolve();

  const gitOutput = async (cwd: string, ...args: string[]) => {
    const { stdout } = await run(git, ['-C', cwd, ...args], {
      env: { ...process.env, GIT_TERMINAL_PROMPT: '0' },
      timeout: 15_000,
      windowsHide: true,
    });
    return stdout.trim();
  };

  const load = async (): Promise<Record<string, string>> => {
    try {
      const value = JSON.parse(await fs.readFile(storeFile, 'utf8')) as unknown;
      if (value && typeof value === 'object' && !Array.isArray(value)) return value as Record<string, string>;
    } catch {
      // Missing or corrupt: nothing linked.
    }
    return {};
  };

  const update = (change: (store: Record<string, string>) => void) =>
    (writes = writes.then(async () => {
      const store = await load();
      change(store);
      await fs.mkdir(path.dirname(storeFile), { recursive: true });
      const temporary = `${storeFile}.${process.pid}.tmp`;
      await fs.writeFile(temporary, JSON.stringify(store, null, 2) + '\n', { mode: 0o600 });
      await fs.rename(temporary, storeFile);
    }));

  const linked = async (repo: unknown) => {
    if (typeof repo !== 'string' || !repo) return null;
    const store = await load();
    const dir = Object.hasOwn(store, repo) ? store[repo] : undefined;
    return typeof dir === 'string' ? dir : null;
  };

  /** The linked clone's folder, which must still be a real (non-symlink) folder. */
  const cloneRoot = async (repo: unknown) => {
    const dir = (await linked(repo)) ?? fail('No local clone is linked to this repo.');
    const real = await fs.realpath(dir).catch(() => fail(`The local clone at ${dir} is missing.`));
    if (real !== dir) fail(`The local clone at ${dir} has moved. Link it again.`);
    return real;
  };

  /**
   * `<clone>/<file>` for an `api-tests/**\/*.yamlflow.yaml` path, checking every
   * component: no symlinks, folders are folders, the file is a file. With
   * `create`, missing folders are made one at a time (and checked the same way).
   */
  const flowFile = async (root: string, file: unknown, create: boolean) => {
    if (!isRepoFlowPath(file)) return fail('Flow files must be under api-tests/ and end in .yamlflow.yaml.');
    const segments = file.split('/');
    let current = root;
    for (const [index, name] of segments.entries()) {
      current = path.join(current, name);
      const last = index === segments.length - 1;
      let stat;
      try {
        stat = await fs.lstat(current);
      } catch (error) {
        if (!isMissing(error)) throw error;
        if (!create) return fail(`${file} does not exist in the local clone.`);
        if (last) break;
        await fs.mkdir(current);
        stat = await fs.lstat(current);
      }
      if (stat.isSymbolicLink()) fail(`${file} goes through a symlink. Studio does not follow symlinks in a clone.`);
      if (!last && !stat.isDirectory()) fail(`${file} is not inside a folder.`);
      if (last && !stat.isFile()) fail(`${file} is not a file.`);
    }
    if (!isInside(root, await fs.realpath(path.dirname(current)))) fail(`${file} is outside the local clone.`);
    return current;
  };

  return {
    /** Values from `<clone>/.env` for the requested names only. */
    envValues: (repo: unknown, names: unknown) =>
      toResult(async () => {
        if (!Array.isArray(names) || names.some((_) => typeof _ !== 'string' || !ENV_NAME.test(_)))
          fail('Variable names must be letters, digits and underscores.');
        const root = await cloneRoot(repo);
        const file = path.join(root, '.env');
        let real: string;
        try {
          real = await fs.realpath(file);
        } catch (error) {
          if (isMissing(error)) return {};
          throw error;
        }
        if (!isInside(root, real)) fail('The clone’s .env points outside the clone.');
        const stat = await fs.stat(real);
        if (!stat.isFile() || stat.size > MAX_FILE_BYTES) fail('The clone’s .env is not a readable file.');
        const values = parseDotenv(await fs.readFile(real, 'utf8'));
        const result: Record<string, string> = {};
        for (const name of names as string[]) {
          const value = values.get(name);
          if (value !== undefined) result[name] = value;
        }
        return result;
      }),

    get: linked,

    head: (repo: unknown) =>
      toResult(async () => {
        const root = await cloneRoot(repo);
        const sha = await gitOutput(root, 'rev-parse', 'HEAD').catch(() => fail('Could not read the clone’s commit.'));
        if (!/^[0-9a-f]{40,64}$/.test(sha)) fail('Could not read the clone’s commit.');
        return sha;
      }),

    /** Asks for a folder, checks that it is a clone of `full_name`, and links it. Null when cancelled. */
    pick: async (repo: unknown): Promise<LocalResult<string> | null> => {
      const { full_name: fullName, slug } = (repo ?? {}) as Record<string, unknown>;
      if (typeof fullName !== 'string' || typeof slug !== 'string' || !slug)
        return { message: 'Unknown repo.', ok: false };
      const picked = await pickDirectory(`Choose your local clone of ${fullName}`);
      if (picked === null) return null;
      return toResult(async () => {
        const top = await gitOutput(picked, 'rev-parse', '--show-toplevel').catch(() =>
          fail('That folder is not a git clone.'),
        );
        const root = await fs.realpath(top);
        const remote = await gitOutput(root, 'remote', 'get-url', 'origin').catch(() =>
          fail('That clone has no origin remote.'),
        );
        if (!remoteMatches(remote, fullName)) fail(`That clone’s origin is ${remote}, not ${fullName}.`);
        await update((store) => {
          store[slug] = root;
        });
        return root;
      });
    },

    read: (repo: unknown, file: unknown) =>
      toResult(async () => {
        const target = await flowFile(await cloneRoot(repo), file, false);
        const handle = await fs.open(target, constants.O_RDONLY | NO_FOLLOW);
        try {
          if ((await handle.stat()).size > MAX_FILE_BYTES) fail(`${String(file)} is too large.`);
          return await handle.readFile('utf8');
        } finally {
          await handle.close();
        }
      }),

    unlink: async (repo: unknown) => {
      if (typeof repo !== 'string') return;
      await update((store) => {
        if (Object.hasOwn(store, repo)) Reflect.deleteProperty(store, repo);
      });
    },

    write: (repo: unknown, file: unknown, text: unknown) =>
      toResult(async () => {
        if (typeof text !== 'string' || Buffer.byteLength(text) > MAX_FILE_BYTES) fail('The flow file is too large.');
        const target = await flowFile(await cloneRoot(repo), file, true);
        const handle = await fs.open(
          target,
          constants.O_WRONLY | constants.O_CREAT | constants.O_TRUNC | NO_FOLLOW,
          0o644,
        );
        try {
          await handle.writeFile(text as string, 'utf8');
        } finally {
          await handle.close();
        }
        return null;
      }),
  };
};

export type LocalClones = ReturnType<typeof createLocalClones>;
