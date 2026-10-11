/* eslint-disable @typescript-eslint/no-floating-promises -- node:test's describe/it return promises by design */
import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import nodePath from 'node:path';
import { afterEach, beforeEach, describe, it, mock } from 'node:test';
import { LEGACY_DATA_DIR_NAMES, legacyDataDirs, migrateDataDir, type MigrationResult } from './migrate-data-dir.ts';
import { dismissRenameNotice, initRenameNotice, RENAME_NOTICE_FILE } from './rename-notice.ts';

let appData: string;
let userData: string;
let legacy: string;

const write = (file: string, content: string) => {
  fs.mkdirSync(nodePath.dirname(file), { recursive: true });
  fs.writeFileSync(file, content);
};
const read = (file: string) => fs.readFileSync(file, 'utf8');

/** A realistic 1.1.x user-data folder. */
const seedLegacy = (dir: string) => {
  write(nodePath.join(dir, 'state.db'), 'DB');
  write(nodePath.join(dir, 'state.db-wal'), 'WAL');
  write(nodePath.join(dir, 'state.db-shm'), 'SHM');
  write(nodePath.join(dir, 'Local Storage', 'leveldb', '000003.log'), 'theme=dark;anthropic-key=sk-test');
  write(nodePath.join(dir, 'logs', 'agent', 'run.jsonl'), '{}\n');
};

const run = (overrides: Partial<Parameters<typeof migrateDataDir>[0]> = {}): MigrationResult =>
  migrateDataDir({
    carryFiles: [RENAME_NOTICE_FILE],
    legacyDirs: legacyDataDirs(appData),
    userDataDir: userData,
    ...overrides,
  });

beforeEach(() => {
  appData = fs.mkdtempSync(nodePath.join(os.tmpdir(), 'migrate-data-dir-'));
  userData = nodePath.join(appData, 'Stresseur Studio');
  legacy = nodePath.join(appData, 'DevTools-Studio');
});

afterEach(() => void fs.rmSync(appData, { force: true, recursive: true }));

describe('legacy folder list', () => {
  it('keeps every historical name, most recent first', () => {
    assert.deepEqual(
      LEGACY_DATA_DIR_NAMES.map((_) => _.name),
      ['DevTools-Studio', 'DevTools Studio', 'DevTools'],
    );
  });
});

describe('migrateDataDir', () => {
  it('does nothing on a fresh install', () => {
    fs.mkdirSync(userData); // Electron creates it before app code runs
    assert.deepEqual(run(), { kind: 'fresh' });
    assert.deepEqual(fs.readdirSync(userData), []);
  });

  it('renames the whole 1.1.x folder into the empty folder Electron created', () => {
    seedLegacy(legacy);
    fs.mkdirSync(userData);

    assert.deepEqual(run(), { from: legacy, kind: 'migrated', method: 'rename' });
    assert.equal(read(nodePath.join(userData, 'state.db')), 'DB');
    assert.equal(read(nodePath.join(userData, 'state.db-wal')), 'WAL');
    assert.equal(
      read(nodePath.join(userData, 'Local Storage', 'leveldb', '000003.log')),
      'theme=dark;anthropic-key=sk-test',
    );
    assert.equal(fs.existsSync(legacy), false, 'moved, not duplicated');
  });

  it('also works when the new folder does not exist yet', () => {
    seedLegacy(legacy);
    assert.equal(run().kind, 'migrated');
    assert.equal(read(nodePath.join(userData, 'state.db')), 'DB');
  });

  it('never overwrites a folder that already has a database, and is idempotent', () => {
    seedLegacy(legacy);
    write(nodePath.join(userData, 'state.db'), 'NEW');

    assert.deepEqual(run(), { kind: 'current' });
    assert.deepEqual(run(), { kind: 'current' });
    assert.equal(read(nodePath.join(userData, 'state.db')), 'NEW');
    assert.equal(read(nodePath.join(legacy, 'state.db')), 'DB', 'legacy data untouched');
  });

  it('a second run after a successful migration is a no-op', () => {
    seedLegacy(legacy);
    run();
    assert.deepEqual(run(), { kind: 'current' });
  });

  it('prefers the most recent legacy folder', () => {
    write(nodePath.join(appData, 'DevTools', 'state.db'), 'OLDEST');
    write(nodePath.join(appData, 'DevTools Studio', 'state.db'), 'OLDER');
    seedLegacy(legacy);
    run();
    assert.equal(read(nodePath.join(userData, 'state.db')), 'DB');
    assert.equal(read(nodePath.join(appData, 'DevTools Studio', 'state.db')), 'OLDER', 'older folders untouched');
  });

  it('ignores legacy folders without a database', () => {
    write(nodePath.join(legacy, 'Preferences'), '{}');
    write(nodePath.join(appData, 'DevTools Studio', 'state.db'), 'OLDER');
    run();
    assert.equal(read(nodePath.join(userData, 'state.db')), 'OLDER');
  });

  it('copies and verifies when the new folder is not empty, keeping the legacy folder as a backup', () => {
    seedLegacy(legacy);
    write(nodePath.join(userData, 'Preferences'), '{"stale":true}');

    assert.deepEqual(run(), { from: legacy, kind: 'migrated', method: 'copy' });
    assert.equal(read(nodePath.join(userData, 'state.db')), 'DB');
    assert.equal(read(nodePath.join(userData, 'state.db-wal')), 'WAL');
    assert.equal(read(nodePath.join(userData, 'state.db-shm')), 'SHM');
    assert.equal(
      read(nodePath.join(userData, 'Local Storage', 'leveldb', '000003.log')),
      'theme=dark;anthropic-key=sk-test',
    );
    assert.equal(read(nodePath.join(legacy, 'state.db')), 'DB', 'backup kept');
    assert.deepEqual(
      fs.readdirSync(userData).filter((_) => _.endsWith('.migrating')),
      [],
      'no temp files left behind',
    );
  });

  it('falls back to copy-then-verify when the folder rename fails (e.g. a file locked on Windows)', () => {
    seedLegacy(legacy);
    fs.mkdirSync(userData);
    const renameSync = fs.renameSync.bind(fs);
    const spy = mock.method(fs, 'renameSync', (from: fs.PathLike, to: fs.PathLike) => {
      if (from === legacy) throw Object.assign(new Error('EBUSY: resource busy or locked'), { code: 'EBUSY' });
      renameSync(from, to);
    });
    try {
      assert.deepEqual(run(), { from: legacy, kind: 'migrated', method: 'copy' });
    } finally {
      spy.mock.restore();
    }
    assert.equal(read(nodePath.join(userData, 'state.db')), 'DB');
    assert.equal(
      read(nodePath.join(userData, 'Local Storage', 'leveldb', '000003.log')),
      'theme=dark;anthropic-key=sk-test',
    );
    assert.equal(read(nodePath.join(legacy, 'state.db')), 'DB', 'backup kept');
  });

  it('only copies the database (and carried files) from the generic 0.1.x "DevTools" folder, never renames it', () => {
    const generic = nodePath.join(appData, 'DevTools');
    write(nodePath.join(generic, 'state.db'), 'OLDEST');
    write(nodePath.join(generic, RENAME_NOTICE_FILE), '{"state":"dismissed"}');
    write(nodePath.join(generic, 'someone-elses-file'), 'x');

    assert.deepEqual(run(), { from: generic, kind: 'migrated', method: 'copy' });
    assert.equal(read(nodePath.join(userData, 'state.db')), 'OLDEST');
    assert.equal(read(nodePath.join(userData, RENAME_NOTICE_FILE)), '{"state":"dismissed"}');
    assert.equal(fs.existsSync(nodePath.join(userData, 'someone-elses-file')), false);
    assert.equal(fs.existsSync(generic), true);
  });

  it('recovers from a crash mid-copy: leftovers are replaced and state.db appears last', () => {
    seedLegacy(legacy);
    // Simulate a previous run that died after placing the WAL but before state.db.
    write(nodePath.join(userData, 'state.db-wal'), 'PARTIAL');
    write(nodePath.join(userData, 'state.db.migrating'), 'PARTIAL');

    assert.equal(run().kind, 'migrated');
    assert.equal(read(nodePath.join(userData, 'state.db')), 'DB');
    assert.equal(read(nodePath.join(userData, 'state.db-wal')), 'WAL');
    assert.equal(fs.existsSync(nodePath.join(userData, 'state.db.migrating')), false);
  });

  it('reports failure and leaves the legacy data intact when the copy cannot complete', () => {
    seedLegacy(legacy);
    // A regular file where the user-data folder should be: nothing can be written there.
    write(userData, 'not a folder');

    const result = run();
    assert.ok(result.kind === 'failed');
    assert.equal(result.from, legacy);
    assert.equal(read(nodePath.join(legacy, 'state.db')), 'DB');
    assert.equal(read(nodePath.join(legacy, 'state.db-wal')), 'WAL');
    assert.equal(read(userData), 'not a folder');
  });

  it('retries on the next launch after a failure', () => {
    seedLegacy(legacy);
    write(userData, 'not a folder');
    assert.equal(run().kind, 'failed');

    fs.rmSync(userData);
    assert.equal(run().kind, 'migrated');
    assert.equal(read(nodePath.join(userData, 'state.db')), 'DB');
  });

  it('does not treat the current folder as its own legacy folder', () => {
    write(nodePath.join(legacy, 'state.db'), 'DB');
    assert.deepEqual(run({ userDataDir: legacy }), { kind: 'current' });
  });
});

describe('rename notice', () => {
  it('is shown once to an upgraded user, then never again', () => {
    seedLegacy(legacy);
    fs.mkdirSync(userData);
    const migration = run();

    assert.equal(initRenameNotice(userData, migration), true);
    assert.equal(initRenameNotice(userData, run()), true, 'still pending until dismissed');
    dismissRenameNotice(userData);
    assert.equal(initRenameNotice(userData, run()), false);
  });

  it('is never shown on a fresh install, including later launches', () => {
    fs.mkdirSync(userData);
    assert.equal(initRenameNotice(userData, run()), false);
    write(nodePath.join(userData, 'state.db'), 'CREATED BY FIRST LAUNCH');
    assert.equal(initRenameNotice(userData, run()), false);
  });

  it('is shown when data already existed before this launch', () => {
    write(nodePath.join(userData, 'state.db'), 'DB');
    assert.equal(initRenameNotice(userData, run()), true);
  });

  it('survives a failed migration: shown from the legacy folder, dismissal carried over by the next migration', () => {
    seedLegacy(legacy);
    write(userData, 'not a folder');
    const failed = run();
    assert.equal(failed.kind, 'failed');

    // The app runs on the legacy folder for this session.
    assert.equal(initRenameNotice(legacy, failed), true);
    dismissRenameNotice(legacy);

    // Next launch: the migration succeeds and brings the dismissal along.
    fs.rmSync(userData);
    const migrated = run();
    assert.equal(migrated.kind, 'migrated');
    assert.equal(initRenameNotice(userData, migrated), false);
  });
});
