/**
 * First-run launchpad, main-process side: finds running dev servers, installs the
 * coding-agent skill into a repo, builds the coding-agent deep links, watches a repo's
 * api-tests/ and .stresseur/ for the agent's progress, and adds the CI check / opens
 * the PR. Repo folders must come from the folder picker (see `allowDir`).
 */
import { execFile, spawn } from 'node:child_process';
import { promises as fs, type FSWatcher, watch } from 'node:fs';
import path from 'node:path';
import { exited, killTree, tail } from './record.ts';

// Running dev servers

export const DEV_PORTS = [3000, 5173, 4321, 8000, 5000, 8080];

export interface DevServer {
  port: number;
  status: number;
  title?: string;
  url: string;
}

const titleOf = (html: string) => {
  const match = /<title[^>]*>([^<]{1,120})<\/title>/i.exec(html);
  const title = match?.[1]?.replace(/\s+/g, ' ').trim();
  return title === '' ? undefined : title;
};

// Server headers of system services on the usual dev ports: macOS's AirPlay receiver
// answers on :5000 (Flask's default) and :7000. Matches the CLI's appdetect.
const NOT_APPS = /\b(AirTunes|ControlCenter)\b/i;

/** Ports where an app answers HTTP on localhost, in the order given. */
export const detectDevServers = async (ports = DEV_PORTS, timeoutMs = 800): Promise<DevServer[]> => {
  const probe = async (port: number): Promise<DevServer | undefined> => {
    const url = `http://localhost:${String(port)}`;
    try {
      const response = await fetch(url, { redirect: 'manual', signal: AbortSignal.timeout(timeoutMs) });
      if (NOT_APPS.test(response.headers.get('server') ?? '')) {
        void response.body?.cancel();
        return undefined;
      }
      const type = response.headers.get('content-type') ?? '';
      const title = type.includes('html') ? titleOf((await response.text()).slice(0, 65_536)) : undefined;
      if (!type.includes('html')) void response.body?.cancel();
      return { port, status: response.status, ...(title !== undefined && { title }), url };
    } catch {
      return undefined;
    }
  };
  const found = await Promise.all(ports.map(probe));
  return found.filter((_): _ is DevServer => _ !== undefined);
};

// Commands in a repo

interface CommandResult {
  code: null | number;
  stderr: string;
  stdout: string;
}

const runIn = (bin: string, args: string[], cwd: string, timeoutMs = 120_000) =>
  new Promise<CommandResult>((resolve) => {
    const child = spawn(bin, args, {
      cwd,
      detached: process.platform !== 'win32',
      stdio: ['ignore', 'pipe', 'pipe'],
      windowsHide: true,
    });
    const stdout: string[] = [];
    const stderr: string[] = [];
    child.stdout.setEncoding('utf8').on('data', (chunk: string) => void stdout.push(chunk));
    child.stderr.setEncoding('utf8').on('data', (chunk: string) => void stderr.push(chunk));
    const timer = setTimeout(() => void killTree(child), timeoutMs);
    child.once('error', (error) => {
      clearTimeout(timer);
      resolve({ code: null, stderr: error.message, stdout: '' });
    });
    void exited(child).then(({ code }) => {
      clearTimeout(timer);
      resolve({ code, stderr: stderr.join(''), stdout: stdout.join('') });
    });
  });

const failure = (result: CommandResult, fallback: string) => tail([result.stderr]) || tail([result.stdout]) || fallback;

export type Result<T> = { data: T; ok: true } | { message: string; ok: false };

// The coding-agent skill

export type AgentName = 'Claude Code' | 'Codex' | 'Cursor';

/** Agents from `install-skill`'s "Wrote N file(s):" list. */
export const agentsFromPaths = (paths: string[]): AgentName[] => {
  const agents = new Set<AgentName>();
  for (const file of paths.map((_) => _.split(path.sep).join('/'))) {
    if (file.includes('/.claude/') || file.startsWith('.claude/')) agents.add('Claude Code');
    else if (file.endsWith('AGENTS.md')) agents.add('Codex');
    else if (file.includes('.cursor/rules/')) agents.add('Cursor');
  }
  return (['Claude Code', 'Codex', 'Cursor'] as const).filter((_) => agents.has(_));
};

export const installSkill = async (
  bin: string,
  dir: string,
): Promise<Result<{ agents: AgentName[]; files: string[] }>> => {
  const result = await runIn(bin, ['install-skill'], dir);
  if (result.code !== 0) return { message: failure(result, 'Could not add the skill.'), ok: false };
  const files = result.stdout
    .split('\n')
    .filter((_) => /^\s{2,}\S/.test(_))
    .map((_) => _.trim());
  return { data: { agents: agentsFromPaths(files), files }, ok: true };
};

// Coding-agent deep links (documented URL schemes only)

/** https://code.claude.com/docs/en/deep-links: `q` up to 5,000 characters, `cwd` absolute. */
const CLAUDE_PROMPT_MAX = 5_000;
/** https://cursor.com/docs/integrations/deeplinks: the whole URL up to 10,000 characters. */
const CURSOR_URL_MAX = 10_000;

export const agentLink = (agent: unknown, dir: unknown, prompt: unknown): string | undefined => {
  if (typeof prompt !== 'string' || !prompt) return undefined;
  if (agent === 'claude') {
    if (prompt.length > CLAUDE_PROMPT_MAX) return undefined;
    const cwd = typeof dir === 'string' && path.isAbsolute(dir) && !dir.split(/[\\/]/).includes('..') ? dir : undefined;
    const params = new URLSearchParams({ ...(cwd && { cwd }), q: prompt });
    return `claude-cli://open?${params.toString().replaceAll('+', '%20')}`;
  }
  if (agent === 'cursor') {
    const url = `cursor://anysphere.cursor-deeplink/prompt?text=${encodeURIComponent(prompt)}`;
    return url.length > CURSOR_URL_MAX ? undefined : url;
  }
  return undefined;
};

// The agent's progress in a repo

export interface RepoProgress {
  /** Steps passed / total over every flow of the last verification. */
  checks?: { flows: number; green: boolean; passed: number; passedFlows: number; total: number; verifiedAt: string };
  /** Generated flow files under api-tests/. */
  flowFiles: string[];
  pr?: { number?: number; state?: string; url: string };
  recording?: { active: boolean; updatedAt: number };
}

const walkFlows = async (root: string, dir = root, depth = 0): Promise<string[]> => {
  if (depth > 6) return [];
  const entries = await fs.readdir(dir, { withFileTypes: true }).catch(() => []);
  const out: string[] = [];
  for (const entry of entries) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory() && !entry.name.startsWith('.') && entry.name !== 'node_modules')
      out.push(...(await walkFlows(root, full, depth + 1)));
    else if (entry.isFile() && /\.ya?ml$/.test(entry.name))
      out.push(path.relative(root, full).split(path.sep).join('/'));
  }
  return out.sort();
};

const readChecks = async (dir: string): Promise<RepoProgress['checks']> => {
  const text = await fs.readFile(path.join(dir, '.stresseur', 'last-verify.json'), 'utf8').catch(() => undefined);
  if (text === undefined) return undefined;
  try {
    const parsed = JSON.parse(text) as { flows?: unknown; verifiedAt?: unknown };
    const flows = Array.isArray(parsed.flows) ? (parsed.flows as Record<string, unknown>[]) : [];
    const count = (key: string) => flows.reduce((sum, _) => sum + (typeof _[key] === 'number' ? _[key] : 0), 0);
    const passedFlows = flows.filter((_) => _['status'] === 'passed').length;
    return {
      flows: flows.length,
      green: flows.length > 0 && passedFlows === flows.length,
      passed: count('passed'),
      passedFlows,
      total: count('total'),
      verifiedAt: typeof parsed.verifiedAt === 'string' ? parsed.verifiedAt : '',
    };
  } catch {
    return undefined;
  }
};

/** Recording counts as active while recording.db changed in the last `activeMs`. */
export const scanRepo = async (dir: string, now = Date.now(), activeMs = 10_000): Promise<RepoProgress> => {
  const db = await fs.stat(path.join(dir, '.stresseur', 'recording.db')).catch(() => undefined);
  const testsDir = path.join(dir, 'api-tests');
  const flowFiles = await walkFlows(testsDir);
  const checks = await readChecks(dir);
  return {
    ...(checks && { checks }),
    flowFiles,
    ...(db && { recording: { active: now - db.mtimeMs < activeMs, updatedAt: db.mtimeMs } }),
  };
};

export type StepState = 'active' | 'done' | 'pending';

/** The progress view's four steps, from a scan. */
export const progressSteps = (progress: RepoProgress) => {
  const generated = progress.flowFiles.length > 0;
  const recording: StepState = progress.recording?.active
    ? 'active'
    : progress.recording || generated
      ? 'done'
      : 'pending';
  const green = progress.checks?.green === true;
  return {
    checks: green ? 'done' : progress.checks ? 'active' : 'pending',
    generated: generated ? 'done' : 'pending',
    pr: progress.pr ? 'done' : 'pending',
    recording,
  } satisfies Record<string, StepState>;
};

const execText = (bin: string, args: string[], cwd: string) =>
  new Promise<string | undefined>((resolve) => {
    execFile(
      bin,
      args,
      { cwd, timeout: 15_000, windowsHide: true },
      (error, stdout) => void resolve(error ? undefined : stdout.trim()),
    );
  });

/** The open PR for the checked-out branch, via `gh` when it is installed. */
export const findPr = async (dir: string, gh = 'gh'): Promise<RepoProgress['pr']> => {
  const out = await execText(gh, ['pr', 'view', '--json', 'url,state,number'], dir);
  if (!out) return undefined;
  try {
    const parsed = JSON.parse(out) as { number?: unknown; state?: unknown; url?: unknown };
    if (typeof parsed.url !== 'string') return undefined;
    return {
      ...(typeof parsed.number === 'number' && { number: parsed.number }),
      ...(typeof parsed.state === 'string' && { state: parsed.state }),
      url: parsed.url,
    };
  } catch {
    return undefined;
  }
};

export interface WatchOptions {
  /** Debounce for file events. */
  debounceMs?: number;
  /** Asks `gh` about a PR, once flows exist. Missing: never. */
  findPr?: (dir: string) => Promise<RepoProgress['pr']>;
  onProgress: (progress: RepoProgress) => void;
  /** Rescan anyway (fs events can be missed, and "recording" goes stale). */
  pollMs?: number;
  prEveryMs?: number;
}

/** Watches the repo, api-tests/ and .stresseur/; reports a scan after every change. */
export const watchRepo = (
  dir: string,
  { debounceMs = 200, findPr: lookUpPr, onProgress, pollMs = 3_000, prEveryMs = 15_000 }: WatchOptions,
) => {
  let closed = false;
  /** A function, so the check after an await is not narrowed away. */
  const isClosed = () => closed;
  let timer: NodeJS.Timeout | undefined;
  let pr: RepoProgress['pr'];
  let prCheckedAt = 0;
  let last = '';
  const watchers = new Map<string, FSWatcher>();

  const report = async () => {
    if (isClosed()) return;
    const progress = await scanRepo(dir);
    if (lookUpPr && progress.flowFiles.length > 0 && !pr && Date.now() - prCheckedAt >= prEveryMs) {
      prCheckedAt = Date.now();
      pr = await lookUpPr(dir);
    }
    if (isClosed()) return;
    const next = { ...progress, ...(pr && { pr }) };
    const key = JSON.stringify(next);
    if (key === last) return;
    last = key;
    onProgress(next);
  };

  const schedule = () => {
    clearTimeout(timer);
    timer = setTimeout(() => void report().then(attach), debounceMs);
  };

  /** Watches the folders that exist; api-tests/ and .stresseur/ may appear later. */
  const attach = () => {
    if (isClosed()) return;
    for (const [sub, recursive] of [
      ['', false],
      ['api-tests', true],
      ['.stresseur', false],
    ] as const) {
      const target = path.join(dir, sub);
      if (watchers.has(target)) continue;
      try {
        const watcher = watch(target, { recursive }, schedule);
        watcher.on('error', () => {
          watcher.close();
          watchers.delete(target);
        });
        watchers.set(target, watcher);
      } catch {
        // Not there yet.
      }
    }
  };

  attach();
  void report();
  const poll = setInterval(() => void report().then(attach), pollMs);

  return () => {
    closed = true;
    clearTimeout(timer);
    clearInterval(poll);
    for (const watcher of watchers.values()) watcher.close();
    watchers.clear();
  };
};

// CI check and PR

export const addCiCheck = async (bin: string, dir: string): Promise<Result<{ output: string }>> => {
  const result = await runIn(bin, ['init-ci'], dir);
  if (result.code !== 0) return { message: failure(result, 'Could not add the CI check.'), ok: false };
  return { data: { output: result.stdout.trim() }, ok: true };
};

/** Paths a PR for the generated flows carries, when they exist. */
const PR_PATHS = ['api-tests', '.env.example', '.gitignore', '.github/workflows/stresseur.yml'];

export const prCommands = (branch: string, paths: string[]) => [
  `git checkout -b ${branch}`,
  `git add ${paths.join(' ')}`,
  'git commit -m "Add API tests"',
  'git push -u origin HEAD',
  'gh pr create --fill',
];

/** Commits the flows on a new branch, pushes it and opens a PR with `gh`; without `gh`, the commands. */
export const openPr = async (
  dir: string,
  { gh = 'gh', git = 'git' }: { gh?: string; git?: string } = {},
): Promise<Result<{ commands?: string[]; url?: string }>> => {
  const paths: string[] = [];
  for (const candidate of PR_PATHS)
    if (await fs.stat(path.join(dir, candidate)).catch(() => undefined)) paths.push(candidate);
  if (!paths.includes('api-tests')) return { message: 'No api-tests folder in this repo.', ok: false };

  const branch = `stresseur/api-tests-${new Date().toISOString().slice(0, 10)}`;
  const commands = prCommands(branch, paths);
  if ((await execText(gh, ['--version'], dir)) === undefined) return { data: { commands }, ok: true };

  const steps: [string, string[]][] = [
    [git, ['checkout', '-b', branch]],
    [git, ['add', ...paths]],
    [git, ['commit', '-m', 'Add API tests']],
    [git, ['push', '-u', 'origin', 'HEAD']],
    [gh, ['pr', 'create', '--fill']],
  ];
  let output = '';
  for (const [bin, args] of steps) {
    const result = await runIn(bin, args, dir);
    if (result.code !== 0)
      return { message: failure(result, `${path.basename(bin)} ${args[0] ?? ''} failed.`), ok: false };
    output = result.stdout;
  }
  const url = /https:\/\/\S+/.exec(output)?.[0];
  return { data: { ...(url && { url }) }, ok: true };
};

// Folders the renderer may use

/** Folders picked in this session; the renderer cannot name others. */
export const createDirAllowlist = () => {
  const dirs = new Set<string>();
  return {
    add: (dir: string) => void dirs.add(path.resolve(dir)),
    has: (dir: unknown): dir is string => typeof dir === 'string' && dirs.has(path.resolve(dir)),
  };
};
