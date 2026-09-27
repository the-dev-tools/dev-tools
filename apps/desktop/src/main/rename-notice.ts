/**
 * One-time "DevTools Studio is now Stresseur Studio" notice for upgraded users.
 *
 * The state lives in a small JSON file in the user-data folder, next to
 * `state.db`, so it travels with the data (the migration renames or copies it).
 *
 * The decision is made once, at the first launch of a build that has this code:
 * - an upgraded user (data was migrated from a legacy folder, or a database
 *   already existed before this launch) gets `pending` → the notice is shown;
 * - a fresh install gets `not-needed` → it is never shown, even on later launches.
 * Dismissing it writes `dismissed`, so it is never shown twice.
 */
import fs from 'node:fs';
import nodePath from 'node:path';
import { DB_FILE, type MigrationResult } from './migrate-data-dir.ts';

export const RENAME_NOTICE_FILE = 'rename-notice.json';

type NoticeState = 'dismissed' | 'not-needed' | 'pending';

const isNoticeState = (_: unknown): _ is NoticeState => _ === 'dismissed' || _ === 'not-needed' || _ === 'pending';

const readState = (file: string): NoticeState | undefined => {
  try {
    const { state } = JSON.parse(fs.readFileSync(file, 'utf8')) as { state?: unknown };
    return isNoticeState(state) ? state : undefined;
  } catch {
    return undefined;
  }
};

const writeState = (file: string, state: NoticeState) => {
  try {
    fs.mkdirSync(nodePath.dirname(file), { recursive: true });
    fs.writeFileSync(file, JSON.stringify({ state }) + '\n');
  } catch {
    // Not being able to remember the notice must never break startup.
  }
};

/**
 * Decide whether to show the notice. Call it after `migrateDataDir` and before
 * the server starts (so a database created by this very launch doesn't count).
 *
 * @param userDataDir the folder the app will actually use for this session
 */
export const initRenameNotice = (userDataDir: string, migration: MigrationResult): boolean => {
  const file = nodePath.join(userDataDir, RENAME_NOTICE_FILE);

  const state = readState(file);
  if (state) return state === 'pending';

  const upgraded =
    migration.kind === 'migrated' || migration.kind === 'failed' || fs.existsSync(nodePath.join(userDataDir, DB_FILE));

  writeState(file, upgraded ? 'pending' : 'not-needed');
  return upgraded;
};

export const dismissRenameNotice = (userDataDir: string) =>
  void writeState(nodePath.join(userDataDir, RENAME_NOTICE_FILE), 'dismissed');
