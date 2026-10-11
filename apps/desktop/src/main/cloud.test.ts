import assert from 'node:assert/strict';
import fs from 'node:fs';
import { createServer, type Server } from 'node:http';
import { type AddressInfo } from 'node:net';
import os from 'node:os';
import nodePath from 'node:path';
import { afterEach, beforeEach, describe, it } from 'node:test';
import { createCloud, isAllowedStudioRequest, isOpenableUrl, isRepoFlowPath } from './cloud.ts';

const repos = {
  repos: [
    {
      default_branch: 'main',
      flow_count: 2,
      full_name: 'acme/shop',
      head_sha: 'a'.repeat(40),
      records_ci: true,
      slug: 'shop',
      url: 'https://app.stresseur.test/r/shop',
    },
  ],
  workspace: { name: 'Acme', slug: 'acme' },
};

const aiStream = [
  'event: message_start\ndata: {"type":"message_start","message":{"usage":{"input_tokens":12}}}\n\n',
  'event: content_block_delta\ndata: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hi"}}\n\n',
  'event: message_stop\ndata: {"type":"message_stop"}\n\n',
];

let server: Server;
let dir: string;
let credentialsFile: string;
let cacheFile: string;
let baseUrl: string;
let calls: string[];
let reply: (url: string, method: string) => undefined | { body: unknown; status: number };

beforeEach(async () => {
  dir = fs.mkdtempSync(nodePath.join(os.tmpdir(), 'studio-cloud-'));
  credentialsFile = nodePath.join(dir, 'credentials.json');
  cacheFile = nodePath.join(dir, 'cloud-cache.json');
  calls = [];
  reply = () => undefined;
  server = createServer((req, res) => {
    let body = '';
    req.on('data', (chunk: Buffer) => (body += chunk.toString()));
    req.on('end', () => {
      calls.push(`${req.method} ${req.url} ${req.headers.authorization ?? ''} ${body}`.trim());
      const custom = reply(req.url ?? '', req.method ?? '');
      const json = (status: number, value: unknown) => {
        res.writeHead(status, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify(value));
      };
      if (custom) return void json(custom.status, custom.body);
      if (req.headers.authorization !== 'Bearer stsr_test') return void json(401, { error: 'invalid_token' });
      if (req.url === '/api/cli/studio/repos') return void json(200, repos);
      if (req.url === '/api/cli/studio/repos/shop/flows')
        return void json(200, {
          flows: [{ name: 'checkout', path: 'api-tests/checkout.yamlflow.yaml' }],
          head_sha: 'a',
        });
      if (req.url?.startsWith('/api/cli/studio/repos/shop/files?'))
        return void json(200, { path: new URL(req.url, 'http://x').searchParams.get('path'), text: 'flows: []' });
      if (req.url === '/api/cli/studio/repos/shop/pull-requests' && req.method === 'POST')
        return void json(200, { branch: 'stresseur/studio/u/1', main_moved: false, number: 7, url: 'https://gh/pr/7' });
      if (req.url === '/api/cli/studio/ai/messages' && req.method === 'POST') {
        res.writeHead(200, { 'Content-Type': 'text/event-stream' });
        // Split mid-event to check the reader buffers lines.
        const sse = aiStream.join('');
        res.write(sse.slice(0, 30));
        res.end(sse.slice(30));
        return;
      }
      json(404, { error: 'not_found', message: 'Not found.' });
    });
  });
  await new Promise<void>((resolve) => void server.listen(0, '127.0.0.1', resolve));
  baseUrl = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
  process.env['STRESSEUR_API_URL'] = baseUrl;
  fs.writeFileSync(
    credentialsFile,
    JSON.stringify({
      api_url: baseUrl,
      token: 'stsr_test',
      user: { email: 'ada@acme.dev' },
      workspace: repos.workspace,
    }),
  );
});

afterEach(() => {
  server.close();
  fs.rmSync(dir, { force: true, recursive: true });
  delete process.env['STRESSEUR_API_URL'];
});

const cloud = () => createCloud({ cacheFile, credentialsFile, timeoutMs: 2_000 });

void describe('cloud allow-list', () => {
  void it('allows only the contract endpoints', () => {
    assert.ok(isAllowedStudioRequest('GET', '/repos'));
    assert.ok(isAllowedStudioRequest('GET', '/repos/shop/flows'));
    assert.ok(isAllowedStudioRequest('GET', '/repos/shop/files?path=api-tests%2Fa.yamlflow.yaml'));
    assert.ok(isAllowedStudioRequest('GET', '/repos/shop/environments'));
    assert.ok(isAllowedStudioRequest('POST', '/repos/shop/pull-requests'));
    assert.ok(isAllowedStudioRequest('POST', '/repos/shop/runs'));
    assert.ok(isAllowedStudioRequest('GET', '/runs/r_1'));
    assert.ok(isAllowedStudioRequest('GET', '/ai/credits'));
    assert.ok(isAllowedStudioRequest('POST', '/ai/messages'));

    assert.ok(!isAllowedStudioRequest('POST', '/repos'));
    assert.ok(!isAllowedStudioRequest('DELETE', '/repos/shop/flows'));
    assert.ok(!isAllowedStudioRequest('GET', '/repos/shop'));
    assert.ok(!isAllowedStudioRequest('GET', '/repos/a/b/flows'));
    assert.ok(!isAllowedStudioRequest('GET', '/repos/../whoami'));
    assert.ok(!isAllowedStudioRequest('GET', '/repos/../flows'));
    assert.ok(!isAllowedStudioRequest('GET', '/runs/..'));
    assert.ok(!isAllowedStudioRequest('GET', '/whoami'));
    assert.ok(!isAllowedStudioRequest('GET', '/ai/messages'));
    assert.ok(!isAllowedStudioRequest('POST', '/ai/credits'));
    assert.ok(!isAllowedStudioRequest('GET', '/repos/shop/files?path=x&token=1'));
  });

  void it('encodes slugs and never sends a request outside the list', async () => {
    const api = cloud();
    const dots = await api.run('..');
    assert.ok(!dots.ok && dots.error.kind === 'error' && dots.error.code === 'not_allowed');
    assert.ok(!(await api.flows('')).ok);
    assert.ok(!(await api.flows(42)).ok);
    assert.equal(calls.length, 0);

    // Slashes stay inside one encoded segment, so the server only sees an unknown repo.
    const traversal = await api.flows('../../whoami');
    assert.ok(!traversal.ok && traversal.error.kind === 'error' && traversal.error.code === 'not_found');
    assert.deepEqual(
      calls.map((_) => _.split(' ')[1]),
      ['/api/cli/studio/repos/..%2F..%2Fwhoami/flows'],
    );
  });

  void it('checks flow file paths', async () => {
    assert.ok(isRepoFlowPath('api-tests/checkout.yamlflow.yaml'));
    assert.ok(isRepoFlowPath('api-tests/shop/checkout.yamlflow.yaml'));
    for (const bad of [
      'checkout.yamlflow.yaml',
      '/api-tests/a.yamlflow.yaml',
      'api-tests/../a.yamlflow.yaml',
      'api-tests/a.yaml',
      'api-tests/.yamlflow.yaml',
      'api-tests\\a.yamlflow.yaml',
    ])
      assert.ok(!isRepoFlowPath(bad), bad);

    const result = await cloud().file('shop', '../.env');
    assert.equal(result.ok, false);
    assert.equal(calls.length, 0);
  });

  void it('opens only https (or the configured Stresseur URL)', () => {
    assert.ok(isOpenableUrl('https://github.com/acme/shop/pull/7'));
    assert.ok(isOpenableUrl(`${baseUrl}/r/shop`));
    assert.ok(!isOpenableUrl('http://evil.test'));
    assert.ok(!isOpenableUrl('file:///etc/passwd'));
    assert.ok(!isOpenableUrl('javascript:alert(1)'));
  });
});

void describe('cloud requests', () => {
  void it('sends the saved token and returns the data', async () => {
    const result = await cloud().repos();
    assert.deepEqual(result, { data: repos, ok: true });
    assert.equal(calls[0], 'GET /api/cli/studio/repos Bearer stsr_test');

    const file = await cloud().file('shop', 'api-tests/a b.yamlflow.yaml');
    assert.ok(file.ok);
    assert.deepEqual(file.data, { path: 'api-tests/a b.yamlflow.yaml', text: 'flows: []' });
  });

  void it('posts a change for review', async () => {
    const result = await cloud().proposeChange('shop', {
      base_sha: 'abc',
      content: 'flows: []\n',
      extra: 'dropped',
      path: 'api-tests/checkout.yamlflow.yaml',
    });
    assert.ok(result.ok);
    const body = JSON.parse(calls[0]?.split('stsr_test ')[1] ?? '{}') as unknown;
    assert.deepEqual(body, { base_sha: 'abc', content: 'flows: []\n', path: 'api-tests/checkout.yamlflow.yaml' });
  });

  void it('sends a title and description when given', async () => {
    await cloud().proposeChange('shop', {
      base_sha: 'abc',
      body: 'Adds a check.',
      content: 'flows: []\n',
      path: 'api-tests/checkout.yamlflow.yaml',
      title: 'Update checkout flow',
    });
    const body = JSON.parse(calls[0]?.split('stsr_test ')[1] ?? '{}') as unknown;
    assert.deepEqual(body, {
      base_sha: 'abc',
      body: 'Adds a check.',
      content: 'flows: []\n',
      path: 'api-tests/checkout.yamlflow.yaml',
      title: 'Update checkout flow',
    });
  });

  void it('is signed out without credentials or for another API URL', async () => {
    fs.rmSync(credentialsFile);
    assert.deepEqual(await cloud().repos(), { error: { kind: 'signed-out' }, ok: false });
    fs.writeFileSync(credentialsFile, JSON.stringify({ api_url: 'https://other.test', token: 'stsr_test' }));
    assert.deepEqual(await cloud().repos(), { error: { kind: 'signed-out' }, ok: false });
    assert.equal(calls.length, 0);
  });

  void it('asks to sign in again on 401 and on 403 insufficient_scope', async () => {
    reply = () => ({ body: { error: 'invalid_token', message: 'Token revoked.' }, status: 401 });
    assert.deepEqual(await cloud().repos(), { error: { kind: 'sign-in-again' }, ok: false });
    reply = () => ({ body: { error: 'insufficient_scope', message: 'Needs flows:write.' }, status: 403 });
    assert.deepEqual(await cloud().flows('shop'), { error: { kind: 'sign-in-again' }, ok: false });
  });

  void it('maps other errors with fix_url and upgrade', async () => {
    reply = () => ({
      body: {
        error: 'plan_limit',
        fix_url: 'https://app.stresseur.test/billing',
        message: 'Free allows 3 cloud runs a day.',
        upgrade: true,
      },
      status: 402,
    });
    assert.deepEqual(await cloud().startRun('shop', { environment: 'env_1', flow: 'checkout' }), {
      error: {
        code: 'plan_limit',
        fixUrl: 'https://app.stresseur.test/billing',
        kind: 'error',
        message: 'Free allows 3 cloud runs a day.',
        upgrade: true,
      },
      ok: false,
    });

    reply = () => ({
      body: { error: 'forbidden_role', fix_url: 'javascript:alert(1)', message: 'Needs Member.' },
      status: 403,
    });
    assert.deepEqual(await cloud().environments('shop'), {
      error: { code: 'forbidden_role', kind: 'error', message: 'Needs Member.' },
      ok: false,
    });

    reply = () => ({ body: 'oops', status: 500 });
    const result = await cloud().run('r1');
    assert.ok(!result.ok && result.error.kind === 'error' && result.error.code === 'http_500');
  });

  void it('serves the last repo and flow lists offline', async () => {
    const online = cloud();
    assert.ok((await online.repos()).ok);
    assert.ok((await online.flows('shop')).ok);
    assert.ok(fs.existsSync(cacheFile));
    assert.ok(!fs.readFileSync(cacheFile, 'utf8').includes('stsr_test'));

    await new Promise<void>((resolve) => void server.close(() => void resolve()));
    server.closeAllConnections();

    const offline = cloud();
    assert.deepEqual(await offline.repos(), { data: repos, offline: true, ok: true });
    const flows = await offline.flows('shop');
    assert.ok(flows.ok && flows.offline);
    // Never cached: offline error.
    assert.deepEqual(await offline.flows('other'), { error: { kind: 'offline' }, ok: false });
    assert.deepEqual(await offline.run('r1'), { error: { kind: 'offline' }, ok: false });

    // Another workspace does not see this cache.
    fs.writeFileSync(
      credentialsFile,
      JSON.stringify({
        api_url: baseUrl,
        token: 'stsr_test',
        user: { email: 'b@b' },
        workspace: { name: 'B', slug: 'b' },
      }),
    );
    assert.deepEqual(await offline.repos(), { error: { kind: 'offline' }, ok: false });
  });
});

void describe('included AI credits', () => {
  void it('streams Claude events and always asks for a stream', async () => {
    const events: unknown[] = [];
    const result = await cloud().aiMessages({ messages: [], stream: false }, (event) => void events.push(event));
    assert.deepEqual(result, { data: null, ok: true });
    assert.deepEqual(
      events.map((_) => (_ as { type: string }).type),
      ['message_start', 'content_block_delta', 'message_stop'],
    );
    assert.equal(calls.length, 1);
    assert.ok(calls[0]?.startsWith('POST /api/cli/studio/ai/messages Bearer stsr_test {"messages":[],"stream":true}'));
  });

  void it('maps a 402 with Upgrade, and refuses a non-object body', async () => {
    reply = (url) =>
      url === '/api/cli/studio/ai/messages'
        ? {
            body: {
              error: 'plan_limit',
              fix_url: 'https://app.stresseur.test/acme/usage',
              message: "This month's $50 of AI credits is used up.",
              upgrade: true,
            },
            status: 402,
          }
        : undefined;
    const result = await cloud().aiMessages({ messages: [] }, () => undefined);
    assert.deepEqual(result, {
      error: {
        code: 'plan_limit',
        fixUrl: 'https://app.stresseur.test/acme/usage',
        kind: 'error',
        message: "This month's $50 of AI credits is used up.",
        upgrade: true,
      },
      ok: false,
    });

    const bad = await cloud().aiMessages('hi', () => undefined);
    assert.ok(!bad.ok && bad.error.kind === 'error' && bad.error.code === 'invalid_request');
    assert.equal(calls.length, 1);
  });

  void it('reads credits', async () => {
    reply = (url) =>
      url === '/api/cli/studio/ai/credits'
        ? {
            body: { included: true, limit_cents: 5000, resets_at: '2026-11-01T00:00:00.000Z', used_cents: 900 },
            status: 200,
          }
        : undefined;
    const result = await cloud().aiCredits();
    assert.ok(result.ok);
    assert.deepEqual(result.data, {
      included: true,
      limit_cents: 5000,
      resets_at: '2026-11-01T00:00:00.000Z',
      used_cents: 900,
    });
  });
});
