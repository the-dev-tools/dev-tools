/**
 * Stresseur account sign-in, through the same device-code flow and saved login
 * as the `stresseur` CLI (stresseur-cloud docs/api/cli-auth-v1.md). Signing in
 * here signs the CLI in too, and the other way round.
 *
 * The token stays in the main process; the renderer only sees who is signed in.
 */
import { randomBytes } from 'node:crypto';
import { promises as fs } from 'node:fs';
import os from 'node:os';
import path from 'node:path';

export interface Account {
  user: { email: string; name?: string };
  workspace: { name: string; slug: string };
}

export type SignInEvent =
  | { account: Account; kind: 'signed-in' }
  | { kind: 'failed'; message: string }
  | { kind: 'waiting'; url: string; userCode: string };

interface Credentials extends Account {
  api_url: string;
  device_id: string;
  saved_at?: string;
  token: string;
}

export const apiUrl = () => (process.env['STRESSEUR_API_URL'] ?? 'https://app.stresseur.com').replace(/\/+$/, '');

/** Same folder as the CLI: $STRESSEUR_CONFIG_DIR, else the OS config dir + "stresseur". */
export const credentialsPath = (appDataDir: string) =>
  path.join(process.env['STRESSEUR_CONFIG_DIR'] ?? path.join(appDataDir, 'stresseur'), 'credentials.json');

const readCredentials = async (file: string): Promise<Credentials | undefined> => {
  try {
    const value = JSON.parse(await fs.readFile(file, 'utf8')) as Partial<Credentials>;
    if (typeof value.token !== 'string' || typeof value.api_url !== 'string') return undefined;
    return value as Credentials;
  } catch {
    return undefined;
  }
};

const toAccount = ({ user, workspace }: Account): Account => {
  const account: Account = { user: { email: user.email }, workspace: { name: workspace.name, slug: workspace.slug } };
  if (user.name) account.user.name = user.name;
  return account;
};

const sameApi = (saved: Credentials | undefined) => saved?.api_url.replace(/\/+$/, '') === apiUrl();

interface RequestOptions {
  body?: string;
  headers?: Record<string, string>;
  method?: string;
}

const request = (pathname: string, { body, headers, method = 'GET' }: RequestOptions = {}) =>
  fetch(`${apiUrl()}${pathname}`, {
    ...(body === undefined ? {} : { body }),
    headers: { Accept: 'application/json', 'Content-Type': 'application/json', ...headers },
    method,
    signal: AbortSignal.timeout(15_000),
  });

/** The saved login for this API URL, checked with whoami. Offline keeps the saved account. */
export const getAccount = async (file: string): Promise<Account | null> => {
  const saved = await readCredentials(file);
  if (!saved || !sameApi(saved)) return null;

  try {
    const response = await request('/api/cli/whoami', { headers: { Authorization: `Bearer ${saved.token}` } });
    if (response.status === 401) return null;
    if (response.ok) return toAccount({ ...saved, ...((await response.json()) as Partial<Account>) } as Account);
  } catch {
    // Offline: show the saved account.
  }
  return toAccount(saved);
};

interface DeviceCode {
  device_code: string;
  expires_in: number;
  interval: number;
  user_code: string;
  verification_uri_complete: string;
}

const errorMessage = async (response: Response) => {
  try {
    const body = (await response.json()) as { error?: string; error_description?: string; message?: string };
    return body.message ?? body.error_description ?? body.error ?? `HTTP ${response.status}`;
  } catch {
    return `HTTP ${response.status}`;
  }
};

const sleep = (ms: number, signal: AbortSignal) =>
  new Promise<void>((resolve) => {
    const timer = setTimeout(resolve, ms);
    signal.addEventListener('abort', () => {
      clearTimeout(timer);
      resolve();
    });
  });

interface SignInOptions {
  file: string;
  onEvent: (event: SignInEvent) => void;
  openUrl: (url: string) => Promise<void>;
  signal: AbortSignal;
  version: string;
}

export const signIn = async ({ file, onEvent, openUrl, signal, version }: SignInOptions) => {
  const fail = (message: string) => {
    onEvent({ kind: 'failed', message });
  };
  const aborted = () => signal.aborted;

  let code: DeviceCode;
  try {
    const response = await request('/api/cli/device-codes', {
      body: JSON.stringify({ client_version: `Stresseur Studio ${version}`, device_name: os.hostname() }),
      method: 'POST',
    });
    if (!response.ok) {
      fail(await errorMessage(response));
      return;
    }
    code = (await response.json()) as DeviceCode;
  } catch {
    fail(`Could not reach ${apiUrl()}.`);
    return;
  }

  onEvent({ kind: 'waiting', url: code.verification_uri_complete, userCode: code.user_code });
  await openUrl(code.verification_uri_complete);

  let interval = code.interval * 1000;
  const deadline = Date.now() + code.expires_in * 1000;

  while (!aborted() && Date.now() < deadline) {
    await sleep(interval, signal);
    if (aborted()) return;

    let response: Response;
    try {
      response = await request('/api/cli/token', {
        body: JSON.stringify({
          device_code: code.device_code,
          grant_type: 'urn:ietf:params:oauth:grant-type:device_code',
        }),
        method: 'POST',
      });
    } catch {
      continue; // Network blip: keep polling until the code expires.
    }

    if (response.ok) {
      const body = (await response.json()) as Account & { access_token: string };
      const previous = await readCredentials(file);
      const credentials: Credentials = {
        api_url: apiUrl(),
        device_id: previous?.device_id ?? randomBytes(16).toString('hex'),
        saved_at: new Date().toISOString(),
        token: body.access_token,
        user: body.user,
        workspace: body.workspace,
      };
      await fs.mkdir(path.dirname(file), { mode: 0o700, recursive: true });
      await fs.writeFile(file, JSON.stringify(credentials, null, 2) + '\n', { mode: 0o600 });
      onEvent({ account: toAccount(credentials), kind: 'signed-in' });
      return;
    }

    const body = (await response.json().catch(() => ({}))) as { error?: string; message?: string };
    if (body.error === 'authorization_pending') continue;
    if (body.error === 'slow_down') {
      interval += 5000;
      continue;
    }
    fail(body.message ?? 'Sign-in was cancelled.');
    return;
  }

  if (!aborted()) fail('The code expired. Try again.');
};

/** Revokes the token (best effort) and forgets the saved login, even offline. */
export const signOut = async (file: string) => {
  const saved = await readCredentials(file);
  if (saved && sameApi(saved)) {
    await request('/api/cli/token', { headers: { Authorization: `Bearer ${saved.token}` }, method: 'DELETE' }).catch(
      () => undefined,
    );
  }
  await fs.rm(file, { force: true });
};
