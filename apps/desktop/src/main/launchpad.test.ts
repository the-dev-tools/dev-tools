import type { AddressInfo } from 'node:net';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import http from 'node:http';
import os from 'node:os';
import nodePath from 'node:path';
import { afterEach, beforeEach, describe, it } from 'node:test';
import {
  addCiCheck,
  agentLink,
  agentsFromPaths,
  createDirAllowlist,
  detectDevServers,
  installSkill,
  openPr,
  progressSteps,
  readRepoFlows,
  type RepoProgress,
  scanRepo,
  watchRepo,
} from './launchpad.ts';
import { writeFakeStresseur } from './test-fake-stresseur.ts';

let dir: string;
let repo: string;
let bin: string;

beforeEach(() => {
  dir = fs.realpathSync(fs.mkdtempSync(nodePath.join(os.tmpdir(), 'studio-launchpad-')));
  repo = nodePath.join(dir, 'repo');
  fs.mkdirSync(repo);
  bin = writeFakeStresseur(dir);
  delete process.env['FAKE_MODE'];
  delete process.env['FAKE_ARGS'];
  delete process.env['FAKE_PIDS'];
});

afterEach(() => {
  fs.rmSync(dir, { force: true, recursive: true });
  delete process.env['FAKE_MODE'];
});

const write = (file: string, text: string) => {
  const full = nodePath.join(repo, file);
  fs.mkdirSync(nodePath.dirname(full), { recursive: true });
  fs.writeFileSync(full, text);
};

const lastVerify = (flows: { passed: number; status: string; total: number }[]) =>
  JSON.stringify({ baseUrl: 'http://localhost:3000', flows, verifiedAt: '2026-10-11T10:00:00.000Z' });

const until = async (check: () => boolean, ms = 5_000) => {
  const deadline = Date.now() + ms;
  while (!check()) {
    if (Date.now() > deadline) throw new Error('Timed out');
    await new Promise((resolve) => setTimeout(resolve, 10));
  }
};

void describe('repo progress', () => {
  void it('scans nothing in an empty repo', async () => {
    const progress = await scanRepo(repo);
    assert.deepEqual(progress, { flowFiles: [] });
    assert.deepEqual(progressSteps(progress), {
      checks: 'pending',
      generated: 'pending',
      pr: 'pending',
      recording: 'pending',
    });
  });

  void it('recording is active while recording.db changes, done once it goes quiet', async () => {
    write('.stresseur/recording.db', 'x');
    const now = Date.now();
    assert.equal(progressSteps(await scanRepo(repo, now)).recording, 'active');
    assert.equal(progressSteps(await scanRepo(repo, now + 60_000)).recording, 'done');
  });

  void it('generated flows, then checks: failed while red (no spinner), done when every flow passed', async () => {
    write('api-tests/shop/checkout.yamlflow.yaml', 'flows: []\n');
    write('api-tests/login.yamlflow.yaml', 'flows: []\n');
    write('api-tests/node_modules/x.yaml', '');
    write(
      '.stresseur/last-verify.json',
      lastVerify([
        { passed: 4, status: 'passed', total: 4 },
        { passed: 1, status: 'failed', total: 3 },
      ]),
    );
    let progress = await scanRepo(repo, Date.now() + 60_000);
    assert.deepEqual(progress.flowFiles, ['login.yamlflow.yaml', 'shop/checkout.yamlflow.yaml']);
    assert.deepEqual(progress.checks, {
      flows: 2,
      green: false,
      passed: 5,
      passedFlows: 1,
      total: 7,
      verifiedAt: '2026-10-11T10:00:00.000Z',
    });
    assert.deepEqual(progressSteps(progress), {
      checks: 'failed',
      generated: 'done',
      pr: 'pending',
      recording: 'done',
    });
    // A PR opened on red checks stays failed, not in progress.
    assert.equal(progressSteps({ ...progress, pr: { url: 'https://github.com/a/b/pull/1' } }).checks, 'failed');

    write(
      '.stresseur/last-verify.json',
      lastVerify([
        { passed: 4, status: 'passed', total: 4 },
        { passed: 3, status: 'passed', total: 3 },
      ]),
    );
    progress = await scanRepo(repo);
    assert.equal(progress.checks?.green, true);
    const withPr: RepoProgress = { ...progress, pr: { url: 'https://github.com/a/b/pull/7' } };
    assert.deepEqual(progressSteps(withPr), { checks: 'done', generated: 'done', pr: 'done', recording: 'done' });
  });

  void it('ignores a broken last-verify.json', async () => {
    write('.stresseur/last-verify.json', '{');
    assert.equal((await scanRepo(repo)).checks, undefined);
  });

  void it('watches the repo: reports each change, asks for the PR once flows exist', async () => {
    const seen: RepoProgress[] = [];
    let prLookups = 0;
    const stop = watchRepo(repo, {
      debounceMs: 20,
      findPr: () => {
        prLookups += 1;
        return Promise.resolve({ number: 7, state: 'OPEN', url: 'https://github.com/a/b/pull/7' });
      },
      onProgress: (_) => void seen.push(_),
      pollMs: 200,
      prEveryMs: 0,
    });
    try {
      await until(() => seen.length === 1);
      assert.equal(prLookups, 0);

      write('.stresseur/recording.db', 'x');
      await until(() => seen.some((_) => _.recording?.active));

      write('api-tests/checkout.yamlflow.yaml', 'flows: []\n');
      await until(() => seen.some((_) => _.flowFiles.length === 1 && _.pr !== undefined));

      write('.stresseur/last-verify.json', lastVerify([{ passed: 2, status: 'passed', total: 2 }]));
      await until(() => seen.some((_) => _.checks?.green === true));
      assert.equal(prLookups, 1, 'a found PR is not looked up again');
      const last = seen.at(-1);
      assert.ok(last);
      assert.deepEqual(progressSteps({ ...last, recording: { active: false, updatedAt: 0 } }), {
        checks: 'done',
        generated: 'done',
        pr: 'done',
        recording: 'done',
      });
    } finally {
      stop();
    }
    const count = seen.length;
    write('api-tests/more.yamlflow.yaml', '');
    await new Promise((resolve) => setTimeout(resolve, 300));
    assert.equal(seen.length, count, 'nothing after stop');
  });
});

void describe('coding agent', { skip: process.platform === 'win32' && 'uses a shebang script' }, () => {
  void it('installs the skill and names the agents that got it', async () => {
    const result = await installSkill(bin, repo);
    assert.ok(result.ok);
    assert.deepEqual(result.data.agents, ['Claude Code', 'Codex', 'Cursor']);
    assert.equal(result.data.files.length, 3);
  });

  void it('reports a refused install', async () => {
    process.env['FAKE_MODE'] = 'skill-conflict';
    const result = await installSkill(bin, repo);
    assert.ok(!result.ok && result.message.includes('Refusing to overwrite'));
  });

  void it('maps written files to agents', () => {
    assert.deepEqual(agentsFromPaths(['/r/AGENTS.md']), ['Codex']);
    assert.deepEqual(agentsFromPaths(['/r/.cursor/rules/stresseur.mdc', '/r/.claude/skills/stresseur/SKILL.md']), [
      'Claude Code',
      'Cursor',
    ]);
  });

  void it('builds the documented deep links', () => {
    assert.equal(
      agentLink('claude', '/Users/me/shop', 'Fix it: a & b\nnow'),
      'claude-cli://open?cwd=%2FUsers%2Fme%2Fshop&q=Fix%20it%3A%20a%20%26%20b%0Anow',
    );
    assert.equal(agentLink('claude', '../x', 'hi'), 'claude-cli://open?q=hi');
    assert.equal(agentLink('claude', '/r', 'x'.repeat(5_001)), undefined);
    assert.equal(agentLink('cursor', '/r', 'a b'), 'cursor://anysphere.cursor-deeplink/prompt?text=a%20b');
    assert.equal(agentLink('cursor', '/r', 'x'.repeat(10_000)), undefined);
    assert.equal(agentLink('vim', '/r', 'hi'), undefined);
    assert.equal(agentLink('claude', '/r', ''), undefined);
  });
});

void describe('CI check and PR', { skip: process.platform === 'win32' && 'uses a shebang script' }, () => {
  void it('adds the CI check with init-ci', async () => {
    const result = await addCiCheck(bin, repo);
    assert.ok(result.ok);
    assert.ok(fs.existsSync(nodePath.join(repo, '.github/workflows/stresseur.yml')));
  });

  void it('without gh, gives the commands for the paths that exist', async () => {
    write('api-tests/a.yamlflow.yaml', '');
    write('.env.example', 'EMAIL=\n');
    const result = await openPr(repo, { gh: nodePath.join(dir, 'no-gh') });
    assert.ok(result.ok);
    const commands = result.data.commands ?? [];
    assert.match(commands[0] ?? '', /^git checkout -b stresseur\/api-tests-\d{4}-\d{2}-\d{2}$/);
    assert.equal(commands[1], 'git add api-tests .env.example');
    assert.equal(commands.at(-1), 'gh pr create --fill');
  });

  void it('needs generated flows', async () => {
    const result = await openPr(repo, { gh: nodePath.join(dir, 'no-gh') });
    assert.ok(!result.ok);
  });
});

void describe('dev servers', () => {
  void it('finds the ports that answer and reads the page title', async () => {
    const server = http.createServer((_req, res) => {
      res.writeHead(200, { 'content-type': 'text/html' });
      res.end('<html><head><title> Shop \n dev </title></head></html>');
    });
    await new Promise<void>((resolve) => void server.listen(0, 'localhost', resolve));
    const { port } = server.address() as AddressInfo;
    const closed = http.createServer();
    await new Promise<void>((resolve) => void closed.listen(0, 'localhost', resolve));
    const free = (closed.address() as AddressInfo).port;
    await new Promise((resolve) => closed.close(resolve));
    try {
      assert.deepEqual(await detectDevServers([free, port]), [
        { port, status: 200, title: 'Shop dev', url: `http://localhost:${String(port)}` },
      ]);
    } finally {
      server.closeAllConnections();
      await new Promise((resolve) => server.close(resolve));
    }
  });

  void it('skips macOS AirPlay', async () => {
    const servers = ['AirTunes/870.14.1', 'ControlCenter'].map((name) =>
      http.createServer((_req, res) => {
        res.writeHead(403, { server: name });
        res.end();
      }),
    );
    const ports = await Promise.all(
      servers.map(async (server) => {
        await new Promise<void>((resolve) => void server.listen(0, 'localhost', resolve));
        return (server.address() as AddressInfo).port;
      }),
    );
    try {
      assert.deepEqual(await detectDevServers(ports), []);
    } finally {
      for (const server of servers) {
        server.closeAllConnections();
        await new Promise((resolve) => server.close(resolve));
      }
    }
  });
});

void describe('picked folders', () => {
  void it('allows only picked folders', () => {
    const dirs = createDirAllowlist();
    dirs.add('/r/app');
    assert.ok(dirs.has('/r/app/'));
    assert.ok(!dirs.has('/r'));
    assert.ok(!dirs.has(3));
  });
});

void describe('readRepoFlows', () => {
  const write = (rel: string, text: string) => {
    fs.mkdirSync(nodePath.dirname(nodePath.join(repo, rel)), { recursive: true });
    fs.writeFileSync(nodePath.join(repo, rel), text);
  };

  void it('reads api-tests flows with their names, and only the .env values they use', async () => {
    write(
      'api-tests/shop/checkout.yamlflow.yaml',
      'flows:\n  - name: checkout\n    steps:\n      - request:\n          url: "{{ #env:APP_URL }}/cart"\n  - name: smoke\n',
    );
    write('api-tests/login.yamlflow.yaml', 'flows:\n  - name: login\n    password: "{{#env:SHOP_PASSWORD}}"\n');
    write('.env', 'APP_URL=http://localhost:3000\nSHOP_PASSWORD="hunter2"\nUNRELATED=secret\n');

    const result = await readRepoFlows(repo);
    assert.deepEqual(
      result.files.map((_) => ({ flows: _.flows, path: _.path })),
      [
        { flows: ['login'], path: 'login.yamlflow.yaml' },
        { flows: ['checkout', 'smoke'], path: 'shop/checkout.yamlflow.yaml' },
      ],
    );
    assert.match(result.files[1]?.text ?? '', /#env:APP_URL/);
    assert.deepEqual(result.env, { APP_URL: 'http://localhost:3000', SHOP_PASSWORD: 'hunter2' });
  });

  void it('returns no files and no env for a repo without api-tests', async () => {
    assert.deepEqual(await readRepoFlows(repo), { env: {}, files: [] });
  });
});
