import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import nodePath from 'node:path';
import { afterEach, beforeEach, describe, it } from 'node:test';
import {
  createRecorder,
  type FinishedEvent,
  parseInitEvent,
  readDotEnv,
  type Recorder,
  type RecordEvent,
  resolveStresseurBinary,
} from './record.ts';
import { writeFakeStresseur } from './test-fake-stresseur.ts';

let dir: string;
let tmpRoot: string;
let bin: string;
let pidsFile: string;
let argsFile: string;
let events: RecordEvent[];
let recorder: Recorder | undefined;

beforeEach(() => {
  dir = fs.realpathSync(fs.mkdtempSync(nodePath.join(os.tmpdir(), 'studio-record-')));
  tmpRoot = nodePath.join(dir, 'tmp');
  fs.mkdirSync(tmpRoot);
  bin = writeFakeStresseur(dir);
  pidsFile = nodePath.join(dir, 'pids.json');
  argsFile = nodePath.join(dir, 'args.json');
  process.env['FAKE_PIDS'] = pidsFile;
  process.env['FAKE_ARGS'] = argsFile;
  delete process.env['FAKE_MODE'];
  events = [];
});

afterEach(() => {
  recorder?.dispose();
  recorder = undefined;
  fs.rmSync(dir, { force: true, recursive: true });
  delete process.env['FAKE_PIDS'];
  delete process.env['FAKE_ARGS'];
  delete process.env['FAKE_MODE'];
});

const create = (options: { stopTimeoutMs?: number } = {}) =>
  (recorder = createRecorder({ binary: () => bin, onEvent: (_) => void events.push(_), tmpRoot, ...options }));

const until = async (check: () => boolean, ms = 5_000) => {
  const deadline = Date.now() + ms;
  while (!check()) {
    if (Date.now() > deadline) throw new Error('Timed out');
    await new Promise((resolve) => setTimeout(resolve, 10));
  }
};

const isAlive = (pid: number) => {
  try {
    process.kill(pid, 0);
    return true;
  } catch {
    return false;
  }
};

const pids = async () => {
  await until(() => events.some((_) => _.type === 'recording'));
  return JSON.parse(fs.readFileSync(pidsFile, 'utf8')) as { browser: number; init: number };
};
const spawned = () => JSON.parse(fs.readFileSync(argsFile, 'utf8')) as { args: string[]; cwd: string };
const finished = async () => {
  await until(() => events.some((_) => _.type === 'finished'));
  const event = events.find((_): _ is FinishedEvent => _.type === 'finished');
  if (!event) throw new Error('No finished event');
  return event;
};

void describe('init', { skip: process.platform === 'win32' && 'uses a shebang script' }, () => {
  void it('green path: streams every event, stops on "done", imports from a temporary folder', async () => {
    const api = create();
    const started = await api.start({ url: 'http://localhost:3000' });
    assert.ok(started.ok);
    const { args } = spawned();
    assert.deepEqual(args, [
      'init',
      '--json',
      '--url',
      'http://localhost:3000/',
      '--dir',
      started.data.dir,
      '--stop-on-stdin-eof',
      '--no-ci',
      '--no-pr',
    ]);
    assert.ok(!args.includes('--no-verify'));
    assert.ok(started.data.dir.startsWith(tmpRoot));

    await until(() => events.filter((_) => _.type === 'progress').length >= 2);
    assert.deepEqual(events[0], { source: 'flag', type: 'detected', url: 'http://localhost:3000/' });
    assert.deepEqual(events[1], { browser: 'chrome', proxy_url: 'http://127.0.0.1:9999', type: 'recording' });
    assert.deepEqual(
      events.find((_) => _.type === 'progress'),
      { calls: 1, endpoints: 1, type: 'progress' },
    );

    assert.deepEqual(api.stop(), { data: null, ok: true });
    const end = await finished();
    const types = events.map((_) => _.type).filter((_) => _ !== 'progress');
    assert.deepEqual(types, ['detected', 'recording', 'recorded', 'generated', 'verified', 'verified', 'done', 'finished']);
    assert.deepEqual(
      events.filter((_) => _.type === 'verified').map((_) => [_.run, _.passed, _.total]),
      [
        [1, 4, 4],
        [2, 4, 4],
      ],
    );
    assert.equal(end.code, 0);
    assert.equal(end.temporary, true);
    assert.deepEqual(end.files, [{ flows: ['checkout'], path: 'shop/checkout.yamlflow.yaml', text: 'flows:\n  - name: checkout\n' }]);
    assert.deepEqual(end.env, { EMAIL: 'a@b.co', PASSWORD: 's3cret' });
    assert.deepEqual(fs.readdirSync(tmpRoot), []);
    assert.equal(api.isRunning(), false);
  });

  void it('failure path: per-step failures, fix_prompt, and the folder is kept for the agent', async () => {
    process.env['FAKE_MODE'] = 'verify-fail';
    const api = create();
    const started = await api.start({ url: 'http://localhost:3000' });
    assert.ok(started.ok);
    api.stop();
    const end = await finished();
    const verified = events.filter((_) => _.type === 'verified');
    assert.deepEqual(verified[1], {
      failures: [{ flow: 'checkout', hint: 'Seed a product.', reason: 'expected 200, got 500', step: 'pay' }],
      passed: 3,
      run: 2,
      total: 4,
      type: 'verified',
    });
    assert.deepEqual(
      events.find((_) => _.type === 'fix_prompt'),
      { prompt: 'Fix api-tests/shop/checkout.yamlflow.yaml: pay fails (500).', type: 'fix_prompt' },
    );
    assert.deepEqual(events.find((_) => _.type === 'done'), { green: false, type: 'done' });
    assert.ok(!events.some((_) => _.type === 'error'), 'exit code 1 is a failed verification, not an error');
    assert.equal(end.code, 1);
    assert.equal(end.files.length, 1);
    assert.ok(fs.existsSync(nodePath.join(started.data.dir, 'api-tests', 'shop', 'checkout.yamlflow.yaml')));
  });

  void it('demo: --demo without a URL, the sample app is detected and started', async () => {
    const api = create();
    const started = await api.start({ demo: true });
    assert.ok(started.ok);
    const { args } = spawned();
    assert.ok(args.includes('--demo') && !args.includes('--url'));
    await until(() => events.some((_) => _.type === 'recording'));
    assert.deepEqual(
      events.slice(0, 3).map((_) => _.type),
      ['detected', 'starting_app', 'app_ready'],
    );
    api.stop();
    const end = await finished();
    assert.equal(end.code, 0);
    assert.deepEqual(
      events.find((_) => _.type === 'done'),
      { green: true, type: 'done' },
    );
    assert.deepEqual(fs.readdirSync(tmpRoot), []);
  });

  void it('writes into a repo folder and leaves it there', async () => {
    const repo = nodePath.join(dir, 'repo');
    fs.mkdirSync(repo);
    const api = create();
    const started = await api.start({ dir: repo, url: 'http://localhost:5173' });
    assert.ok(started.ok);
    assert.equal(spawned().cwd, repo);
    await until(() => events.some((_) => _.type === 'recording'));
    api.stop();
    const end = await finished();
    assert.equal(end.temporary, false);
    assert.deepEqual(end.env, {}, 'values stay in the repo .env');
    assert.ok(fs.existsSync(nodePath.join(repo, 'api-tests', 'shop', 'checkout.yamlflow.yaml')));
  });

  void it('rejects bad input and runs one at a time', async () => {
    const api = create();
    assert.equal((await api.start({ url: 'file:///etc/passwd' })).ok, false);
    assert.equal((await api.start({ url: 'not a url' })).ok, false);
    assert.equal((await api.start({ dir: 'relative', url: 'http://localhost:3000' })).ok, false);
    assert.equal((await api.start({ dir: nodePath.join(dir, 'missing'), url: 'http://localhost:3000' })).ok, false);
    assert.ok((await api.start({ url: 'http://localhost:3000' })).ok);
    const second = await api.start({ url: 'http://localhost:4000' });
    assert.ok(!second.ok && second.message.includes('already'));
    await api.cancel();
  });

  void it('reports an init that fails to start', async () => {
    process.env['FAKE_MODE'] = 'fail-start';
    const api = create();
    const result = await api.start({ url: 'http://localhost:3000' });
    assert.deepEqual(result, { message: 'No browser found. Install Chrome.', ok: false });
    assert.equal(api.isRunning(), false);
    assert.deepEqual(fs.readdirSync(tmpRoot), []);
  });

  void it('reports a missing binary', async () => {
    recorder = createRecorder({ binary: () => undefined, onEvent: () => undefined, tmpRoot });
    assert.equal((await recorder.start({ url: 'http://localhost:3000' })).ok, false);
  });

  void it('kills a recording that ignores "done" after the stop timeout', async () => {
    process.env['FAKE_MODE'] = 'ignore-stdin';
    const api = create({ stopTimeoutMs: 300 });
    assert.ok((await api.start({ url: 'http://localhost:3000' })).ok);
    const { browser, init } = await pids();
    api.stop();
    await until(() => !isAlive(init) && !isAlive(browser));
    await finished();
    assert.ok(events.some((_) => _.type === 'error' && _.message === 'The recording did not stop.'));
    assert.deepEqual(fs.readdirSync(tmpRoot), []);
  });

  void it('cancels: kills the tree, removes the folder, no finished event', async () => {
    const api = create();
    assert.ok((await api.start({ url: 'http://localhost:3000' })).ok);
    const { browser, init } = await pids();
    await api.cancel();
    await until(() => !isAlive(init) && !isAlive(browser));
    assert.deepEqual(fs.readdirSync(tmpRoot), []);
    assert.ok(!events.some((_) => _.type === 'finished'));
    assert.equal(api.stop().ok, false);
  });

  void it('kills the whole process tree on quit', async () => {
    const api = create();
    assert.ok((await api.start({ url: 'http://localhost:3000' })).ok);
    const { browser, init } = await pids();
    assert.ok(isAlive(init) && isAlive(browser));
    api.dispose();
    await until(() => !isAlive(init) && !isAlive(browser));
    assert.deepEqual(fs.readdirSync(tmpRoot), []);
  });
});

void describe('init events', () => {
  void it('keeps only contract fields and drops unknown events', () => {
    assert.deepEqual(parseInitEvent({ extra: 1, green: true, type: 'done' }), { green: true, type: 'done' });
    assert.deepEqual(parseInitEvent({ type: 'pr', url: 'https://github.com/a/b/pull/1' }), {
      type: 'pr',
      url: 'https://github.com/a/b/pull/1',
    });
    assert.deepEqual(parseInitEvent({ path: '/r/.github/workflows/stresseur.yml', type: 'ci', warnings: ['DB', 3] }), {
      path: '/r/.github/workflows/stresseur.yml',
      type: 'ci',
      warnings: ['DB'],
    });
    assert.equal(parseInitEvent({ type: 'nope' }), undefined);
    assert.equal(parseInitEvent({ type: 'detected' }), undefined);
    assert.equal(parseInitEvent('x'), undefined);
  });

  void it('reads only the named .env values', () => {
    assert.deepEqual(readDotEnv('A=1\n# B=2\nexport B="two"\nC=3\nA=again\n', ['A', 'B']), { A: '1', B: 'two' });
  });
});

void describe('stresseur binary', () => {
  void it('uses the bundled binary when packaged', () => {
    fs.writeFileSync(nodePath.join(dir, 'stresseur.exe'), '');
    assert.equal(resolveStresseurBinary({ isPackaged: true, platform: 'linux', resourcesPath: dir }), bin);
    assert.equal(
      resolveStresseurBinary({ isPackaged: true, platform: 'win32', resourcesPath: dir }),
      nodePath.join(dir, 'stresseur.exe'),
    );
    assert.equal(resolveStresseurBinary({ isPackaged: true, platform: 'linux', resourcesPath: tmpRoot }), undefined);
  });

  void it('uses STRESSEUR_BIN, then PATH, in dev', () => {
    assert.equal(resolveStresseurBinary({ env: { STRESSEUR_BIN: bin }, isPackaged: false, platform: 'linux' }), bin);
    assert.equal(
      resolveStresseurBinary({
        env: { PATH: [tmpRoot, dir].join(nodePath.delimiter) },
        isPackaged: false,
        platform: 'linux',
      }),
      bin,
    );
    assert.equal(resolveStresseurBinary({ env: { PATH: tmpRoot }, isPackaged: false, platform: 'linux' }), undefined);
  });
});
