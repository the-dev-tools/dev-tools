import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import nodePath from 'node:path';
import { afterEach, beforeEach, describe, it } from 'node:test';
import { createRecorder, type Recorder, type RecordEvent, resolveStresseurBinary } from './record.ts';

/** A stand-in for the stresseur binary, speaking the `--json` NDJSON contract. */
const FAKE_STRESSEUR = String.raw`
const fs = require('node:fs');
const path = require('node:path');
const { spawn } = require('node:child_process');
const args = process.argv.slice(2);
const arg = (name) => args[args.indexOf(name) + 1];
const emit = (event) => process.stdout.write(JSON.stringify(event) + '\n');
const mode = process.env.FAKE_MODE ?? '';

if (args[0] === 'record') {
  const dir = arg('--dir');
  if (mode === 'fail') {
    emit({ type: 'error', message: 'Chrome not found.' });
    process.exit(1);
  }
  // Like the browser that record opens: a child that outlives a plain kill of its parent.
  const browser = spawn(process.execPath, ['-e', 'setInterval(() => {}, 1000)'], { stdio: 'ignore' });
  fs.writeFileSync(process.env.FAKE_PIDS, JSON.stringify({ browser: browser.pid, record: process.pid }));
  console.error('Recording ' + arg('--browser'));
  emit({ type: 'started', proxy_url: 'http://127.0.0.1:9999', collector_url: 'http://127.0.0.1:9998', dir, db: path.join(dir, 'x.db') });
  let calls = 0;
  const timer = setInterval(() => {
    calls += 1;
    emit({ type: 'progress', calls, sessions: 1, endpoints: calls });
  }, 20);
  if (mode !== 'ignore-stdin') {
    process.stdin.on('end', () => {
      clearInterval(timer);
      browser.kill();
      fs.writeFileSync(path.join(dir, 'traffic.json'), '[]');
      emit({ type: 'stopped', calls, sessions: 1, endpoints: calls });
      process.exit(0);
    });
  }
  process.stdin.resume();
} else if (args[0] === 'generate') {
  const out = arg('--out');
  if (!fs.existsSync(path.join(arg('--dir'), 'traffic.json'))) {
    emit({ type: 'error', message: 'No traffic recorded.' });
    process.exit(1);
  }
  if (!args.includes('--no-verify') || !args.includes('--no-upload')) process.exit(2);
  fs.mkdirSync(path.join(out, 'shop'), { recursive: true });
  const file = path.join(out, 'shop', 'checkout.yamlflow.yaml');
  fs.writeFileSync(file, 'flows:\n  - name: checkout\n');
  emit({ type: 'generated', files: [{ path: file, flows: ['checkout'] }, { path: '/etc/hosts', flows: [] }] });
}
`;

let dir: string;
let tmpRoot: string;
let bin: string;
let pidsFile: string;
let events: RecordEvent[];
let recorder: Recorder | undefined;

beforeEach(() => {
  dir = fs.realpathSync(fs.mkdtempSync(nodePath.join(os.tmpdir(), 'studio-record-')));
  tmpRoot = nodePath.join(dir, 'tmp');
  fs.mkdirSync(tmpRoot);
  bin = nodePath.join(dir, 'stresseur');
  fs.writeFileSync(bin, `#!${process.execPath}\n${FAKE_STRESSEUR}`, { mode: 0o755 });
  pidsFile = nodePath.join(dir, 'pids.json');
  process.env['FAKE_PIDS'] = pidsFile;
  delete process.env['FAKE_MODE'];
  events = [];
});

afterEach(() => {
  recorder?.dispose();
  recorder = undefined;
  fs.rmSync(dir, { force: true, recursive: true });
  delete process.env['FAKE_PIDS'];
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

const pids = () => JSON.parse(fs.readFileSync(pidsFile, 'utf8')) as { browser: number; record: number };

void describe('record', { skip: process.platform === 'win32' && 'uses a shebang script' }, () => {
  void it('forwards progress, stops on stdin close and returns the generated files', async () => {
    const api = create();
    assert.deepEqual(await api.start('http://localhost:3000'), { data: null, ok: true });
    assert.deepEqual(events[0], { proxy_url: 'http://127.0.0.1:9999', type: 'started' });

    await until(() => events.filter((_) => _.type === 'progress').length >= 2);
    assert.deepEqual(
      events.find((_) => _.type === 'progress'),
      { calls: 1, endpoints: 1, sessions: 1, type: 'progress' },
    );

    const result = await api.stop();
    assert.deepEqual(result, {
      data: [{ flows: ['checkout'], path: 'shop/checkout.yamlflow.yaml', text: 'flows:\n  - name: checkout\n' }],
      ok: true,
    });
    const types = events.map((_) => _.type);
    assert.equal(types.at(-2), 'stopped');
    const stopped = events.at(-2);
    assert.ok(stopped?.type === 'stopped' && stopped.endpoints === stopped.calls);
    assert.equal(types.at(-1), 'generating');
    assert.deepEqual(fs.readdirSync(tmpRoot), []);
    assert.equal(api.isRecording(), false);
  });

  void it('records one at a time and rejects bad URLs', async () => {
    const api = create();
    assert.equal((await api.start('file:///etc/passwd')).ok, false);
    assert.equal((await api.start('not a url')).ok, false);
    assert.ok((await api.start('http://localhost:3000')).ok);
    const second = await api.start('http://localhost:4000');
    assert.ok(!second.ok && second.message.includes('already'));
    await api.cancel();
  });

  void it('reports a recording that fails to start', async () => {
    process.env['FAKE_MODE'] = 'fail';
    const api = create();
    assert.deepEqual(await api.start('http://localhost:3000'), { message: 'Chrome not found.', ok: false });
    assert.deepEqual(fs.readdirSync(tmpRoot), []);
    assert.equal(api.isRecording(), false);
  });

  void it('reports a missing binary', async () => {
    recorder = createRecorder({ binary: () => undefined, onEvent: () => undefined, tmpRoot });
    assert.equal((await recorder.start('http://localhost:3000')).ok, false);
  });

  void it('kills a recording that ignores stdin after the stop timeout', async () => {
    process.env['FAKE_MODE'] = 'ignore-stdin';
    const api = create({ stopTimeoutMs: 300 });
    assert.ok((await api.start('http://localhost:3000')).ok);
    const { browser, record } = pids();
    const result = await api.stop();
    assert.deepEqual(result, { message: 'No traffic recorded.', ok: false });
    await until(() => !isAlive(record) && !isAlive(browser));
    assert.deepEqual(fs.readdirSync(tmpRoot), []);
  });

  void it('cancels without generating', async () => {
    const api = create();
    assert.ok((await api.start('http://localhost:3000')).ok);
    const { browser, record } = pids();
    await api.cancel();
    await until(() => !isAlive(record) && !isAlive(browser));
    assert.ok(!events.some((_) => _.type === 'generating'));
    assert.deepEqual(fs.readdirSync(tmpRoot), []);
    assert.equal((await api.stop()).ok, false);
  });

  void it('kills the whole process tree on quit', async () => {
    const api = create();
    assert.ok((await api.start('http://localhost:3000')).ok);
    const { browser, record } = pids();
    assert.ok(isAlive(record) && isAlive(browser));
    api.dispose();
    await until(() => !isAlive(record) && !isAlive(browser));
    assert.deepEqual(fs.readdirSync(tmpRoot), []);
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
