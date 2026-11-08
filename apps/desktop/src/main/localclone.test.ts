import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import nodePath from 'node:path';
import { afterEach, beforeEach, describe, it } from 'node:test';
import { createLocalClones, parseDotenv, remoteMatches } from './localclone.ts';

let dir: string;
let clone: string;
let storeFile: string;
let picked: null | string;

const git = (cwd: string, ...args: string[]) =>
  execFileSync('git', ['-C', cwd, ...args], { encoding: 'utf8', stdio: ['ignore', 'pipe', 'ignore'] }).trim();

beforeEach(() => {
  dir = fs.realpathSync(fs.mkdtempSync(nodePath.join(os.tmpdir(), 'studio-clone-')));
  clone = nodePath.join(dir, 'shop');
  fs.mkdirSync(nodePath.join(clone, 'api-tests'), { recursive: true });
  git(dir, 'init', '-q', clone);
  git(clone, 'remote', 'add', 'origin', 'git@github.com:Acme/Shop.git');
  fs.writeFileSync(nodePath.join(clone, 'api-tests', 'checkout.yamlflow.yaml'), 'flows: []\n');
  git(clone, 'add', '.');
  git(clone, '-c', 'user.email=t@t', '-c', 'user.name=t', 'commit', '-q', '-m', 'init');
  storeFile = nodePath.join(dir, 'userData', 'local-clones.json');
  picked = clone;
});

afterEach(() => {
  fs.rmSync(dir, { force: true, recursive: true });
});

const clones = () => createLocalClones({ pickDirectory: () => Promise.resolve(picked), storeFile });

const linkedClones = async () => {
  const api = clones();
  const result = await api.pick({ full_name: 'acme/shop', slug: 'shop' });
  assert.deepEqual(result, { data: clone, ok: true });
  return api;
};

void describe('remote matching', () => {
  void it('accepts https and ssh forms, with or without .git', () => {
    for (const remote of [
      'https://github.com/acme/shop',
      'https://github.com/acme/shop.git',
      'https://github.com/Acme/Shop.git/',
      'https://x-access-token:abc@github.com/acme/shop.git',
      'git@github.com:acme/shop.git',
      'git@github.com:acme/shop',
      'ssh://git@github.com/acme/shop.git',
      'ssh://git@github.com:22/acme/shop',
    ])
      assert.ok(remoteMatches(remote, 'acme/shop'), remote);

    for (const remote of [
      'https://github.com/acme/shop-api.git',
      'https://github.com/evil/acme/shop.git',
      'git@github.com:other/shop.git',
      'file:///tmp/acme/shop',
      '/tmp/acme/shop',
    ])
      assert.ok(!remoteMatches(remote, 'acme/shop'), remote);
  });
});

void describe('local clones', () => {
  void it('links a clone whose origin is the repo, from any folder inside it', async () => {
    picked = nodePath.join(clone, 'api-tests');
    const api = await linkedClones();
    assert.equal(await api.get('shop'), clone);
    assert.deepEqual(JSON.parse(fs.readFileSync(storeFile, 'utf8')), { shop: clone });

    const head = await api.head('shop');
    assert.deepEqual(head, { data: git(clone, 'rev-parse', 'HEAD'), ok: true });

    await api.unlink('shop');
    assert.equal(await api.get('shop'), null);
  });

  void it('refuses a clone of another repo, or a folder that is not a clone', async () => {
    const api = clones();
    const wrong = await api.pick({ full_name: 'acme/other', slug: 'other' });
    assert.ok(wrong && !wrong.ok && wrong.message.includes('acme/other'));

    picked = fs.mkdtempSync(nodePath.join(dir, 'plain-'));
    const plain = await api.pick({ full_name: 'acme/shop', slug: 'shop' });
    assert.ok(plain && !plain.ok);

    picked = null;
    assert.equal(await api.pick({ full_name: 'acme/shop', slug: 'shop' }), null);
    assert.equal(await api.get('shop'), null);
  });

  void it('reads and writes flow files under api-tests/', async () => {
    const api = await linkedClones();
    assert.deepEqual(await api.read('shop', 'api-tests/checkout.yamlflow.yaml'), { data: 'flows: []\n', ok: true });

    assert.deepEqual(await api.write('shop', 'api-tests/new/nested/login.yamlflow.yaml', 'flows: [a]\n'), {
      data: null,
      ok: true,
    });
    assert.equal(
      fs.readFileSync(nodePath.join(clone, 'api-tests', 'new', 'nested', 'login.yamlflow.yaml'), 'utf8'),
      'flows: [a]\n',
    );
  });

  void it('refuses `..`, absolute paths, other extensions and other folders', async () => {
    const api = await linkedClones();
    for (const bad of [
      'api-tests/../.env',
      '../shop/api-tests/x.yamlflow.yaml',
      nodePath.join(clone, 'api-tests', 'x.yamlflow.yaml'),
      '/etc/x.yamlflow.yaml',
      'api-tests/x.yaml',
      'api-tests/x.yamlflow.yaml.sh',
      'src/x.yamlflow.yaml',
      '.env',
      'api-tests//x.yamlflow.yaml',
      'api-tests\\..\\x.yamlflow.yaml',
    ]) {
      const write = await api.write('shop', bad, 'x');
      assert.equal(write.ok, false, bad);
      const read = await api.read('shop', bad);
      assert.equal(read.ok, false, bad);
    }
    assert.equal(fs.existsSync(nodePath.join(clone, '.env')), false);
    assert.equal(fs.readdirSync(nodePath.join(clone, 'api-tests')).length, 1);
  });

  void it('refuses symlinks anywhere in the path', async () => {
    const api = await linkedClones();
    const outside = fs.mkdtempSync(nodePath.join(dir, 'outside-'));
    fs.writeFileSync(nodePath.join(outside, 'secret.yamlflow.yaml'), 'secret');

    // A symlinked folder inside api-tests.
    fs.symlinkSync(outside, nodePath.join(clone, 'api-tests', 'linked'));
    const viaFolder = await api.write('shop', 'api-tests/linked/secret.yamlflow.yaml', 'pwned');
    assert.ok(!viaFolder.ok && viaFolder.message.includes('symlink'));
    assert.equal((await api.read('shop', 'api-tests/linked/secret.yamlflow.yaml')).ok, false);
    assert.equal(fs.readFileSync(nodePath.join(outside, 'secret.yamlflow.yaml'), 'utf8'), 'secret');

    // A symlinked file.
    fs.symlinkSync(
      nodePath.join(outside, 'secret.yamlflow.yaml'),
      nodePath.join(clone, 'api-tests', 'f.yamlflow.yaml'),
    );
    assert.equal((await api.read('shop', 'api-tests/f.yamlflow.yaml')).ok, false);
    assert.equal((await api.write('shop', 'api-tests/f.yamlflow.yaml', 'pwned')).ok, false);
    assert.equal(fs.readFileSync(nodePath.join(outside, 'secret.yamlflow.yaml'), 'utf8'), 'secret');

    // api-tests itself is a symlink.
    fs.rmSync(nodePath.join(clone, 'api-tests'), { force: true, recursive: true });
    fs.symlinkSync(outside, nodePath.join(clone, 'api-tests'));
    assert.equal((await api.read('shop', 'api-tests/secret.yamlflow.yaml')).ok, false);
    assert.equal((await api.write('shop', 'api-tests/secret.yamlflow.yaml', 'pwned')).ok, false);
    assert.equal(fs.readFileSync(nodePath.join(outside, 'secret.yamlflow.yaml'), 'utf8'), 'secret');
  });

  void it('refuses a clone folder that became a symlink', async () => {
    const api = await linkedClones();
    const moved = nodePath.join(dir, 'moved');
    fs.renameSync(clone, moved);
    fs.symlinkSync(moved, clone);
    assert.equal((await api.read('shop', 'api-tests/checkout.yamlflow.yaml')).ok, false);
  });

  void it('returns only the requested .env values', async () => {
    const api = await linkedClones();
    assert.deepEqual(await api.envValues('shop', ['APP_URL']), { data: {}, ok: true });

    fs.writeFileSync(
      nodePath.join(clone, '.env'),
      [
        '# comment',
        'APP_URL=http://localhost:3000 # inline comment',
        'export API_KEY="abc def"',
        "PASSWORD='p#ss'",
        'SECRET=do-not-leak',
        'MULTI="line1',
        'line2"',
        'ESCAPED="a\\nb"',
        '',
      ].join('\n'),
    );
    const before = fs.readFileSync(nodePath.join(clone, '.env'), 'utf8');
    assert.deepEqual(await api.envValues('shop', ['APP_URL', 'API_KEY', 'PASSWORD', 'MULTI', 'ESCAPED', 'MISSING']), {
      data: {
        API_KEY: 'abc def',
        APP_URL: 'http://localhost:3000',
        ESCAPED: 'a\nb',
        MULTI: 'line1\nline2',
        PASSWORD: 'p#ss',
      },
      ok: true,
    });
    assert.equal(fs.readFileSync(nodePath.join(clone, '.env'), 'utf8'), before);
    assert.equal((await api.envValues('shop', ['../x'])).ok, false);
    assert.equal((await api.envValues('other', ['APP_URL'])).ok, false);
  });

  void it('refuses a .env that points outside the clone', async () => {
    const api = await linkedClones();
    const outside = nodePath.join(dir, 'outside.env');
    fs.writeFileSync(outside, 'APP_URL=x\n');
    fs.symlinkSync(outside, nodePath.join(clone, '.env'));
    assert.equal((await api.envValues('shop', ['APP_URL'])).ok, false);
  });

  void it('parses dotenv', () => {
    assert.deepEqual(Object.fromEntries(parseDotenv('A=1\r\nexport B = two \n#C=3\nD=\nE="x \\"y\\""\n')), {
      A: '1',
      B: 'two',
      D: '',
      E: 'x "y"',
    });
  });
});
