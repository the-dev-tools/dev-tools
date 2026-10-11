import { Code, ConnectError, Transport } from '@connectrpc/connect';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate, useRouter } from '@tanstack/react-router';
import { idEqual, Ulid } from 'id128';
import { useSyncExternalStore } from 'react';
import { CloudLink, CloudLinkKind, CloudLinkService } from '@the-dev-tools/spec/buf/api/cloud_link/v1/cloud_link_pb';
import { request } from '~/shared/api';
import { routes } from '~/shared/routes';

// Desktop APIs. All of them are missing outside the desktop app.

const electron = () =>
  typeof window === 'undefined' ? undefined : (window.electron as undefined | Window['electron']);

export const cloudApi = () => electron()?.cloud;
export const accountApi = () => electron()?.account;
export const localCloneApi = () => electron()?.localClone;
export const recordApi = () => electron()?.record;

// Account

export type CloudAccountState =
  | { account: StresseurAccount; kind: 'signed-in' }
  | { kind: 'loading' }
  | { kind: 'signed-out' };

/** Fired by the header account button after sign out, so the Cloud section follows. */
export const accountChangedEvent = 'stresseur-account-changed';

let accountState: CloudAccountState = { kind: 'loading' };
const accountListeners = new Set<() => void>();
let accountStarted = false;

const setAccountState = (state: CloudAccountState) => {
  accountState = state;
  accountListeners.forEach((_) => void _());
};

const refreshAccount = async () => {
  const api = accountApi();
  if (!api) return;
  const account = await api.get();
  setAccountState(account ? { account, kind: 'signed-in' } : { kind: 'signed-out' });
};

const subscribeAccount = (listener: () => void) => {
  accountListeners.add(listener);

  const api = accountApi();
  if (!accountStarted && api) {
    accountStarted = true;
    void refreshAccount();
    api.onEvent((event) => {
      if (event.kind === 'signed-in') setAccountState({ account: event.account, kind: 'signed-in' });
    });
    window.addEventListener(accountChangedEvent, () => void refreshAccount());
  }

  return () => void accountListeners.delete(listener);
};

export const useCloudAccount = () => useSyncExternalStore(subscribeAccount, () => accountState);

// Cloud queries

const signedOut: StresseurCloudResult<never> = { error: { kind: 'signed-out' }, ok: false };

export const cloudKeys = {
  all: ['stresseur-cloud'] as const,
  apiUrl: ['stresseur-cloud', 'api-url'] as const,
  clone: (repo: string) => ['stresseur-cloud', 'clone', repo] as const,
  cloneHead: (repo: string) => ['stresseur-cloud', 'clone-head', repo] as const,
  environments: (repo: string) => ['stresseur-cloud', 'environments', repo] as const,
  flows: (workspace: string, repo: string) => ['stresseur-cloud', 'flows', workspace, repo] as const,
  links: (workspace: string) => ['stresseur-cloud', 'links', workspace] as const,
  repos: (workspace: string) => ['stresseur-cloud', 'repos', workspace] as const,
  run: (id: string) => ['stresseur-cloud', 'run', id] as const,
};

const accountSlug = (state: CloudAccountState) => (state.kind === 'signed-in' ? state.account.workspace.slug : '');

export const useCloudRepos = () => {
  const account = useCloudAccount();
  const cloud = cloudApi();

  return useQuery({
    enabled: !!cloud && account.kind === 'signed-in',
    queryFn: () => (cloud ? cloud.repos() : Promise.resolve(signedOut)),
    queryKey: cloudKeys.repos(accountSlug(account)),
    staleTime: 60_000,
  });
};

export const useCloudFlows = (repo: string, enabled = true) => {
  const account = useCloudAccount();
  const cloud = cloudApi();

  return useQuery({
    enabled: enabled && !!cloud && account.kind === 'signed-in',
    queryFn: () => (cloud ? cloud.flows(repo) : Promise.resolve(signedOut)),
    queryKey: cloudKeys.flows(accountSlug(account), repo),
    staleTime: 60_000,
  });
};

export const useApiUrl = () => {
  const cloud = cloudApi();
  return useQuery({
    enabled: !!cloud,
    queryFn: () => (cloud ? cloud.apiUrl() : Promise.resolve('')),
    queryKey: cloudKeys.apiUrl,
    staleTime: Infinity,
  }).data;
};

/** Path of the linked local clone, or null. */
export const useClone = (repo: string) => {
  const api = localCloneApi();
  return useQuery({
    enabled: !!api && !!repo,
    queryFn: () => (api ? api.get(repo) : Promise.resolve(null)),
    queryKey: cloudKeys.clone(repo),
  }).data;
};

export const useCloudLinks = (refetchInterval?: number) => {
  const { transport } = routes.root.useRouteContext();
  const { workspaceId } = routes.dashboard.workspace.route.useLoaderData();

  return useQuery({
    enabled: !!cloudApi(),
    queryFn: async () => {
      const { message } = await request({
        input: { workspaceId },
        method: CloudLinkService.method.cloudLinkList,
        transport,
      });
      return message.items;
    },
    queryKey: cloudKeys.links(Ulid.construct(workspaceId).toCanonical()),
    ...(refetchInterval && { refetchInterval }),
  });
};

/** The flow-file link that holds the given flow, if any. */
export const findFlowLink = (links: CloudLink[] | undefined, flowId: Uint8Array) =>
  links?.find(
    (link) =>
      link.kind === CloudLinkKind.FLOW_FILE &&
      link.flows.some((_) => idEqual(Ulid.construct(_.flowId), Ulid.construct(flowId))),
  );

export const useInvalidateCloud = () => {
  const queryClient = useQueryClient();
  return () => queryClient.invalidateQueries({ queryKey: cloudKeys.all });
};

// Errors

export const cloudErrorText = (error: StresseurCloudError) => {
  switch (error.kind) {
    case 'error':
      return error.message;
    case 'offline':
      return 'Offline';
    case 'sign-in-again':
      return 'Sign in again';
    case 'signed-out':
      return 'Sign in to see your repos';
  }
};

export const cloudErrorReason = (error: StresseurCloudError) => (error.kind === 'error' ? error.code : error.kind);

export const connectErrorText = (error: unknown) => {
  if (error instanceof ConnectError) return error.rawMessage;
  if (error instanceof Error) return error.message;
  return String(error);
};

// Flow source

export type SourceResult =
  | { error?: StresseurCloudError; message: string; ok: false }
  | { ok: true; sha: string; source: 'clone' | 'cloud'; text: string };

/** Reads a flow file from the linked clone when there is one, otherwise from the Cloud. */
export const fetchSource = async (
  repo: Pick<StresseurCloudRepo, 'head_sha' | 'slug'>,
  path: string,
): Promise<SourceResult> => {
  const clone = localCloneApi();
  const cloud = cloudApi();

  const dir = clone ? await clone.get(repo.slug) : null;
  if (clone && dir) {
    const [text, head] = await Promise.all([clone.read(repo.slug, path), clone.head(repo.slug)]);
    if (!text.ok) return { message: text.message, ok: false };
    return { ok: true, sha: head.ok ? head.data : (repo.head_sha ?? ''), source: 'clone', text: text.data };
  }

  if (!cloud) return { message: 'Sign in to see your repos', ok: false };
  const file = await cloud.file(repo.slug, path);
  if (!file.ok) return { error: file.error, message: cloudErrorText(file.error), ok: false };
  // commit_sha is null until the repo's first scan.
  return { ok: true, sha: file.data.commit_sha ?? repo.head_sha ?? '', source: 'cloud', text: file.data.text };
};

// Upsert

export interface UpsertInput {
  apiUrl: string;
  cloudWorkspaceSlug: string;
  force?: boolean;
  path: string;
  repoName: string;
  repoSlug: string;
  sha: string;
  yamlText: string;
}

export type UpsertResult = { kind: 'dirty' } | { kind: 'error'; message: string } | { kind: 'ok'; link: CloudLink };

export const upsertFlowFile = async (
  transport: Transport,
  workspaceId: Uint8Array,
  input: UpsertInput,
): Promise<UpsertResult> => {
  try {
    const { message } = await request({
      input: { ...input, workspaceId },
      method: CloudLinkService.method.cloudLinkUpsert,
      transport,
    });
    if (!message.link) return { kind: 'error', message: 'No flows in this file' };
    return { kind: 'ok', link: message.link };
  } catch (error) {
    if (error instanceof ConnectError && error.code === Code.FailedPrecondition) return { kind: 'dirty' };
    return { kind: 'error', message: connectErrorText(error) };
  }
};

/** Flow to open after an upsert: the one with the given name, else the first. */
export const pickFlow = (link: CloudLink, name?: string) =>
  (name === undefined ? undefined : link.flows.find((_) => _.name === name)) ?? link.flows[0];

export const useNavigateToFlow = () => {
  const router = useRouter();
  const navigate = useNavigate();

  return (flowId: Uint8Array) =>
    navigate({
      from: router.routesById[routes.dashboard.workspace.route.id].fullPath,
      params: { flowIdCan: Ulid.construct(flowId).toCanonical() },
      to: router.routesById[routes.dashboard.workspace.flow.route.id].fullPath,
    });
};

// #env: references

export const envRefs = (yaml: string) => [
  ...new Set([...yaml.matchAll(/#env:([A-Za-z_][A-Za-z0-9_]*)/g)].flatMap((_) => (_[1] ? [_[1]] : []))),
];

/** Values typed into the run prompt, kept in memory for this session only. */
export const promptedEnv = new Map<string, Record<string, string>>();

// Pinned repos

const pinnedKey = (workspace: string) => `stresseur.cloud.pinned.${workspace}`;

export const readPinned = (workspace: string): string[] => {
  try {
    const raw = localStorage.getItem(pinnedKey(workspace));
    const parsed: unknown = raw ? JSON.parse(raw) : [];
    return Array.isArray(parsed) ? parsed.filter((_): _ is string => typeof _ === 'string') : [];
  } catch {
    return [];
  }
};

export const writePinned = (workspace: string, pinned: string[]) => {
  try {
    localStorage.setItem(pinnedKey(workspace), JSON.stringify(pinned));
  } catch {
    // Pins are a convenience; losing them is fine.
  }
};

export const readFlag = (key: string, fallback: boolean) => {
  try {
    const raw = localStorage.getItem(key);
    return raw === null ? fallback : raw === 'true';
  } catch {
    return fallback;
  }
};

export const writeFlag = (key: string, value: boolean) => {
  try {
    localStorage.setItem(key, String(value));
  } catch {
    // Ignore
  }
};

export const shortSha = (sha: string) => sha.slice(0, 7);
