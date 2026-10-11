import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Ulid } from 'id128';
import { useState } from 'react';
import { MenuTrigger, Tooltip, TooltipTrigger } from 'react-aria-components';
import { FiChevronDown, FiCloud, FiRefreshCw } from 'react-icons/fi';
import { twJoin } from 'tailwind-merge';
import { CloudLink, CloudLinkService } from '@the-dev-tools/spec/buf/api/cloud_link/v1/cloud_link_pb';
import { FlowService } from '@the-dev-tools/spec/buf/api/flow/v1/flow_pb';
import { Button } from '@the-dev-tools/ui/button';
import { Menu, MenuItem } from '@the-dev-tools/ui/menu';
import { Select, SelectItem } from '@the-dev-tools/ui/select';
import { Spinner } from '@the-dev-tools/ui/spinner';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useToastQueue } from '@the-dev-tools/ui/toast';
import { track } from '~/app/umami';
import { request } from '~/shared/api';
import { routes } from '~/shared/routes';
import { CloudDialog, CloudErrorView, EnvPrompt, useDialogHost } from './dialogs';
import {
  cloudApi,
  cloudErrorReason,
  cloudKeys,
  connectErrorText,
  envRefs,
  findFlowLink,
  localCloneApi,
  promptedEnv,
  shortSha,
  useApiUrl,
  useClone,
  useCloudFlows,
  useCloudLinks,
  useCloudRepos,
} from './lib';
import { useSyncCloudFlow } from './open-flow';

const useFlowLink = (flowId: Uint8Array) => {
  const links = useCloudLinks(cloudApi() ? 5000 : undefined);
  const link = findFlowLink(links.data, flowId);
  const name = link?.flows.find(
    (_) => Ulid.construct(_.flowId).toCanonical() === Ulid.construct(flowId).toCanonical(),
  )?.name;
  return { link, links, name };
};

const exportLink = async (transport: Parameters<typeof request>[0]['transport'], link: CloudLink) => {
  const { message } = await request({
    input: { fileId: link.fileId },
    method: CloudLinkService.method.cloudLinkExport,
    transport,
  });
  return message;
};

// Run locally (used by the flow editor's Run button)

/** Runs the flow. For a linked flow it passes #env: values from the clone's .env, asking for missing ones. */
export const useRunLocally = (flowId: Uint8Array) => {
  const { transport } = routes.root.useRouteContext();
  const toastQueue = useToastQueue();
  const dialogs = useDialogHost();
  const { link } = useFlowLink(flowId);

  const run = async () => {
    if (!link) {
      track('run_local', { linked: false });
      await request({ input: { flowId }, method: FlowService.method.flowRun, transport });
      return;
    }

    let yamlText: string;
    try {
      ({ yamlText } = await exportLink(transport, link));
    } catch (error) {
      toastQueue.add({ title: connectErrorText(error) }, { timeout: 5000 });
      return;
    }

    const names = envRefs(yamlText);
    let values: Record<string, string> = {};

    const clone = localCloneApi();
    if (names.length > 0 && clone && (await clone.get(link.repoSlug))) {
      const result = await clone.envValues(link.repoSlug, names);
      if (result.ok) values = result.data;
    }

    const missing = names.filter((_) => values[_] === undefined);
    if (missing.length > 0) {
      const remembered = promptedEnv.get(link.repoSlug) ?? {};
      const entered = await dialogs.ask<Record<string, string>>((done) => (
        <EnvPrompt done={done} initial={remembered} names={missing} />
      ));
      if (!entered) return;
      promptedEnv.set(link.repoSlug, { ...remembered, ...entered });
      for (const name of missing) if (entered[name]) values[name] = entered[name];
    }

    track('run_local', { env: names.length, linked: true });
    await request({
      input: { env: Object.entries(values).map(([key, value]) => ({ key, value })), flowId },
      method: FlowService.method.flowRun,
      transport,
    });
  };

  return { isLinked: !!link, render: dialogs.render, run };
};

// Top bar controls for linked flows

export const CloudFlowControls = () => {
  const { flowId } = routes.dashboard.workspace.flow.route.useLoaderData();
  const { link, name } = useFlowLink(flowId);

  if (!cloudApi() || !link) return null;

  return <LinkedFlowControls flowName={name ?? ''} link={link} />;
};

interface LinkedFlowControlsProps {
  flowName: string;
  link: CloudLink;
}

const LinkedFlowControls = ({ flowName, link }: LinkedFlowControlsProps) => {
  const { transport } = routes.root.useRouteContext();
  const { workspaceId } = routes.dashboard.workspace.route.useLoaderData();
  const queryClient = useQueryClient();
  const toastQueue = useToastQueue();
  const dialogs = useDialogHost();
  const sync = useSyncCloudFlow(dialogs);
  const apiUrl = useApiUrl();
  const cloud = cloudApi();

  const repos = useCloudRepos();
  const repo = repos.data?.ok ? repos.data.data.repos.find((_) => _.slug === link.repoSlug) : undefined;
  const flows = useCloudFlows(link.repoSlug);
  const cloudFlow = flows.data?.ok
    ? flows.data.data.flows.find((_) => _.path === link.path && _.name === flowName)
    : undefined;

  const clone = useClone(link.repoSlug);
  const cloneHead = useQuery({
    enabled: !!clone,
    queryFn: async () => {
      const result = await localCloneApi()?.head(link.repoSlug);
      return result?.ok ? result.data : null;
    },
    queryKey: cloudKeys.cloneHead(link.repoSlug),
    refetchInterval: 10_000,
  }).data;

  const latestSha = clone ? cloneHead : repo?.head_sha;
  const isBehind = !!latestSha && !!link.baseSha && latestSha !== link.baseSha;
  const [busy, setBusy] = useState<string>();

  const invalidateLinks = () =>
    queryClient.invalidateQueries({ queryKey: cloudKeys.links(Ulid.construct(workspaceId).toCanonical()) });

  const withBusy = async (key: string, action: () => Promise<void>) => {
    setBusy(key);
    try {
      await action();
    } catch (error) {
      toastQueue.add({ title: connectErrorText(error) }, { timeout: 5000 });
    } finally {
      setBusy(undefined);
    }
  };

  const update = () =>
    withBusy('update', async () => {
      if (!apiUrl) return;
      await sync(
        {
          apiUrl,
          repo: repo ?? { ...fallbackRepo, full_name: link.repoSlug, head_sha: link.baseSha, slug: link.repoSlug },
          workspaceSlug: link.cloudWorkspaceSlug,
        },
        link.path,
        flowName,
        true,
      );
    });

  const markSaved = async (baseSha?: string) => {
    await request({
      input: { fileId: link.fileId, ...(baseSha && { baseSha }) },
      method: CloudLinkService.method.cloudLinkMarkSaved,
      transport,
    });
    await invalidateLinks();
  };

  const saveToClone = () =>
    withBusy('save', async () => {
      const api = localCloneApi();
      if (!api) return;
      const { path, yamlText } = await exportLink(transport, link);
      const written = await api.write(link.repoSlug, path || link.path, yamlText);
      if (!written.ok) {
        track('cloud_refused', { action: 'save_clone', reason: 'write_failed' });
        toastQueue.add({ title: written.message }, { timeout: 5000 });
        return;
      }
      const head = await api.head(link.repoSlug);
      await markSaved(head.ok ? head.data : undefined);
      track('cloud_save_clone');
      toastQueue.add({ title: 'Saved to your clone' }, { timeout: 4000 });
    });

  const proposeChange = () =>
    withBusy('save', async () => {
      if (!cloud) return;
      const { path, yamlText } = await exportLink(transport, link);
      const result = await cloud.proposeChange(link.repoSlug, {
        base_sha: link.baseSha,
        content: yamlText,
        path: path || link.path,
      });

      if (!result.ok) {
        track('cloud_refused', { action: 'propose_change', reason: cloudErrorReason(result.error) });
        dialogs.show((close) => (
          <CloudDialog onClose={close} title='Change not proposed'>
            <CloudErrorView apiUrl={apiUrl} error={result.error} />
          </CloudDialog>
        ));
        return;
      }

      await markSaved();
      track('cloud_propose_change', { main_moved: result.data.main_moved });

      const pr = result.data;
      dialogs.show((close) => (
        <CloudDialog
          actions={
            <Button onPress={() => void cloud.open(pr.url)} variant='primary'>
              Open pull request
            </Button>
          }
          onClose={close}
          title={`Pull request #${pr.number}`}
        >
          <p>Your change is on {pr.branch}.</p>
          {pr.main_moved && (
            <p className={tw`text-danger`}>Main has moved since you opened this flow. Check the pull request.</p>
          )}
        </CloudDialog>
      ));
    });

  const runInCloud = () =>
    withBusy('run', async () => {
      // Cloud runs the version on main, so unsaved edits can't run there.
      const fresh = await queryClient.fetchQuery({
        queryFn: async () => {
          const { message } = await request({
            input: { workspaceId },
            method: CloudLinkService.method.cloudLinkList,
            transport,
          });
          return message.items;
        },
        queryKey: cloudKeys.links(Ulid.construct(workspaceId).toCanonical()),
        staleTime: 0,
      });
      const current = fresh.find(
        (_) => Ulid.construct(_.fileId).toCanonical() === Ulid.construct(link.fileId).toCanonical(),
      );

      if (current?.dirty) {
        track('cloud_refused', { action: 'run_cloud', reason: 'dirty' });
        dialogs.show((close) => (
          <CloudDialog onClose={close} title='Save your edits first'>
            Cloud runs the version on main.
          </CloudDialog>
        ));
        return;
      }

      dialogs.show((close) => (
        <CloudRunDialog apiUrl={apiUrl} close={close} flowName={flowName} repoSlug={link.repoSlug} />
      ));
    });

  const branch = repo?.default_branch ?? 'main';

  return (
    <>
      {dialogs.render}

      <TooltipTrigger delay={500}>
        <Button
          className={tw`gap-1.5 px-1.5 py-0.5 font-mono text-xs/5 text-on-neutral-low`}
          onPress={() => void (cloudFlow ? cloud?.open(cloudFlow.url) : repo && cloud?.open(repo.url))}
          variant='ghost'
        >
          <FiCloud className={tw`size-3.5 text-accent`} />
          {branch} @ {shortSha(link.baseSha)}
          {link.dirty && <span className={tw`font-sans text-on-neutral-low`}>· edited</span>}
        </Button>
        <Tooltip className={tw`rounded-md bg-inverse px-2 py-1 text-xs text-on-inverse`}>
          {repo?.full_name ?? link.repoSlug} / {link.path}
        </Tooltip>
      </TooltipTrigger>

      {isBehind && (
        <Button
          className={tw`gap-1 px-2 py-0.5 text-sm/5`}
          isPending={busy === 'update'}
          onPress={() => void update()}
          variant='secondary'
        >
          <FiRefreshCw className={tw`size-3.5`} />
          Update
        </Button>
      )}

      <Button
        className={tw`px-2 py-0.5 text-sm/5`}
        isPending={busy === 'save'}
        onPress={() => void (clone ? saveToClone() : proposeChange())}
        variant='secondary'
      >
        {clone ? 'Save to clone' : 'Propose change'}
      </Button>

      <Button
        className={tw`px-2 py-0.5 text-sm/5`}
        isPending={busy === 'run'}
        onPress={() => void runInCloud()}
        variant='secondary'
      >
        Run in cloud
      </Button>

      <MenuTrigger>
        <Button aria-label='More cloud actions' className={tw`p-0.5`} variant='ghost'>
          <FiChevronDown className={tw`size-4 text-on-neutral-low`} />
        </Button>

        <Menu>
          {clone && <MenuItem onAction={() => void proposeChange()}>Propose change</MenuItem>}
          {cloudFlow && (
            <MenuItem
              onAction={() => {
                track('cloud_load_test_link');
                cloud?.open(loadTestUrl(cloudFlow.url));
              }}
            >
              Load test in Stresseur Cloud
            </MenuItem>
          )}
          {(cloudFlow ?? repo) && (
            <MenuItem onAction={() => void cloud?.open(cloudFlow?.url ?? repo?.url ?? '')}>
              Open in Stresseur Cloud
            </MenuItem>
          )}
        </Menu>
      </MenuTrigger>

      <div className={twJoin(tw`h-4 w-px bg-neutral`)} />
    </>
  );
};

const fallbackRepo: StresseurCloudRepo = {
  default_branch: 'main',
  flow_count: 0,
  full_name: '',
  head_sha: '',
  records_ci: true,
  slug: '',
  url: '',
};

/** Load tests stay in the web app; the flow page opens its load test form. */
const loadTestUrl = (flowUrl: string) => {
  try {
    const url = new URL(flowUrl);
    url.searchParams.set('loadtest', '1');
    return url.toString();
  } catch {
    return flowUrl;
  }
};

// Run in cloud

const terminal = new Set<StresseurCloudRun['status']>(['error', 'failed', 'passed']);

interface CloudRunDialogProps {
  apiUrl: string | undefined;
  close: () => void;
  flowName: string;
  repoSlug: string;
}

const CloudRunDialog = ({ apiUrl, close, flowName, repoSlug }: CloudRunDialogProps) => {
  const cloud = cloudApi();

  const environments = useQuery({
    enabled: !!cloud,
    queryFn: () => (cloud ? cloud.environments(repoSlug) : Promise.resolve(null)),
    queryKey: cloudKeys.environments(repoSlug),
  });

  const [environment, setEnvironment] = useState<string>();
  const [refusal, setRefusal] = useState<StresseurCloudError>();
  const [runId, setRunId] = useState<string>();
  const [isPending, setIsPending] = useState(false);

  const envList = environments.data?.ok ? environments.data.data.environments : [];
  const selected = environment ?? envList[0]?.id;

  const start = async () => {
    if (!cloud || !selected) return;
    setIsPending(true);
    const result = await cloud.startRun(repoSlug, { environment: selected, flow: flowName });
    setIsPending(false);

    if (!result.ok) {
      track('cloud_refused', { action: 'run_cloud', reason: cloudErrorReason(result.error) });
      setRefusal(result.error);
      return;
    }

    track('run_cloud');
    setRunId(result.data.run_id);
  };

  if (runId) return <CloudRunStatus close={close} flowName={flowName} runId={runId} />;

  return (
    <CloudDialog
      actions={
        <>
          <Button onPress={close}>Cancel</Button>
          <Button isDisabled={!selected} isPending={isPending} onPress={() => void start()} variant='primary'>
            Run
          </Button>
        </>
      }
      onClose={close}
      title={`Run ${flowName} in cloud`}
    >
      {environments.isPending ? (
        <Spinner />
      ) : environments.data && !environments.data.ok ? (
        <CloudErrorView apiUrl={apiUrl} error={environments.data.error} />
      ) : envList.length === 0 ? (
        <p className={tw`text-on-neutral-low`}>No environments. Add one in Stresseur Cloud.</p>
      ) : (
        <Select
          items={envList}
          label='Environment'
          onChange={(key) => void (typeof key === 'string' && setEnvironment(key))}
          value={selected ?? null}
        >
          {(_) => (
            <SelectItem id={_.id} textValue={_.name}>
              {_.name}
            </SelectItem>
          )}
        </Select>
      )}

      <p className={tw`text-sm text-on-neutral-low`}>Runs the version on main.</p>

      {refusal && <CloudErrorView apiUrl={apiUrl} error={refusal} />}
    </CloudDialog>
  );
};

interface CloudRunStatusProps {
  close: () => void;
  flowName: string;
  runId: string;
}

const CloudRunStatus = ({ close, flowName, runId }: CloudRunStatusProps) => {
  const cloud = cloudApi();

  const run = useQuery({
    enabled: !!cloud,
    queryFn: () => (cloud ? cloud.run(runId) : Promise.resolve(null)),
    queryKey: cloudKeys.run(runId),
    refetchInterval: (query) => {
      const data = query.state.data;
      return data?.ok && terminal.has(data.data.status) ? false : 2000;
    },
  });

  const data = run.data?.ok ? run.data.data : undefined;
  const status = data?.status ?? 'queued';

  return (
    <CloudDialog
      actions={
        data && (
          <Button onPress={() => void cloud?.open(data.url)} variant='primary'>
            Open in Stresseur Cloud
          </Button>
        )
      }
      onClose={close}
      title={flowName}
    >
      <div className={tw`flex items-center gap-2`}>
        {!terminal.has(status) && <Spinner />}
        <span
          className={twJoin(
            tw`font-medium capitalize`,
            status === 'passed' && tw`text-success`,
            (status === 'failed' || status === 'error') && tw`text-danger`,
          )}
        >
          {status}
        </span>
      </div>

      {run.data && !run.data.ok && <CloudErrorView error={run.data.error} />}

      {data?.summary && data.summary.steps.length > 0 && (
        <ul className={tw`flex flex-col gap-1 text-sm`}>
          {data.summary.steps.map((step, index) => (
            <li className={tw`flex gap-2`} key={`${index}:${step.name}`}>
              <span
                className={twJoin(
                  tw`w-14 shrink-0`,
                  step.status === 'passed' ? tw`text-success` : step.status === 'failed' && tw`text-danger`,
                )}
              >
                {step.status}
              </span>
              <span className={tw`flex-1 truncate`}>{step.name}</span>
              {step.duration_ms !== undefined && <span className={tw`text-on-neutral-low`}>{step.duration_ms} ms</span>}
            </li>
          ))}
        </ul>
      )}

      {data?.log_tail && (
        <pre
          className={tw`
            max-h-64 overflow-auto rounded-md bg-neutral-lower p-2 font-mono text-xs/4 whitespace-pre-wrap
            text-on-neutral
          `}
        >
          {data.log_tail}
        </pre>
      )}
    </CloudDialog>
  );
};
