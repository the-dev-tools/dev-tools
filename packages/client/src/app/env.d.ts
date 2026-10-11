import type { Dialog } from 'electron';

declare const PUBLIC_ENV: object;

declare global {
  interface StresseurAccount {
    user: { email: string; name?: string };
    workspace: { name: string; slug: string };
  }

  type StresseurSignInEvent =
    | { account: StresseurAccount; kind: 'signed-in' }
    | { kind: 'failed'; message: string }
    | { kind: 'waiting'; url: string; userCode: string };

  /** Error from a Stresseur Cloud request, mapped in the main process. */
  type StresseurCloudError =
    | { code: string; fixUrl?: string; kind: 'error'; message: string; upgrade?: boolean }
    | { kind: 'offline' }
    | { kind: 'sign-in-again' }
    | { kind: 'signed-out' };

  /** `offline` is set when the data comes from the last saved copy. */
  type StresseurCloudResult<T> = { data: T; offline?: boolean; ok: true } | { error: StresseurCloudError; ok: false };

  interface StresseurCloudRepo {
    default_branch: string;
    flow_count: number;
    full_name: string;
    /** Null until the repo's first scan. */
    head_sha: null | string;
    records_ci: boolean;
    slug: string;
    url: string;
  }

  interface StresseurCloudRepos {
    repos: StresseurCloudRepo[];
    workspace: { name: string; slug: string };
  }

  interface StresseurCloudFlow {
    last_ci: null | { at: string; status: 'failed' | 'passed'; url: string };
    line?: number;
    name: string;
    pass_rate: null | number;
    path: string;
    url: string;
    yaml_sha: string;
  }

  interface StresseurCloudFlows {
    flows: StresseurCloudFlow[];
    head_sha: null | string;
  }

  interface StresseurCloudFile {
    blob_sha: string;
    /** Null until the repo's first scan. */
    commit_sha: null | string;
    path: string;
    text: string;
  }

  interface StresseurCloudEnvironment {
    id: string;
    name: string;
  }

  interface StresseurCloudPullRequest {
    branch: string;
    main_moved: boolean;
    number: number;
    url: string;
  }

  interface StresseurCloudRun {
    finished_at?: string;
    flow: string;
    id: string;
    log_tail: string;
    started_at?: string;
    status: 'error' | 'failed' | 'passed' | 'queued' | 'running';
    summary?: { steps: { duration_ms?: number; message?: string; name: string; status: string }[] };
    url: string;
  }

  /** Included AI credits for the coding agent, this UTC month. */
  interface StresseurAiCredits {
    included: boolean;
    limit_cents: number;
    resets_at: string;
    used_cents: number;
  }

  type StresseurLocalResult<T> = { data: T; ok: true } | { message: string; ok: false };

  interface StresseurInitFailure {
    flow: string;
    hint?: string;
    reason: string;
    step: string;
  }

  interface StresseurInitFile {
    flows: string[];
    /** Absolute. */
    path: string;
    steps: number;
  }

  interface StresseurGeneratedFile {
    flows: string[];
    /** Relative to the generated api-tests folder, e.g. "shop/checkout.yamlflow.yaml". */
    path: string;
    text: string;
  }

  /** NDJSON events from `stresseur init --json`, plus Studio's own `finished`. */
  type StresseurRecordEvent =
    | { browser: string; proxy_url: string; type: 'recording' }
    | { calls: number; endpoints: number; type: 'progress' }
    | { calls: number; sessions: number; type: 'recorded' }
    | {
        code: null | number;
        dir: string;
        env: Record<string, string>;
        files: StresseurGeneratedFile[];
        temporary: boolean;
        type: 'finished';
      }
    | { command: string; type: 'starting_app' }
    | { command?: string; source: string; type: 'detected'; url: string }
    | { commands?: string[]; type: 'pr'; url?: string }
    | { env?: { names: string[]; path: string }; files: StresseurInitFile[]; type: 'generated' }
    | { failures: StresseurInitFailure[]; passed: number; run: number; total: number; type: 'verified' }
    | { green: boolean; type: 'done' }
    | { hint?: string; message: string; type: 'error' }
    | { path: string; type: 'ci'; warnings: string[] }
    | { prompt: string; type: 'fix_prompt' }
    | { type: 'app_ready'; url: string };

  interface StresseurDevServer {
    port: number;
    status: number;
    title?: string;
    url: string;
  }

  type StresseurStepState = 'active' | 'done' | 'pending';

  interface StresseurRepoProgress {
    checks?: { flows: number; green: boolean; passed: number; passedFlows: number; total: number; verifiedAt: string };
    flowFiles: string[];
    pr?: { number?: number; state?: string; url: string };
    recording?: { active: boolean; updatedAt: number };
  }

  interface StresseurRepoProgressEvent {
    dir: string;
    progress: StresseurRepoProgress;
    steps: Record<'checks' | 'generated' | 'pr' | 'recording', StresseurStepState>;
  }

  interface Window {
    electron: {
      /** Missing outside the desktop app. */
      account?: {
        cancel: () => void;
        get: () => Promise<null | StresseurAccount>;
        onEvent: (callback: (event: StresseurSignInEvent) => void) => () => void;
        open: () => void;
        signIn: () => void;
        signOut: () => Promise<void>;
      };
      /** Stresseur Cloud API (/api/cli/studio/*). The token stays in the main process. */
      cloud?: {
        aiAbort: (id: string) => void;
        aiCredits: () => Promise<StresseurCloudResult<StresseurAiCredits>>;
        /** Anthropic Messages on included AI credits; `onEvent` gets each stream event. */
        aiMessages: (
          id: string,
          body: object,
          onEvent: (event: unknown) => void,
        ) => Promise<StresseurCloudResult<null>>;
        apiUrl: () => Promise<string>;
        environments: (repo: string) => Promise<StresseurCloudResult<{ environments: StresseurCloudEnvironment[] }>>;
        file: (repo: string, path: string) => Promise<StresseurCloudResult<StresseurCloudFile>>;
        flows: (repo: string) => Promise<StresseurCloudResult<StresseurCloudFlows>>;
        /** Opens an https URL in the browser. */
        open: (url: string) => void;
        proposeChange: (
          repo: string,
          change: { base_sha: string; body?: string; content: string; path: string; title?: string },
        ) => Promise<StresseurCloudResult<StresseurCloudPullRequest>>;
        repos: () => Promise<StresseurCloudResult<StresseurCloudRepos>>;
        run: (id: string) => Promise<StresseurCloudResult<StresseurCloudRun>>;
        startRun: (
          repo: string,
          run: { environment: string; flow: string },
        ) => Promise<StresseurCloudResult<{ run_id: string; url: string }>>;
      };
      /** Flows recorded with `init --demo`, run against `stresseur demo`. Flow ids are canonical ULIDs. */
      demo?: {
        mark: (flowIds: string[], env: Record<string, string>) => Promise<StresseurLocalResult<null>>;
        /** #env: overrides for a local run: null for other flows; for a demo flow APP_URL of the started sample shop. */
        runEnv: (flowId: string) => Promise<StresseurLocalResult<null | Record<string, string>>>;
      };
      dialog: <T extends keyof Dialog>(method: T, ...options: Parameters<Dialog[T]>) => Promise<ReturnType<Dialog[T]>>;
      /** First-run launchpad. Repo commands take folders picked with `pickFolder` only. */
      launchpad?: {
        addCi: (dir: string) => Promise<StresseurLocalResult<{ output: string }>>;
        detect: () => Promise<StresseurDevServer[]>;
        installSkill: (dir: string) => Promise<StresseurLocalResult<{ agents: string[]; files: string[] }>>;
        onProgress: (callback: (event: StresseurRepoProgressEvent) => void) => () => void;
        /** Opens Claude Code or Cursor with the prompt typed in; false when it does not fit a link. */
        openAgent: (agent: 'claude' | 'cursor', dir: string, prompt: string) => Promise<boolean>;
        openPr: (dir: string) => Promise<StresseurLocalResult<{ commands?: string[]; url?: string }>>;
        pickFolder: (title?: string) => Promise<null | string>;
        unwatch: () => Promise<void>;
        watch: (dir: string) => Promise<boolean>;
      };
      /** Linked local clones, keyed by repo slug. Writes only <clone>/api-tests/**\/*.yamlflow.yaml. */
      localClone?: {
        /** Values from the clone's .env for the given names only. Never written. */
        envValues: (repo: string, names: string[]) => Promise<StresseurLocalResult<Record<string, string>>>;
        get: (repo: string) => Promise<null | string>;
        /** Commit checked out in the clone. */
        head: (repo: string) => Promise<StresseurLocalResult<string>>;
        /** Folder picker; checks that `origin` is the repo. Resolves null when cancelled. */
        pick: (repo: { full_name: string; slug: string }) => Promise<null | StresseurLocalResult<string>>;
        read: (repo: string, path: string) => Promise<StresseurLocalResult<string>>;
        unlink: (repo: string) => Promise<void>;
        write: (repo: string, path: string, text: string) => Promise<StresseurLocalResult<null>>;
      };
      /** Bundled `stresseur init --json`. */
      record?: {
        cancel: () => Promise<void>;
        onEvent: (callback: (event: StresseurRecordEvent) => void) => () => void;
        start: (options: {
          demo?: boolean;
          dir?: string;
          url?: string;
        }) => Promise<StresseurLocalResult<{ args: string[]; dir: string }>>;
        /** Ends the recording; generating and verifying follow as events. */
        stop: () => Promise<StresseurLocalResult<null>>;
      };
    };
  }
}
