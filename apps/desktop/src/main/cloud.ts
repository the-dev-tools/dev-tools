/**
 * Stresseur Cloud API for Studio (`/api/cli/studio/*`, stresseur-cloud
 * docs/api/studio-v1.md), called from the main process with the saved CLI
 * token. The token never reaches the renderer; it only gets the data or a
 * mapped error.
 *
 * Only the endpoints of the contract can be called (see `isAllowedStudioRequest`).
 * Repo and flow lists (and fetched flow files) are cached in the user-data
 * folder and returned with `offline: true` when the network is down.
 */
import { promises as fs } from 'node:fs';
import path from 'node:path';
import { apiUrl, type Credentials, readCredentials, sameApi } from './account.ts';

export type CloudError =
  | { code: string; fixUrl?: string; kind: 'error'; message: string; upgrade?: boolean }
  | { kind: 'offline' }
  | { kind: 'sign-in-again' }
  | { kind: 'signed-out' };

export type CloudResult<T> = { data: T; offline?: boolean; ok: true } | { error: CloudError; ok: false };

type Method = 'GET' | 'POST';

const STUDIO_PREFIX = '/api/cli/studio';

/** One path segment as `encodeURIComponent` writes it. */
const SEGMENT = String.raw`[A-Za-z0-9\-_.!~*'()%]+`;

const ALLOWED: { method: Method; pattern: RegExp }[] = [
  { method: 'GET', pattern: /^\/repos$/ },
  { method: 'GET', pattern: new RegExp(`^/repos/${SEGMENT}/flows$`) },
  { method: 'GET', pattern: new RegExp(`^/repos/${SEGMENT}/files\\?path=${SEGMENT}$`) },
  { method: 'GET', pattern: new RegExp(`^/repos/${SEGMENT}/environments$`) },
  { method: 'POST', pattern: new RegExp(`^/repos/${SEGMENT}/pull-requests$`) },
  { method: 'POST', pattern: new RegExp(`^/repos/${SEGMENT}/runs$`) },
  { method: 'GET', pattern: new RegExp(`^/runs/${SEGMENT}$`) },
];

/** True only for the contract's endpoints (path relative to /api/cli/studio, query included). */
export const isAllowedStudioRequest = (method: string, pathAndQuery: string) => {
  const pathname = pathAndQuery.split('?')[0] ?? '';
  // "." and ".." segments would be collapsed by URL parsing and escape the allow-list.
  if (pathname.split('/').some((_) => _ === '.' || _ === '..' || /^%2e/i.test(_))) return false;
  return ALLOWED.some((_) => _.method === method && _.pattern.test(pathAndQuery));
};

/** A repo-relative flow file path: `api-tests/**\/*.yamlflow.yaml`, no `..`, not absolute. */
export const isRepoFlowPath = (value: unknown): value is string => {
  if (typeof value !== 'string' || value.length > 1024) return false;
  if (value.includes('\\') || value.includes('\0')) return false;
  const segments = value.split('/');
  if (segments[0] !== 'api-tests' || segments.length < 2) return false;
  if (segments.some((_) => _ === '' || _ === '.' || _ === '..')) return false;
  const name = segments.at(-1) ?? '';
  return name.endsWith('.yamlflow.yaml') && name.length > '.yamlflow.yaml'.length;
};

/** URLs Studio may open in the browser: https, or the configured Stresseur URL (http in local dev). */
export const isOpenableUrl = (value: unknown): value is string => {
  if (typeof value !== 'string') return false;
  try {
    const url = new URL(value);
    return url.protocol === 'https:' || url.origin === new URL(apiUrl()).origin;
  } catch {
    return false;
  }
};

const segment = (value: unknown) => (typeof value === 'string' ? encodeURIComponent(value) : '');

const invalid = (message: string): CloudResult<never> => ({
  error: { code: 'invalid_request', kind: 'error', message },
  ok: false,
});

interface ErrorBody {
  error?: unknown;
  fix_url?: unknown;
  message?: unknown;
  upgrade?: unknown;
}

const mapError = async (response: Response): Promise<CloudError> => {
  const body = (await response.json().catch(() => ({}))) as ErrorBody;
  const code = typeof body.error === 'string' && body.error ? body.error : `http_${response.status}`;
  if (response.status === 401 || (response.status === 403 && code === 'insufficient_scope'))
    return { kind: 'sign-in-again' };

  const message =
    typeof body.message === 'string' && body.message
      ? body.message
      : `Stresseur Cloud returned HTTP ${response.status}.`;
  const error: CloudError = { code, kind: 'error', message };
  if (isOpenableUrl(body.fix_url)) error.fixUrl = body.fix_url;
  if (body.upgrade === true) error.upgrade = true;
  return error;
};

interface CacheEntry {
  at: string;
  data: unknown;
}

const MAX_CACHE_ENTRIES = 200;

const createCache = (file: string) => {
  let writes = Promise.resolve();

  const load = async (): Promise<Record<string, CacheEntry>> => {
    try {
      const value = JSON.parse(await fs.readFile(file, 'utf8')) as unknown;
      if (value && typeof value === 'object' && !Array.isArray(value)) return value as Record<string, CacheEntry>;
    } catch {
      // Missing or corrupt: start over.
    }
    return {};
  };

  const get = async (key: string) => {
    const entries = await load();
    return Object.hasOwn(entries, key) ? entries[key]?.data : undefined;
  };

  const set = (key: string, data: unknown) =>
    (writes = writes
      .then(async () => {
        const entries = await load();
        entries[key] = { at: new Date().toISOString(), data };
        const kept = Object.entries(entries)
          .sort(([, a], [, b]) => b.at.localeCompare(a.at))
          .slice(0, MAX_CACHE_ENTRIES);
        await fs.mkdir(path.dirname(file), { recursive: true });
        const temporary = `${file}.${process.pid}.tmp`;
        await fs.writeFile(temporary, JSON.stringify(Object.fromEntries(kept)), { mode: 0o600 });
        await fs.rename(temporary, file);
      })
      .catch(() => undefined));

  return { get, set };
};

export interface CloudOptions {
  /** JSON file for the offline copy, e.g. userData/cloud-cache.json. */
  cacheFile: string;
  /** The CLI's credentials.json (see account.credentialsPath). */
  credentialsFile: string;
  fetch?: typeof fetch;
  timeoutMs?: number;
}

interface RequestOptions {
  body?: unknown;
  /** Save successful responses and serve them when offline. */
  cached?: boolean;
}

export const createCloud = ({
  cacheFile,
  credentialsFile,
  fetch: fetchImpl = globalThis.fetch,
  timeoutMs = 20_000,
}: CloudOptions) => {
  const cache = createCache(cacheFile);

  const request = async <T>(
    method: Method,
    pathAndQuery: string,
    { body, cached = false }: RequestOptions = {},
  ): Promise<CloudResult<T>> => {
    if (!isAllowedStudioRequest(method, pathAndQuery))
      return { error: { code: 'not_allowed', kind: 'error', message: 'This request is not allowed.' }, ok: false };

    const saved = await readCredentials(credentialsFile);
    if (!sameApi(saved)) return { error: { kind: 'signed-out' }, ok: false };

    const base = apiUrl();
    // The cache is per API URL and workspace; the file may lack a workspace.
    const workspace = (saved as Partial<Credentials>).workspace?.slug ?? '';
    const key = [base, workspace, method, pathAndQuery].join(' ');

    let response: Response;
    try {
      response = await fetchImpl(`${base}${STUDIO_PREFIX}${pathAndQuery}`, {
        ...(body === undefined ? {} : { body: JSON.stringify(body) }),
        headers: {
          Accept: 'application/json',
          Authorization: `Bearer ${saved.token}`,
          ...(body === undefined ? {} : { 'Content-Type': 'application/json' }),
        },
        method,
        redirect: 'error',
        signal: AbortSignal.timeout(timeoutMs),
      });
    } catch {
      if (cached) {
        const data = await cache.get(key);
        if (data !== undefined) return { data: data as T, offline: true, ok: true };
      }
      return { error: { kind: 'offline' }, ok: false };
    }

    if (!response.ok) return { error: await mapError(response), ok: false };

    let data: T;
    try {
      data = (await response.json()) as T;
    } catch {
      return {
        error: { code: 'bad_response', kind: 'error', message: 'Stresseur Cloud sent an invalid response.' },
        ok: false,
      };
    }
    if (cached) await cache.set(key, data);
    return { data, ok: true };
  };

  return {
    environments: (repo: unknown) => request('GET', `/repos/${segment(repo)}/environments`),

    file: (repo: unknown, filePath: unknown) =>
      isRepoFlowPath(filePath)
        ? request('GET', `/repos/${segment(repo)}/files?path=${encodeURIComponent(filePath)}`, { cached: true })
        : Promise.resolve(invalid('Flow files must be under api-tests/ and end in .yamlflow.yaml.')),

    flows: (repo: unknown) => request('GET', `/repos/${segment(repo)}/flows`, { cached: true }),

    proposeChange: (repo: unknown, change: unknown) => {
      const { base_sha, body, content, path: filePath, title } = (change ?? {}) as Record<string, unknown>;
      if (!isRepoFlowPath(filePath))
        return Promise.resolve(invalid('Flow files must be under api-tests/ and end in .yamlflow.yaml.'));
      if (
        typeof content !== 'string' ||
        typeof base_sha !== 'string' ||
        (title !== undefined && typeof title !== 'string') ||
        (body !== undefined && typeof body !== 'string')
      )
        return Promise.resolve(invalid('The change is incomplete.'));
      return request('POST', `/repos/${segment(repo)}/pull-requests`, {
        body: {
          base_sha,
          content,
          path: filePath,
          ...(title === undefined ? {} : { title }),
          ...(body ? { body } : {}),
        },
      });
    },

    repos: () => request('GET', '/repos', { cached: true }),

    run: (id: unknown) => request('GET', `/runs/${segment(id)}`),

    startRun: (repo: unknown, run: unknown) => {
      const { environment, flow } = (run ?? {}) as Record<string, unknown>;
      if (typeof environment !== 'string' || typeof flow !== 'string')
        return Promise.resolve(invalid('Choose a flow and an environment.'));
      return request('POST', `/repos/${segment(repo)}/runs`, { body: { environment, flow } });
    },
  };
};

export type Cloud = ReturnType<typeof createCloud>;
