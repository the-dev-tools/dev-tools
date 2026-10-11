import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import nodePath from 'node:path';
import { afterEach, beforeEach, describe, it } from 'node:test';
import { createDemoApp, createDemoFlows, type DemoApp, demoRunEnv } from './demo.ts';
import { writeFakeStresseur } from './test-fake-stresseur.ts';

let dir: string;
let bin: string;
let pidFile: string;
let app: DemoApp | undefined;

beforeEach(() => {
  dir = fs.realpathSync(fs.mkdtempSync(nodePath.join(os.tmpdir(), 'studio-demo-')));
  bin = writeFakeStresseur(dir);
  pidFile = nodePath.join(dir, 'demo-pids');
  process.env['FAKE_DEMO_PID'] = pidFile;
  delete process.env['FAKE_MODE'];
});

afterEach(() => {
  app?.dispose();
  app = undefined;
  fs.rmSync(dir, { force: true, recursive: true });
  delete process.env['FAKE_DEMO_PID'];
  delete process.env['FAKE_MODE'];
});

const create = (options: { binary?: () => string | undefined; startTimeoutMs?: number } = {}) =>
  (app = createDemoApp({ binary: () => bin, ...options }));

const demoPids = () =>
  fs.existsSync(pidFile)
    ? fs
        .readFileSync(pidFile, 'utf8')
        .split('\n')
        .filter(Boolean)
        .map((_) => Number(_))
    : [];

const isAlive = (pid: number) => {
  try {
    process.kill(pid, 0);
    return true;
  } catch {
    return false;
  }
};

const until = async (check: () => boolean, ms = 5_000) => {
  const deadline = Date.now() + ms;
  while (!check()) {
    if (Date.now() > deadline) throw new Error('Timed out');
    await new Promise((resolve) => setTimeout(resolve, 10));
  }
};

void describe('sample shop', { skip: process.platform === 'win32' && 'uses a shebang script' }, () => {
  void it('starts `demo --json` once, returns the app_ready URL, and stops on dispose', async () => {
    const demo = create();
    const [first, second] = await Promise.all([demo.ensure(), demo.ensure()]);
    assert.deepEqual(first, { data: { url: 'http://localhost:4599' }, ok: true });
    assert.deepEqual(second, first);
    assert.deepEqual(await demo.ensure(), first);
    assert.equal(demoPids().length, 1);
    const [pid] = demoPids();
    assert.ok(pid && isAlive(pid));

    demo.dispose();
    assert.equal(demo.isRunning(), false);
    await until(() => !isAlive(pid));
  });

  void it('starts it again after it exits', async () => {
    const demo = create();
    assert.ok((await demo.ensure()).ok);
    const [pid] = demoPids();
    assert.ok(pid);
    process.kill(pid, 'SIGKILL');
    await until(() => !demo.isRunning());
    assert.deepEqual(await demo.ensure(), { data: { url: 'http://localhost:4599' }, ok: true });
    assert.equal(demoPids().length, 2);
  });

  void it('reports why it did not start, and retries on the next call', async () => {
    process.env['FAKE_MODE'] = 'demo-fail';
    const demo = create();
    assert.deepEqual(await demo.ensure(), { message: 'listen tcp :4599: address already in use', ok: false });
    assert.equal(demo.isRunning(), false);
    delete process.env['FAKE_MODE'];
    assert.ok((await demo.ensure()).ok);
  });

  void it('gives up and kills it when app_ready never comes', async () => {
    process.env['FAKE_MODE'] = 'demo-hang';
    const demo = create({ startTimeoutMs: 1_500 });
    assert.deepEqual(await demo.ensure(), { message: 'The sample shop did not start.', ok: false });
    const [pid] = demoPids();
    assert.ok(pid);
    await until(() => !isAlive(pid));
  });

  void it('fails without a binary', async () => {
    const demo = create({ binary: () => undefined });
    assert.deepEqual(await demo.ensure(), { message: 'The stresseur binary was not found.', ok: false });
  });
});

void describe('demo flows', { skip: process.platform === 'win32' && 'uses a shebang script' }, () => {
  void it('marks imported flows with their .env and survives a restart', async () => {
    const storeFile = nodePath.join(dir, 'state', 'demo-flows.json');
    const flows = createDemoFlows({ storeFile });
    assert.equal(await flows.envFor('A'), undefined);

    assert.deepEqual(await flows.mark(['A', 'B'], { 'BAD NAME': 'x', EMAIL: 'demo@stresseur.dev', PASSWORD: 'pw' }), {
      data: null,
      ok: true,
    });
    assert.deepEqual(await flows.mark(['B', 'C'], { PASSWORD: 'pw2' }), { data: null, ok: true });
    assert.deepEqual((await flows.mark('A', {})).ok, false);

    const again = createDemoFlows({ storeFile });
    assert.deepEqual(await again.envFor('A'), { EMAIL: 'demo@stresseur.dev', PASSWORD: 'pw2' });
    assert.deepEqual(await again.envFor('C'), { EMAIL: 'demo@stresseur.dev', PASSWORD: 'pw2' });
    assert.equal(await again.envFor('D'), undefined);
    assert.equal(await again.envFor(42), undefined);
    assert.deepEqual(JSON.parse(fs.readFileSync(storeFile, 'utf8')), {
      env: { EMAIL: 'demo@stresseur.dev', PASSWORD: 'pw2' },
      flowIds: ['A', 'B', 'C'],
    });
  });

  void it('run env: the sample shop URL as APP_URL for a demo flow, nothing for another', async () => {
    const flows = createDemoFlows({ storeFile: nodePath.join(dir, 'demo-flows.json') });
    await flows.mark(['DEMO'], { APP_URL: 'http://localhost:51234', PASSWORD: 'pw' });
    const demo = create();

    assert.deepEqual(await demoRunEnv(flows, demo, 'MINE'), { data: null, ok: true });
    assert.equal(demo.isRunning(), false);

    assert.deepEqual(await demoRunEnv(flows, demo, 'DEMO'), {
      data: { APP_URL: 'http://localhost:4599', PASSWORD: 'pw' },
      ok: true,
    });
    assert.equal(demo.isRunning(), true);
  });

  void it('run env: a sample shop that does not start fails the run', async () => {
    process.env['FAKE_MODE'] = 'demo-fail';
    const flows = createDemoFlows({ storeFile: nodePath.join(dir, 'demo-flows.json') });
    await flows.mark(['DEMO'], {});
    assert.deepEqual(await demoRunEnv(flows, create(), 'DEMO'), {
      message: 'listen tcp :4599: address already in use',
      ok: false,
    });
  });
});
