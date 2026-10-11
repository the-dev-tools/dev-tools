import assert from 'node:assert/strict';
import fs from 'node:fs';
import { createServer, type Server } from 'node:http';
import { type AddressInfo } from 'node:net';
import os from 'node:os';
import nodePath from 'node:path';
import { afterEach, beforeEach, describe, it } from 'node:test';
import { getAccount, signIn, type SignInEvent, signOut } from './account.ts';

const account = {
  user: { email: 'ada@acme.dev', id: 'u1', name: 'Ada' },
  workspace: { id: 'w1', name: 'Acme', slug: 'acme' },
};

let server: Server;
let dir: string;
let file: string;
let calls: string[];
let tokenPolls: number;

beforeEach(async () => {
  dir = fs.mkdtempSync(nodePath.join(os.tmpdir(), 'studio-account-'));
  file = nodePath.join(dir, 'credentials.json');
  calls = [];
  tokenPolls = 0;
  server = createServer((req, res) => {
    calls.push(`${req.method} ${req.url} ${req.headers.authorization ?? ''}`.trim());
    const json = (status: number, body: unknown) => {
      res.writeHead(status, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify(body));
    };
    if (req.method === 'POST' && req.url === '/api/cli/device-codes')
      return void json(200, {
        device_code: 'dev-code',
        expires_in: 60,
        interval: 0,
        user_code: 'WDJB-MJHT',
        verification_uri_complete: 'http://example.test/cli?code=WDJB-MJHT',
      });
    if (req.method === 'POST' && req.url === '/api/cli/token') {
      tokenPolls += 1;
      if (tokenPolls === 1) return void json(400, { error: 'authorization_pending' });
      return void json(200, { access_token: 'stsr_test', token_type: 'Bearer', ...account });
    }
    if (req.url === '/api/cli/whoami')
      return void (req.headers.authorization === 'Bearer stsr_test'
        ? json(200, account)
        : json(401, { error: 'invalid_token' }));
    if (req.method === 'DELETE' && req.url === '/api/cli/token') return void res.writeHead(204).end();
    json(404, {});
  });
  await new Promise<void>((resolve) => void server.listen(0, '127.0.0.1', resolve));
  process.env['STRESSEUR_API_URL'] = `http://127.0.0.1:${(server.address() as AddressInfo).port}/`;
});

afterEach(() => {
  server.close();
  fs.rmSync(dir, { force: true, recursive: true });
  delete process.env['STRESSEUR_API_URL'];
});

const runSignIn = async () => {
  const events: SignInEvent[] = [];
  const opened: string[] = [];
  await signIn({
    file,
    onEvent: (_) => void events.push(_),
    openUrl: (url) => {
      opened.push(url);
      return Promise.resolve();
    },
    signal: new AbortController().signal,
    version: '1.2.0',
  });
  return { events, opened };
};

void describe('account', () => {
  void it('signs in through the device code and saves the CLI credentials file', async () => {
    const { events, opened } = await runSignIn();

    assert.deepEqual(opened, ['http://example.test/cli?code=WDJB-MJHT']);
    assert.deepEqual(
      events.map((_) => _.kind),
      ['waiting', 'signed-in'],
    );
    const saved = JSON.parse(fs.readFileSync(file, 'utf8')) as Record<string, unknown>;
    assert.equal(saved['token'], 'stsr_test');
    assert.equal(saved['api_url'], process.env['STRESSEUR_API_URL']?.replace(/\/$/, ''));
    assert.match(saved['device_id'] as string, /^[0-9a-f]{32}$/);
    if (process.platform !== 'win32') assert.equal(fs.statSync(file).mode & 0o777, 0o600);

    assert.deepEqual(await getAccount(file), {
      user: { email: 'ada@acme.dev', name: 'Ada' },
      workspace: { name: 'Acme', slug: 'acme' },
    });
  });

  void it('keeps the device id across logins, like the CLI', async () => {
    await runSignIn();
    const first = (JSON.parse(fs.readFileSync(file, 'utf8')) as { device_id: string }).device_id;
    tokenPolls = 0;
    await runSignIn();
    assert.equal((JSON.parse(fs.readFileSync(file, 'utf8')) as { device_id: string }).device_id, first);
  });

  void it('treats a revoked token or another API URL as signed out', async () => {
    fs.writeFileSync(file, JSON.stringify({ api_url: process.env['STRESSEUR_API_URL'], token: 'stsr_old', ...account }));
    assert.equal(await getAccount(file), null);

    fs.writeFileSync(file, JSON.stringify({ api_url: 'https://elsewhere.test', token: 'stsr_test', ...account }));
    assert.equal(await getAccount(file), null);
  });

  void it('revokes the token and removes the file on sign-out', async () => {
    await runSignIn();
    await signOut(file);
    assert.equal(fs.existsSync(file), false);
    assert.ok(calls.includes('DELETE /api/cli/token Bearer stsr_test'));
  });
});
