/**
 * User-data folder migration.
 *
 * Electron derives the user-data folder from the app name, so every time the
 * visible product name changed, the folder moved:
 *
 *   0.1.x        "DevTools"
 *   0.2.0        "DevTools Studio"
 *   0.3 – 1.1.x  "DevTools-Studio"
 *   1.2+         "Stresseur Studio"
 *
 * `migrateDataDir` brings the most recent legacy folder into the current one.
 * It runs synchronously at the very top of the main process, before the Go
 * server opens `state.db` and before Chromium writes into the folder.
 *
 * Guarantees:
 * - Never overwrites: if the current folder already has `state.db`, nothing happens.
 * - Never deletes: the legacy folder is either atomically renamed (so the data
 *   is always complete in exactly one place) or copied and kept as a backup.
 * - Idempotent and crash-safe: `state.db` in the current folder is the "done"
 *   marker. It is always the last file to appear, atomically, after every
 *   other file was copied and verified. A crash at any point before that
 *   leaves the legacy data untouched, and the next launch simply retries.
 * - On failure it returns `failed`, and the caller keeps running on the legacy
 *   folder for that session, so the app still starts with the user's data.
 *
 * This module only depends on Node built-ins so it can be unit tested without
 * Electron (see `migrate-data-dir.test.ts`).
 */
import { createHash } from 'node:crypto';
import fs from 'node:fs';
import nodePath from 'node:path';

export const DB_FILE = 'state.db';

/** SQLite files, in the order they are put in place: `state.db` must be last. */
const DB_FILES_IN_PLACEMENT_ORDER = [`${DB_FILE}-journal`, `${DB_FILE}-wal`, `${DB_FILE}-shm`, DB_FILE];

const TMP_SUFFIX = '.migrating';

export interface LegacyDataDir {
  path: string;
  /**
   * Whether the whole folder is ours: it may be moved with an atomic rename,
   * and the copy fallback may bring along everything in it (window state,
   * local storage with theme and AI provider keys, logs). When false, only the
   * database is copied, as 0.x did, because the folder name is too generic to
   * be sure the rest belongs to us.
   */
  wholeFolder: boolean;
}

/**
 * Legacy folder names, most recent first. These are identifiers of past
 * releases: never rename or remove an entry, only add new ones at the top.
 */
export const LEGACY_DATA_DIR_NAMES: readonly { name: string; wholeFolder: boolean }[] = [
  // 0.3 – 1.1.x (current folder before the Stresseur Studio rename)
  { name: 'DevTools-Studio', wholeFolder: true },
  // 0.2.0
  { name: 'DevTools Studio', wholeFolder: true },
  // 0.1.x
  { name: 'DevTools', wholeFolder: false },
];

export const legacyDataDirs = (appDataDir: string): LegacyDataDir[] =>
  LEGACY_DATA_DIR_NAMES.map(({ name, wholeFolder }) => ({ path: nodePath.join(appDataDir, name), wholeFolder }));

export type MigrationResult =
  /** Migration failed. The legacy folder is intact and should be used for this session. */
  | { error: unknown; from: string; kind: 'failed' }
  /** Legacy data now lives in the current folder. */
  | { from: string; kind: 'migrated'; method: 'copy' | 'rename' }
  /** The current folder already had a database. Nothing was touched. */
  | { kind: 'current' }
  /** No legacy data was found. Nothing was touched. */
  | { kind: 'fresh' };

export interface MigrateDataDirOptions {
  /** Small files that must also be carried over when only the database is copied. */
  carryFiles?: readonly string[];
  /** Legacy folders, most recent first. */
  legacyDirs: readonly LegacyDataDir[];
  log?: (message: string) => void;
  /** The current user-data folder. */
  userDataDir: string;
}

const hasDb = (dir: string) => fs.existsSync(nodePath.join(dir, DB_FILE));

const isEmptyDir = (dir: string) => {
  try {
    return fs.readdirSync(dir).length === 0;
  } catch {
    return false;
  }
};

const sha256 = (file: string) => createHash('sha256').update(fs.readFileSync(file)).digest('hex');

/**
 * Move the whole legacy folder into place with one atomic rename. Electron
 * creates the (empty) user-data folder before any app code runs, so remove it
 * first: Windows cannot rename onto an existing folder.
 */
const migrateByRename = (from: string, to: string) => {
  if (fs.existsSync(to)) fs.rmdirSync(to); // only called when it is empty; rmdir refuses otherwise
  try {
    fs.renameSync(from, to);
  } catch (error) {
    // Recreate the empty folder Electron expects before falling back to copying.
    fs.mkdirSync(to, { recursive: true });
    throw error;
  }
};

/**
 * Copy, verify, then place. The legacy folder is never modified.
 *
 * 1. (whole-folder sources only) copy everything except the database, best
 *    effort: losing a window position is not worth failing the migration.
 * 2. Copy the database files next to their final names, and verify each copy
 *    by size and SHA-256.
 * 3. Rename them into place, `state.db` last.
 */
const migrateByCopy = (source: LegacyDataDir, to: string, carryFiles: readonly string[], log: (_: string) => void) => {
  const from = source.path;
  fs.mkdirSync(to, { recursive: true });

  const isDbFile = (src: string) =>
    nodePath.dirname(src) === from && DB_FILES_IN_PLACEMENT_ORDER.includes(nodePath.basename(src));

  if (source.wholeFolder) {
    try {
      fs.cpSync(from, to, { filter: (src) => !isDbFile(src), force: true, recursive: true });
    } catch (error) {
      log(`Some non-database files could not be copied: ${String(error)}`);
    }
  } else {
    for (const file of carryFiles) {
      const src = nodePath.join(from, file);
      if (fs.existsSync(src)) fs.copyFileSync(src, nodePath.join(to, file));
    }
  }

  const dbFiles = DB_FILES_IN_PLACEMENT_ORDER.filter((file) => fs.existsSync(nodePath.join(from, file)));
  const tmpFiles: string[] = [];

  try {
    for (const file of dbFiles) {
      const src = nodePath.join(from, file);
      const tmp = nodePath.join(to, file + TMP_SUFFIX);
      tmpFiles.push(tmp);
      fs.copyFileSync(src, tmp);
      if (fs.statSync(src).size !== fs.statSync(tmp).size || sha256(src) !== sha256(tmp))
        throw new Error(`Copy of ${file} does not match the original`);
    }

    for (const file of dbFiles) fs.renameSync(nodePath.join(to, file + TMP_SUFFIX), nodePath.join(to, file));
  } catch (error) {
    for (const tmp of tmpFiles) fs.rmSync(tmp, { force: true });
    throw error;
  }
};

export const migrateDataDir = ({
  carryFiles = [],
  legacyDirs,
  log = () => undefined,
  userDataDir,
}: MigrateDataDirOptions): MigrationResult => {
  // Never overwrite existing data. This is also what makes the migration idempotent.
  if (hasDb(userDataDir)) return { kind: 'current' };

  const source = legacyDirs.find(
    (dir) => nodePath.resolve(dir.path) !== nodePath.resolve(userDataDir) && hasDb(dir.path),
  );
  if (!source) return { kind: 'fresh' };

  log(`Migrating user data from ${source.path} to ${userDataDir}`);

  if (source.wholeFolder && (!fs.existsSync(userDataDir) || isEmptyDir(userDataDir))) {
    try {
      migrateByRename(source.path, userDataDir);
      log('User data migrated (folder renamed)');
      return { from: source.path, kind: 'migrated', method: 'rename' };
    } catch (error) {
      log(`Rename failed, falling back to copy: ${String(error)}`);
    }
  }

  try {
    migrateByCopy(source, userDataDir, carryFiles, log);
    log(`User data migrated (copied; ${source.path} was kept as a backup)`);
    return { from: source.path, kind: 'migrated', method: 'copy' };
  } catch (error) {
    log(`User data migration failed, using ${source.path} for this session: ${String(error)}`);
    return { error, from: source.path, kind: 'failed' };
  }
};
