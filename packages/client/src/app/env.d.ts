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

  /** NDJSON events from `stresseur record --json`, plus Studio's own `generating`. */
  type StresseurRecordEvent =
    | { calls: number; endpoints: number; sessions: number; type: 'progress' }
    | { calls: number; endpoints?: number; sessions: number; type: 'stopped' }
    | { message: string; type: 'error' }
    | { proxy_url: string; type: 'started' }
    | { type: 'generating' };

  interface StresseurGeneratedFile {
    flows: string[];
    /** Relative to the generated api-tests folder, e.g. "checkout.yamlflow.yaml". */
    path: string;
    text: string;
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
      dialog: <T extends keyof Dialog>(method: T, ...options: Parameters<Dialog[T]>) => Promise<ReturnType<Dialog[T]>>;
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
      /** Bundled `stresseur record` / `generate`. */
      record?: {
        cancel: () => Promise<void>;
        onEvent: (callback: (event: StresseurRecordEvent) => void) => () => void;
        start: (url: string) => Promise<StresseurLocalResult<null>>;
        /** Stops recording and generates flows. */
        stop: () => Promise<StresseurLocalResult<StresseurGeneratedFile[]>>;
      };
    };
  }
}
