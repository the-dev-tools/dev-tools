import { useSyncExternalStore } from 'react';
import { track } from '~/app/umami';
import { readStored, writeStored } from '~/shared/lib';
import { recordApi } from './lib';

// One `stresseur init` run at a time, module-level so it outlives the dialogs and the header.

export interface RepoTarget {
  apiUrl: string;
  repo: StresseurCloudRepo;
  workspaceSlug: string;
}

/** Where recorded flows go. */
export type Destination = { dir: string; kind: 'folder' } | { kind: 'cloud'; repo: RepoTarget } | { kind: 'files' };

export type Verified = Extract<StresseurRecordEvent, { type: 'verified' }>;
type Finished = Extract<StresseurRecordEvent, { type: 'finished' }>;

export interface InitRun {
  calls: number;
  ci?: { path: string; warnings: string[] };
  destination: Destination;
  detected?: { command?: string; source: string; url: string };
  done?: { green: boolean };
  error?: { hint?: string; message: string };
  finished?: Finished;
  fixPrompt?: string;
  generated?: Extract<StresseurRecordEvent, { type: 'generated' }>;
  /** Set once the flows are in Studio (Files or a cloud repo). */
  imported?: { flowId?: Uint8Array; where: string };
  importError?: string;
  mode: 'demo' | 'record';
  pr?: { commands?: string[]; url?: string };
  recorded?: { calls: number; sessions: number };
  recording?: { browser: string; proxyUrl: string };
  startingApp?: string;
  /** `importing` while Studio imports the finished flows. */
  status: 'finished' | 'importing' | 'running';
  url?: string;
  verified: Verified[];
}

/** Applies one `init --json` event. */
export const applyInitEvent = (run: InitRun, event: StresseurRecordEvent): InitRun => {
  switch (event.type) {
    case 'app_ready':
      return run;
    case 'ci':
      return { ...run, ci: { path: event.path, warnings: event.warnings } };
    case 'detected':
      return {
        ...run,
        detected: { ...(event.command && { command: event.command }), source: event.source, url: event.url },
      };
    case 'done':
      return { ...run, done: { green: event.green } };
    case 'error':
      return { ...run, error: { ...(event.hint && { hint: event.hint }), message: event.message } };
    case 'finished':
      return { ...run, finished: event, status: 'finished' };
    case 'fix_prompt':
      return { ...run, fixPrompt: event.prompt };
    case 'generated':
      return { ...run, generated: event };
    case 'pr':
      return {
        ...run,
        pr: { ...(event.commands && { commands: event.commands }), ...(event.url && { url: event.url }) },
      };
    case 'progress':
      return { ...run, calls: event.calls };
    case 'recorded':
      return { ...run, calls: event.calls, recorded: { calls: event.calls, sessions: event.sessions } };
    case 'recording':
      return { ...run, recording: { browser: event.browser, proxyUrl: event.proxy_url } };
    case 'starting_app':
      return { ...run, startingApp: event.command };
    case 'verified':
      return { ...run, verified: [...run.verified.filter((_) => _.run !== event.run), event] };
  }
};

// Steps shown for a run

export type StepKey = 'detected' | 'generated' | 'recording' | 'verified';
export type StepState = 'active' | 'done' | 'failed' | 'pending';

export interface Step {
  command: string;
  detail?: string;
  failures?: StresseurInitFailure[];
  key: StepKey;
  label: string;
  state: StepState;
}

const quote = (arg: string) => (/^[\w./:@-]+$/.test(arg) ? arg : `'${arg.replaceAll("'", `'\\''`)}'`);

/** The CLI equivalent of the whole run. */
export const initCommand = (run: Pick<InitRun, 'destination' | 'mode' | 'url'>) =>
  [
    'npx stresseur init',
    run.mode === 'demo' ? '--demo' : `--url ${quote(run.url ?? '')}`,
    ...(run.destination.kind === 'folder' ? [`--dir ${quote(run.destination.dir)}`] : []),
  ].join(' ');

const plural = (count: number, one: string, many = `${one}s`) => `${String(count)} ${count === 1 ? one : many}`;

export const runSteps = (run: InitRun): Step[] => {
  const failed = !!run.error || run.done?.green === false || (run.status !== 'running' && !run.done);
  const last = run.verified.at(-1);
  const url = run.detected?.url ?? run.url ?? '';
  const reached = {
    detected: !!run.detected,
    generated: !!run.generated,
    recording: !!run.recorded,
    verified: run.verified.length >= 2 || !!run.done,
  };

  // The first unfinished step is active, or failed when the run stopped there.
  const order: StepKey[] = ['detected', 'recording', 'generated', 'verified'];
  const current = order.find((_) => !reached[_]);
  const state = (key: StepKey): StepState => {
    if (key === 'verified' && reached.verified) return last && last.failures.length > 0 ? 'failed' : 'done';
    if (reached[key]) return 'done';
    if (key !== current) return 'pending';
    return failed ? 'failed' : 'active';
  };

  const envNames = run.generated?.env?.names ?? [];
  const flows = run.generated?.files.reduce((sum, _) => sum + _.flows.length, 0) ?? 0;

  const detected = run.detected
    ? `${run.detected.url}${run.detected.command ? ` · ${run.detected.command}` : ''}`
    : run.startingApp
      ? `Starting: ${run.startingApp}`
      : undefined;

  const recording = run.recorded
    ? plural(run.recorded.calls, 'call')
    : run.recording
      ? `${plural(run.calls, 'call')} · use your app, then click Done here or in the browser`
      : undefined;

  return [
    {
      command: initCommand(run),
      ...(detected && { detail: detected }),
      key: 'detected',
      label: run.mode === 'demo' ? 'Sample shop' : 'Detected',
      state: state('detected'),
    },
    {
      command: `npx stresseur record --browser ${quote(url)}`,
      ...(recording && { detail: recording }),
      key: 'recording',
      label: 'Recording',
      state: state('recording'),
    },
    {
      command: 'npx stresseur generate',
      ...(run.generated && {
        detail: [
          `${plural(run.generated.files.length, 'file')}, ${plural(flows, 'flow')}`,
          envNames.length > 0 && `${envNames.join(', ')} saved to .env`,
        ]
          .filter(Boolean)
          .join(' · '),
      }),
      key: 'generated',
      label: 'Generated',
      state: state('generated'),
    },
    {
      command: 'npx stresseur ci --iterations 2 --warmup 0',
      ...(last && {
        detail: run.verified
          .map((_) => `Run ${String(_.run)}: ${String(_.passed)}/${String(_.total)} passed`)
          .join(' · '),
        ...(last.failures.length > 0 && { failures: last.failures }),
      }),
      key: 'verified',
      label: 'Ran twice',
      state: state('verified'),
    },
  ];
};

/** The step a run failed at, if it did. */
export const failedStep = (run: InitRun) => runSteps(run).find((_) => _.state === 'failed');

export const isGreen = (run: InitRun) => run.done?.green === true;

// Store

let run: InitRun | undefined;
const listeners = new Set<() => void>();
let started = false;

export const setRun = (next: InitRun | undefined) => {
  run = next;
  listeners.forEach((_) => void _());
};

export const getRun = () => run;

const subscribe = (listener: () => void) => {
  listeners.add(listener);
  const api = recordApi();
  if (!started && api) {
    started = true;
    api.onEvent((event) => {
      if (!run || run.status === 'finished') return;
      const next = applyInitEvent(run, event);
      if (event.type === 'error') track('record_failed');
      if (event.type === 'done') track('record_done', { green: event.green, mode: run.mode });
      setRun(event.type === 'finished' ? { ...next, status: 'importing' } : next);
    });
  }
  return () => void listeners.delete(listener);
};

export const useInitRun = () => useSyncExternalStore(subscribe, () => run);

const lastUrlKey = 'stresseur.record.url';
export const readLastUrl = () => readStored(lastUrlKey);

export const startRun = async (options: { destination: Destination; mode: 'demo' | 'record'; url?: string }) => {
  const api = recordApi();
  if (!api) return { message: 'Recording needs DevTools Studio for desktop.', ok: false as const };
  const { destination, mode, url } = options;
  // Before `start`: its first events arrive before it resolves.
  setRun({ calls: 0, destination, mode, status: 'running', ...(url && { url }), verified: [] });
  const result = await api.start({
    ...(mode === 'demo' ? { demo: true } : { url: url ?? '' }),
    ...(destination.kind === 'folder' && { dir: destination.dir }),
  });
  if (!result.ok) {
    setRun(undefined);
    track('record_failed');
    return result;
  }
  if (url) writeStored(lastUrlKey, url);
  track('record_start', { destination: destination.kind, mode });
  return result;
};

export const stopRecording = () => recordApi()?.stop();

export const cancelRun = async () => {
  await recordApi()?.cancel();
  setRun(undefined);
};

// Coding agent

/** The prompt for setting it all up with a coding agent (the skill does the rest). */
export const AGENT_PROMPT =
  'Set up API tests for this app with the stresseur skill: record it, make every flow pass twice, add the CI check and open a PR.';

/** A ready prompt for a failed run: the CLI's own when it gave one. */
export const fixPrompt = (current: InitRun) => {
  if (current.fixPrompt) return current.fixPrompt;
  const step = failedStep(current);
  const lines = [
    `\`${initCommand(current)}\` failed${step ? ` at "${step.label}"` : ''}.`,
    ...(current.error ? [`${current.error.message}${current.error.hint ? ` ${current.error.hint}` : ''}`] : []),
    ...(step?.failures ?? []).map((_) => `- ${_.flow} › ${_.step}: ${_.reason}${_.hint ? ` (${_.hint})` : ''}`),
    ...(current.finished?.temporary && current.finished.files.length > 0
      ? [`The flows are in ${current.finished.dir}/api-tests.`]
      : []),
    'Use the stresseur skill to fix it, then run it again until every flow passes twice.',
  ];
  return lines.join('\n');
};
