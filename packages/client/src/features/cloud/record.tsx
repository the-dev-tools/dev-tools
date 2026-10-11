import { eq } from '@tanstack/react-db';
import { useQueryClient } from '@tanstack/react-query';
import { Ulid } from 'id128';
import { useState, useSyncExternalStore } from 'react';
import { Tooltip, TooltipTrigger } from 'react-aria-components';
import { FiCircle, FiSquare, FiX } from 'react-icons/fi';
import { ImportMissingDataKind, ImportService } from '@the-dev-tools/spec/buf/api/import/v1/import_pb';
import {
  EnvironmentCollectionSchema,
  EnvironmentVariableCollectionSchema,
} from '@the-dev-tools/spec/tanstack-db/v1/api/environment';
import { Button } from '@the-dev-tools/ui/button';
import { Checkbox } from '@the-dev-tools/ui/checkbox';
import { Select, SelectItem } from '@the-dev-tools/ui/select';
import { Spinner } from '@the-dev-tools/ui/spinner';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextInputField } from '@the-dev-tools/ui/text-field';
import { useToastQueue } from '@the-dev-tools/ui/toast';
import { track } from '~/app/umami';
import { request, useApiCollection } from '~/shared/api';
import { getNextOrder, pick, queryCollection, readStored, writeStored } from '~/shared/lib';
import { routes } from '~/shared/routes';
import { CloudDialog, useDialogHost } from './dialogs';
import {
  cloudKeys,
  connectErrorText,
  pickFlow,
  recordApi,
  upsertFlowFile,
  useApiUrl,
  useCloudAccount,
  useCloudRepos,
  useNavigateToFlow,
} from './lib';

// Recording state, module-level so it survives the header remounting.

interface RepoTarget {
  apiUrl: string;
  repo: StresseurCloudRepo;
  workspaceSlug: string;
}

/** Where recorded flows go: Files, a cloud repo, or both. */
interface Destination {
  appUrl: string;
  files: boolean;
  repo?: RepoTarget;
}

const destinationKind = ({ files, repo }: Destination) => (files && repo ? 'both' : repo ? 'repo' : 'files');

type RecordState =
  | { calls: number; destination: Destination; endpoints: number; kind: 'recording' }
  | { destination: Destination; kind: 'generating' }
  | { destination: Destination; kind: 'importing' }
  | { kind: 'error'; message: string }
  | { kind: 'idle' };

let recordState: RecordState = { kind: 'idle' };
const recordListeners = new Set<() => void>();
let recordStarted = false;

const setRecordState = (state: RecordState) => {
  recordState = state;
  recordListeners.forEach((_) => void _());
};

const subscribeRecord = (listener: () => void) => {
  recordListeners.add(listener);

  const api = recordApi();
  if (!recordStarted && api) {
    recordStarted = true;
    api.onEvent((event) => {
      const state = recordState;
      if (event.type === 'progress' && state.kind === 'recording')
        setRecordState({ ...state, calls: event.calls, endpoints: event.endpoints });
      if (event.type === 'generating' && state.kind === 'recording')
        setRecordState({ destination: state.destination, kind: 'generating' });
      if (event.type === 'error' && state.kind !== 'idle') {
        track('record_failed');
        setRecordState({ kind: 'error', message: event.message });
      }
    });
  }

  return () => void recordListeners.delete(listener);
};

const useRecordState = () => useSyncExternalStore(subscribeRecord, () => recordState);

// Header control: Record when idle, live state while recording.

export const RecordControl = () => {
  const dialogs = useDialogHost();
  const state = useRecordState();
  const stop = useStopRecording();

  if (!recordApi()) return null;

  const text = tw`flex items-center gap-2 text-xs/4 font-normal tracking-tight text-on-inverse`;

  if (state.kind === 'recording')
    return (
      <div className={text}>
        <FiCircle className={tw`size-2.5 animate-pulse fill-danger stroke-danger`} />
        <span>{state.calls === 1 ? '1 call' : `${state.calls} calls`}</span>
        <Button className={tw`gap-1 px-1.5 py-0.5 text-xs/4`} onPress={() => void stop(state)} variant='ghost dark'>
          <FiSquare className={tw`size-3`} />
          Stop
        </Button>
        <Button
          aria-label='Cancel recording'
          className={tw`p-0.5`}
          onPress={() =>
            void recordApi()
              ?.cancel()
              .then(() => void setRecordState({ kind: 'idle' }))
          }
          variant='ghost dark'
        >
          <FiX className={tw`size-3`} />
        </Button>
      </div>
    );

  if (state.kind === 'generating' || state.kind === 'importing')
    return (
      <div className={text}>
        <Spinner />
        <span>{state.kind === 'generating' ? 'Generating flows' : 'Importing'}</span>
      </div>
    );

  return (
    <div className={text}>
      {dialogs.render}

      {state.kind === 'error' && (
        <>
          <span className={tw`max-w-72 truncate text-danger`}>{state.message}</span>
          <Button
            aria-label='Dismiss'
            className={tw`p-0.5`}
            onPress={() => void setRecordState({ kind: 'idle' })}
            variant='ghost dark'
          >
            <FiX className={tw`size-3`} />
          </Button>
        </>
      )}

      <TooltipTrigger delay={750}>
        <Button
          className={tw`gap-1.5 px-2 py-1 text-xs/4 font-semibold`}
          onPress={() => void dialogs.show((close) => <RecordDialog close={close} />)}
          variant='ghost dark'
        >
          <FiCircle className={tw`size-2.5 fill-danger stroke-danger`} />
          Record
        </Button>
        <Tooltip className={tw`rounded-md bg-inverse px-2 py-1 text-xs text-on-inverse`}>
          Record your app and get flows
        </Tooltip>
      </TooltipTrigger>
    </div>
  );
};

const lastUrlKey = (workspaceId: Uint8Array) => `stresseur.record.url.${Ulid.construct(workspaceId).toCanonical()}`;

/** Last cloud repo slug, or empty when cloud was off. */
const lastRepoKey = (workspaceId: Uint8Array) => `stresseur.record.repo.${Ulid.construct(workspaceId).toCanonical()}`;

const RecordDialog = ({ close }: { close: () => void }) => {
  const account = useCloudAccount();
  const repos = useCloudRepos();
  const apiUrl = useApiUrl();
  const { workspaceId } = routes.dashboard.workspace.route.useLoaderData();

  const [url, setUrl] = useState(() => readStored(lastUrlKey(workspaceId)) ?? 'http://localhost:3000');
  const [lastRepo] = useState(() => readStored(lastRepoKey(workspaceId)) ?? '');
  const [toFiles, setToFiles] = useState(true);
  const [toCloud, setToCloud] = useState(lastRepo !== '');
  const [repoSlug, setRepoSlug] = useState(lastRepo);
  const [error, setError] = useState<string>();
  const [isPending, setIsPending] = useState(false);

  // Signed out or no repos: Files only, no options.
  const cloud = account.kind === 'signed-in' && repos.data?.ok ? repos.data.data : undefined;
  const choices = cloud && apiUrl ? cloud.repos : [];
  const repo = choices.find((_) => _.slug === repoSlug) ?? choices[0];
  const cloudOn = toCloud && repo !== undefined;
  const filesOn = toFiles || choices.length === 0;

  const start = async () => {
    const api = recordApi();
    if (!api) return;

    const target: Destination = {
      appUrl: url.trim(),
      files: filesOn,
      ...(cloudOn && cloud && apiUrl && { repo: { apiUrl, repo, workspaceSlug: cloud.workspace.slug } }),
    };

    setIsPending(true);
    const result = await api.start(url.trim());
    setIsPending(false);

    if (!result.ok) {
      track('record_failed');
      setError(result.message);
      return;
    }

    writeStored(lastUrlKey(workspaceId), url.trim());
    if (choices.length > 0) writeStored(lastRepoKey(workspaceId), target.repo?.repo.slug ?? '');
    track('record_start', { destination: destinationKind(target) });
    setRecordState({ calls: 0, destination: target, endpoints: 0, kind: 'recording' });
    close();
  };

  return (
    <CloudDialog
      actions={
        <>
          <Button onPress={close}>Cancel</Button>
          <Button
            isDisabled={!url.trim() || (!filesOn && !cloudOn)}
            isPending={isPending} onPress={() => void start()} variant='primary'>
            Record
          </Button>
        </>
      }
      onClose={close}
      title='Record'
    >
      <TextInputField label='App URL' onChange={setUrl} value={url} />

      {choices.length > 0 && (
        <div className={tw`flex flex-col gap-2`}>
          <span className={tw`text-sm/5 font-medium text-on-neutral`}>Save flows to</span>

          <Checkbox isSelected={toFiles} onChange={setToFiles}>
            Files (this computer)
          </Checkbox>

          <div className={tw`flex items-center gap-3`}>
            <Checkbox className={tw`shrink-0`} isSelected={toCloud} onChange={setToCloud}>
              Cloud repo
            </Checkbox>

            {toCloud && repo && (
              <Select
                aria-label='Cloud repo'
                className={tw`min-w-0 flex-1`}
                items={choices.map((_) => ({ id: _.slug, name: _.full_name }))}
                onChange={(key) => void (typeof key === 'string' && setRepoSlug(key))}
                value={repo.slug}
              >
                {(_) => (
                  <SelectItem id={_.id} textValue={_.name}>
                    {_.name}
                  </SelectItem>
                )}
              </Select>
            )}
          </div>
        </div>
      )}

      {error && <p className={tw`text-danger`}>{error}</p>}
    </CloudDialog>
  );
};

const useStopRecording = () => {
  const { transport } = routes.root.useRouteContext();
  const { workspaceId } = routes.dashboard.workspace.route.useLoaderData();
  const queryClient = useQueryClient();
  const toastQueue = useToastQueue();
  const navigateToFlow = useNavigateToFlow();
  const environmentCollection = useApiCollection(EnvironmentCollectionSchema);
  const variableCollection = useApiCollection(EnvironmentVariableCollectionSchema);

  // Recorded flows call {{ #env:APP_URL }}. Locally that resolves to a Global Environment
  // variable of the same name, so add it when it is missing (never overwrite one).
  const ensureAppUrl = async (appUrl: string) => {
    const globalEnv = (
      await queryCollection((_) =>
        _.from({ environment: environmentCollection })
          .where((_) => eq(_.environment.workspaceId, workspaceId))
          .where((_) => eq(_.environment.isGlobal, true))
          .select((_) => pick(_.environment, 'environmentId')),
      )
    )[0];
    if (!globalEnv) return;

    const existing = await queryCollection((_) =>
      _.from({ variable: variableCollection })
        .where((_) => eq(_.variable.environmentId, globalEnv.environmentId))
        .where((_) => eq(_.variable.key, 'APP_URL'))
        .select((_) => pick(_.variable, 'environmentVariableId')),
    );
    if (existing.length > 0) return;

    variableCollection.utils.insert({
      enabled: true,
      environmentId: globalEnv.environmentId,
      environmentVariableId: Ulid.generate().bytes,
      key: 'APP_URL',
      order: await getNextOrder(variableCollection),
      value: appUrl,
    });
  };

  const importLocal = async (files: StresseurGeneratedFile[]) => {
    let first: Uint8Array | undefined;
    for (const file of files) {
      const input = { name: file.path, textData: file.text, workspaceId };
      let { message } = await request({ input, method: ImportService.method.import, transport });
      if (message.missingData === ImportMissingDataKind.DOMAIN)
        ({ message } = await request({
          input: { ...input, domainData: message.domains.map((domain) => ({ domain, enabled: true })) },
          method: ImportService.method.import,
          transport,
        }));
      first ??= message.flowId;
    }
    return first;
  };

  const importRepo = async (destination: RepoTarget, files: StresseurGeneratedFile[]) => {
    let first: Uint8Array | undefined;
    for (const file of files) {
      const result = await upsertFlowFile(transport, workspaceId, {
        apiUrl: destination.apiUrl,
        cloudWorkspaceSlug: destination.workspaceSlug,
        path: `api-tests/${file.path}`,
        repoName: destination.repo.full_name,
        repoSlug: destination.repo.slug,
        sha: destination.repo.head_sha ?? '',
        yamlText: file.text,
      });
      if (result.kind === 'dirty') throw new Error(`api-tests/${file.path} has unsaved edits`);
      if (result.kind === 'error') throw new Error(result.message);
      first ??= pickFlow(result.link)?.flowId;
    }
    await queryClient.invalidateQueries({ queryKey: cloudKeys.all });
    return first;
  };

  return async (state: Extract<RecordState, { kind: 'recording' }>) => {
    const api = recordApi();
    if (!api) return;

    const { destination } = state;
    track('record_stop', { calls: state.calls });
    setRecordState({ destination, kind: 'generating' });

    const result = await api.stop();
    if (!result.ok) {
      track('record_failed');
      setRecordState({ kind: 'error', message: result.message });
      return;
    }

    const files = result.data;
    const flows = files.reduce((sum, _) => sum + _.flows.length, 0);
    track('record_generated', { destination: destinationKind(destination), files: files.length, flows });

    if (files.length === 0) {
      setRecordState({ kind: 'error', message: 'No flows recorded' });
      return;
    }

    setRecordState({ destination, kind: 'importing' });

    try {
      // Files first: when both are on, open the Files copy.
      const localFlowId = destination.files ? await importLocal(files) : undefined;
      if (destination.files) await ensureAppUrl(destination.appUrl);
      const repoFlowId = destination.repo ? await importRepo(destination.repo, files) : undefined;
      setRecordState({ kind: 'idle' });

      const where = [destination.files && 'Files', destination.repo?.repo.full_name].filter(Boolean).join(' and ');
      const count = flows === 1 ? '1 flow recorded' : `${flows} flows recorded`;
      toastQueue.add({ title: `${count} · ${where}` }, { timeout: 5000 });

      const flowId = localFlowId ?? repoFlowId;
      if (flowId) await navigateToFlow(flowId);
    } catch (error) {
      track('record_failed');
      setRecordState({ kind: 'error', message: connectErrorText(error) });
    }
  };
};
