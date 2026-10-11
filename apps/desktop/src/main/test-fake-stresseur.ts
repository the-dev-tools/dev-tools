import fs from 'node:fs';
import nodePath from 'node:path';

/** A stand-in for the stresseur binary, speaking the `init --json` contract. */
const FAKE_STRESSEUR = String.raw`
const fs = require('node:fs');
const path = require('node:path');
const { spawn } = require('node:child_process');
const args = process.argv.slice(2);
const arg = (name) => args[args.indexOf(name) + 1];
const emit = (event) => process.stdout.write(JSON.stringify(event) + '\n');
const mode = process.env.FAKE_MODE ?? '';
if (process.env.FAKE_ARGS) fs.writeFileSync(process.env.FAKE_ARGS, JSON.stringify({ args, cwd: process.cwd() }));

if (args[0] === 'init') {
  const dir = arg('--dir');
  const demo = args.includes('--demo');
  if (!args.includes('--json') || !args.includes('--stop-on-stdin-eof') || !args.includes('--no-pr')) process.exit(2);
  if (mode === 'fail-start') {
    emit({ type: 'error', message: 'No browser found.', hint: 'Install Chrome.' });
    process.exit(2);
  }
  const url = demo ? 'http://localhost:4100' : arg('--url');
  emit({ type: 'detected', url, source: demo ? 'started' : 'flag' });
  if (demo) {
    emit({ type: 'starting_app', command: 'sample shop' });
    emit({ type: 'app_ready', url });
  }
  // Like the browser init opens: a child that outlives a plain kill of its parent.
  const browser = spawn(process.execPath, ['-e', 'setInterval(() => {}, 1000)'], { stdio: 'ignore' });
  if (process.env.FAKE_PIDS) fs.writeFileSync(process.env.FAKE_PIDS, JSON.stringify({ browser: browser.pid, init: process.pid }));
  emit({ type: 'recording', proxy_url: 'http://127.0.0.1:9999', browser: 'chrome' });
  let calls = 0;
  const timer = setInterval(() => {
    calls += 1;
    emit({ type: 'progress', calls, endpoints: calls });
  }, 20);
  let buffer = '';
  const finish = () => {
    clearInterval(timer);
    browser.kill();
    emit({ type: 'recorded', calls, sessions: 1 });
    const tests = path.join(dir, 'api-tests', 'shop');
    fs.mkdirSync(tests, { recursive: true });
    const file = path.join(tests, 'checkout.yamlflow.yaml');
    fs.writeFileSync(file, 'flows:\n  - name: checkout\n');
    fs.writeFileSync(path.join(dir, '.env'), 'EMAIL=a@b.co\nexport PASSWORD="s3cret"\nOTHER=x\n');
    emit({
      type: 'generated',
      files: [{ path: file, flows: ['checkout'], steps: 4 }, { path: '/etc/hosts', flows: [], steps: 0 }],
      env: { path: path.join(dir, '.env'), names: ['EMAIL', 'PASSWORD'] },
    });
    const failed = mode === 'verify-fail';
    const failures = failed ? [{ flow: 'checkout', step: 'pay', reason: 'expected 200, got 500', hint: 'Seed a product.' }] : [];
    emit({ type: 'verified', run: 1, passed: failed ? 3 : 4, total: 4, failures });
    emit({ type: 'verified', run: 2, passed: failed ? 3 : 4, total: 4, failures });
    if (failed) emit({ type: 'fix_prompt', prompt: 'Fix api-tests/shop/checkout.yamlflow.yaml: pay fails (500).' });
    emit({ type: 'done', green: !failed });
    process.exit(failed ? 1 : 0);
  };
  if (mode !== 'ignore-stdin') {
    process.stdin.setEncoding('utf8').on('data', (chunk) => {
      buffer += chunk;
      if (buffer.includes('done\n')) finish();
    });
  }
  process.stdin.resume();
} else if (args[0] === 'install-skill') {
  if (mode === 'skill-conflict') {
    console.error('Refusing to overwrite existing file(s) not written by npx stresseur install-skill:');
    process.exit(1);
  }
  const files = [path.join(process.cwd(), '.claude/skills/stresseur/SKILL.md'), path.join(process.cwd(), '.cursor/rules/stresseur.mdc'), path.join(process.cwd(), 'AGENTS.md')];
  console.log('Wrote ' + files.length + ' file(s):');
  for (const file of files) console.log('  ' + file);
} else if (args[0] === 'init-ci') {
  fs.mkdirSync(path.join(process.cwd(), '.github/workflows'), { recursive: true });
  fs.writeFileSync(path.join(process.cwd(), '.github/workflows/stresseur.yml'), 'on: pull_request\n');
  console.log('Wrote .github/workflows/stresseur.yml');
}
`;

export const writeFakeStresseur = (dir: string) => {
  const bin = nodePath.join(dir, 'stresseur');
  fs.writeFileSync(bin, `#!${process.execPath}\n${FAKE_STRESSEUR}`, { mode: 0o755 });
  return bin;
};
