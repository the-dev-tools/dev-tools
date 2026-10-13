import { ReactNode, useEffect, useState, useSyncExternalStore } from 'react';
import * as RAC from 'react-aria-components';
import {
  FiArrowLeft,
  FiCheckCircle,
  FiCircle,
  FiCopy,
  FiFolder,
  FiShoppingBag,
  FiTerminal,
  FiX,
  FiXCircle,
} from 'react-icons/fi';
import { twJoin } from 'tailwind-merge';
import { Button } from '@the-dev-tools/ui/button';
import { Modal } from '@the-dev-tools/ui/modal';
import { Radio, RadioGroup } from '@the-dev-tools/ui/radio-group';
import { Select, SelectItem } from '@the-dev-tools/ui/select';
import { Spinner } from '@the-dev-tools/ui/spinner';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextInputField } from '@the-dev-tools/ui/text-field';
import { track } from '~/app/umami';
import { readStored, writeStored } from '~/shared/lib';
import { useFinishRun, useImportFlows } from './launchpad-import';
import {
  AGENT_PROMPT,
  cancelRun,
  type Destination,
  failedStep,
  fixPrompt,
  type InitRun,
  isGreen,
  readLastUrl,
  runSteps,
  setRun,
  startRun,
  type StepState,
  stopRecording,
  useInitRun,
} from './launchpad-run';
import { cloudApi, launchpadApi, recordApi, useApiUrl, useCloudAccount, useCloudRepos, useNavigateToFlow } from './lib';

// Which launchpad view is open, module-level so the header and the overview can open it.

export type LaunchpadView = 'agent' | 'choose' | 'record' | 'run';

let view: LaunchpadView | undefined;
const viewListeners = new Set<() => void>();

export const openLaunchpad = (next: LaunchpadView) => {
  view = next;
  viewListeners.forEach((_) => void _());
};

const closeLaunchpad = () => {
  view = undefined;
  viewListeners.forEach((_) => void _());
};

const useLaunchpadView = () =>
  useSyncExternalStore(
    (listener) => {
      viewListeners.add(listener);
      return () => void viewListeners.delete(listener);
    },
    () => view,
  );

const seenKey = 'stresseur.launchpad.seen';

/** True until the launchpad has opened once (desktop first launch). */
export const isFirstLaunch = () => !!recordApi() && readStored(seenKey) === null;

/** The launchpad dialog. Opens by itself on the very first launch, never again unless chosen. */
export const LaunchpadHost = () => {
  useFinishRun();
  const current = useLaunchpadView();

  useEffect(() => {
    if (!isFirstLaunch()) return;
    writeStored(seenKey, 'true');
    track('launchpad_first_run');
    openLaunchpad('choose');
  }, []);

  if (!recordApi()) return null;

  return (
    <Modal
      className={tw`h-auto max-h-[85vh] w-full max-w-xl`}
      isDismissable
      isOpen={current !== undefined}
      onOpenChange={(isOpen) => void (!isOpen && closeLaunchpad())}
    >
      <RAC.Dialog aria-label='Get started' className={tw`flex max-h-[85vh] flex-col outline-hidden`}>
        {current === 'choose' && <ChooseView />}
        {current === 'agent' && <AgentView />}
        {current === 'record' && <RecordView />}
        {current === 'run' && <RunView />}
      </RAC.Dialog>
    </Modal>
  );
};

// Layout

interface FrameProps {
  actions?: ReactNode;
  back?: LaunchpadView | undefined;
  children: ReactNode;
  title: string;
}

const Frame = ({ actions, back, children, title }: FrameProps) => (
  <>
    <div className={tw`flex items-center gap-2 px-5 pt-4`}>
      {back && (
        <Button aria-label='Back' className={tw`-ml-1 p-1`} onPress={() => void openLaunchpad(back)} variant='ghost'>
          <FiArrowLeft className={tw`size-4 text-on-neutral-low`} />
        </Button>
      )}

      <RAC.Heading className={tw`flex-1 text-lg/6 font-semibold tracking-tight text-on-neutral`} slot='title'>
        {title}
      </RAC.Heading>

      <Button aria-label='Close' className={tw`p-1`} onPress={closeLaunchpad} variant='ghost'>
        <FiX className={tw`size-4 text-on-neutral-low`} />
      </Button>
    </div>

    <div className={tw`flex min-h-0 flex-1 flex-col gap-4 overflow-auto px-5 py-4 text-md/5 text-on-neutral`}>
      {children}
    </div>

    {actions && <div className={tw`flex justify-end gap-2 border-t border-neutral px-5 py-3`}>{actions}</div>}
  </>
);

const copy = (text: string) =>
  navigator.clipboard.writeText(text).then(
    () => true,
    () => false,
  );

const CopyButton = ({ label = 'Copy', text }: { label?: string; text: string }) => {
  const [copied, setCopied] = useState(false);
  return (
    <Button
      className={tw`gap-1.5 px-2 py-1 text-xs/4`}
      onPress={() => void copy(text).then((ok) => void setCopied(ok))}
    >
      <FiCopy className={tw`size-3`} />
      {copied ? 'Copied' : label}
    </Button>
  );
};

const CodeBlock = ({ children }: { children: string }) => (
  <pre
    className={tw`
      overflow-x-auto rounded-md border border-neutral bg-neutral-lower px-3 py-2 font-mono text-xs/5
      whitespace-pre-wrap text-on-neutral
    `}
  >
    {children}
  </pre>
);

const StateIcon = ({ state }: { state: StepState }) => {
  const box = tw`flex size-5 shrink-0 items-center justify-center`;
  if (state === 'active')
    return (
      <span className={box}>
        <Spinner />
      </span>
    );
  if (state === 'done') return <FiCheckCircle className={tw`size-5 shrink-0 text-success`} />;
  if (state === 'failed') return <FiXCircle className={tw`size-5 shrink-0 text-danger`} />;
  return <FiCircle className={tw`size-5 shrink-0 text-on-neutral-lower`} />;
};

interface StepRowProps {
  children?: ReactNode;
  command?: string;
  detail?: ReactNode;
  label: string;
  state: StepState;
}

const StepRow = ({ children, command, detail, label, state }: StepRowProps) => {
  const [showCommand, setShowCommand] = useState(false);

  return (
    <li className={tw`flex gap-3`}>
      <StateIcon state={state} />

      <div className={tw`flex min-w-0 flex-1 flex-col gap-1.5`}>
        <div className={tw`flex items-center gap-2`}>
          <span
            className={twJoin(tw`font-semibold`, state === 'pending' ? tw`text-on-neutral-low` : tw`text-on-neutral`)}
          >
            {label}
          </span>

          {command && (
            <Button
              className={tw`ml-auto px-1.5 py-0.5 text-xs/4 text-on-neutral-low`}
              onPress={() => void setShowCommand(!showCommand)}
              variant='ghost'
            >
              {showCommand ? 'Hide command' : 'Show command'}
            </Button>
          )}
        </div>

        {detail && <div className={tw`text-sm/5 wrap-break-word text-on-neutral-low`}>{detail}</div>}
        {showCommand && command && <CodeBlock>{command}</CodeBlock>}
        {children}
      </div>
    </li>
  );
};

// Choose

interface ChoiceProps {
  description: string;
  icon: ReactNode;
  onPress: () => void;
  recommended?: boolean;
  title: string;
}

const Choice = ({ description, icon, onPress, recommended, title }: ChoiceProps) => (
  <RAC.Button
    className={twJoin(
      tw`flex cursor-pointer items-start gap-3 rounded-lg border p-4 text-left transition-colors hover:bg-neutral-lower`,
      recommended ? tw`border-accent` : tw`border-neutral`,
    )}
    onPress={onPress}
  >
    <span className={tw`mt-0.5 text-xl text-on-neutral-low`}>{icon}</span>

    <span className={tw`flex flex-col gap-0.5`}>
      <span className={tw`flex items-center gap-2 font-semibold text-on-neutral`}>
        {title}
        {recommended && (
          <span className={tw`rounded-sm bg-neutral-low px-1.5 py-0.5 text-xs/4 font-medium text-on-neutral-low`}>
            Recommended
          </span>
        )}
      </span>
      <span className={tw`text-sm/5 text-on-neutral-low`}>{description}</span>
    </span>
  </RAC.Button>
);

const ChooseView = () => {
  const run = useInitRun();
  const [demoError, setDemoError] = useState<string>();
  const [demoPending, setDemoPending] = useState(false);

  return (
    <Frame title='Get started'>
      <p className={tw`text-on-neutral-low`}>API tests from the way you use your app.</p>

      <div className={tw`flex flex-col gap-3`}>
        <Choice
          description='Your coding agent records your app, writes the flows, gets them green and opens a PR.'
          icon={<FiTerminal />}
          onPress={() => {
            track('launchpad_choice', { choice: 'agent' });
            openLaunchpad('agent');
          }}
          recommended
          title='Set it up with your coding agent'
        />

        <Choice
          description='Use your app in a browser. Studio writes the flows and runs them twice.'
          icon={<FiCircle className={tw`fill-danger stroke-danger`} />}
          onPress={() => {
            track('launchpad_choice', { choice: 'record' });
            openLaunchpad(run?.status === 'running' ? 'run' : 'record');
          }}
          title='Record it myself'
        />

        <Choice
          description='The whole loop on a sample app. Nothing to set up.'
          icon={<FiShoppingBag />}
          onPress={() => {
            track('launchpad_choice', { choice: 'demo' });
            if (run?.status === 'running') return void openLaunchpad('run');
            if (demoPending) return;
            setDemoPending(true);
            void startRun({ destination: { kind: 'files' }, mode: 'demo' }).then((result) => {
              setDemoPending(false);
              if (result.ok) openLaunchpad('run');
              else setDemoError(result.message);
            });
          }}
          title='Try the sample shop'
        />
      </div>

      {demoPending && (
        <div className={tw`flex items-center gap-2 text-on-neutral-low`}>
          <Spinner /> Starting the sample shop
        </div>
      )}
      {demoError && <p className={tw`text-danger`}>{demoError}</p>}
    </Frame>
  );
};

// Coding agent

const AgentView = () => {
  const api = launchpadApi();
  const [dir, setDir] = useState<string>();
  const [skill, setSkill] = useState<{ agents: string[] }>();
  const [isPending, setIsPending] = useState(false);
  const [error, setError] = useState<string>();

  if (!api) return null;

  const pick = async () => {
    const picked = await api.pickFolder("Pick your app's repo");
    if (!picked) return;
    setDir(picked);
    setSkill(undefined);
    setError(undefined);
  };

  const install = async () => {
    if (!dir) return;
    setIsPending(true);
    const result = await api.installSkill(dir);
    setIsPending(false);
    if (!result.ok) return void setError(result.message);
    track('launchpad_skill_installed', { agents: result.data.agents.length });
    setError(undefined);
    setSkill({ agents: result.data.agents });
  };

  return (
    <Frame back='choose' title='Set it up with your coding agent'>
      <ol className={tw`flex flex-col gap-4`}>
        <StepRow detail={dir} label="Your app's repo" state={dir ? 'done' : 'active'}>
          <div>
            <Button className={tw`gap-1.5`} onPress={() => void pick()}>
              <FiFolder className={tw`size-3.5`} />
              {dir ? 'Change folder' : 'Choose folder'}
            </Button>
          </div>
        </StepRow>

        <StepRow
          command='npx stresseur install-skill'
          detail={skill && (skill.agents.length > 0 ? `Added for ${skill.agents.join(', ')}` : 'Added')}
          label='Add the Stresseur skill'
          state={skill ? 'done' : error ? 'failed' : dir ? 'active' : 'pending'}
        >
          {dir && !skill && (
            <div>
              <Button isPending={isPending} onPress={() => void install()} variant='primary'>
                Add skill
              </Button>
            </div>
          )}
          {error && <p className={tw`text-sm/5 text-danger`}>{error}</p>}
        </StepRow>

        <StepRow label='Paste this prompt' state={skill ? 'active' : 'pending'}>
          {skill && dir && <PromptActions dir={dir} prompt={AGENT_PROMPT} />}
        </StepRow>
      </ol>

      {skill && dir && <AgentProgress dir={dir} />}
    </Frame>
  );
};

const PromptActions = ({ dir, prompt }: { dir: string; prompt: string }) => {
  const api = launchpadApi();
  const [notice, setNotice] = useState<string>();

  const open = async (agent: 'claude' | 'cursor') => {
    track('launchpad_open_agent', { agent });
    const opened = await api?.openAgent(agent, dir, prompt);
    setNotice(opened ? undefined : 'Too long for a link. Copy it instead.');
  };

  return (
    <div className={tw`flex flex-col gap-2`}>
      <CodeBlock>{prompt}</CodeBlock>

      <div className={tw`flex flex-wrap gap-2`}>
        <CopyButton label='Copy prompt' text={prompt} />
        <Button className={tw`px-2 py-1 text-xs/4`} onPress={() => void open('claude')}>
          Open in Claude Code
        </Button>
        <Button className={tw`px-2 py-1 text-xs/4`} onPress={() => void open('cursor')}>
          Open in Cursor
        </Button>
      </div>

      {notice && <p className={tw`text-sm/5 text-on-neutral-low`}>{notice}</p>}
    </div>
  );
};

/** Follows the agent in the repo: api-tests/ and .stresseur/, and the PR when `gh` finds one. */
const AgentProgress = ({ dir }: { dir: string }) => {
  const [event, setEvent] = useState<StresseurRepoProgressEvent>();

  useEffect(() => {
    const api = launchpadApi();
    if (!api) return;
    const off = api.onProgress((_) => void (_.dir === dir && setEvent(_)));
    void api.watch(dir);
    return () => {
      off();
      void api.unwatch();
    };
  }, [dir]);

  const progress = event?.progress;
  const steps = event?.steps;
  const checks = progress?.checks;
  const files = progress?.flowFiles.length ?? 0;

  return (
    <div className={tw`flex flex-col gap-3 border-t border-neutral pt-4`}>
      <span className={tw`text-sm/5 font-semibold text-on-neutral-low`}>Your agent&apos;s progress</span>

      <ol className={tw`flex flex-col gap-3`}>
        <StepRow
          detail={steps?.recording === 'active' ? 'Use your app in the browser it opened' : undefined}
          label={steps?.recording === 'active' ? 'Recording…' : 'Recorded'}
          state={steps?.recording ?? 'pending'}
        />
        <StepRow
          detail={files > 0 ? `${String(files)} ${files === 1 ? 'file' : 'files'} in api-tests/` : undefined}
          label='Flows generated'
          state={steps?.generated ?? 'pending'}
        />
        <StepRow
          detail={
            checks &&
            `${String(checks.passedFlows)}/${String(checks.flows)} flows passing · ${String(checks.passed)}/${String(checks.total)} steps`
          }
          label='Checks green'
          state={steps?.checks ?? 'pending'}
        />
        {progress?.pr && (
          <StepRow
            detail={
              <Button
                className={tw`p-0 text-sm/5 text-accent underline`}
                onPress={() => void (progress.pr && cloudApi()?.open(progress.pr.url))}
                variant='ghost'
              >
                {progress.pr.url}
              </Button>
            }
            label='PR opened'
            state='done'
          />
        )}
      </ol>
    </div>
  );
};

// Record it myself

type DestinationKind = Destination['kind'];

const RecordView = () => {
  const api = launchpadApi();
  const account = useCloudAccount();
  const repos = useCloudRepos();
  const apiUrl = useApiUrl();

  const [url, setUrl] = useState(() => readLastUrl() ?? '');
  const [servers, setServers] = useState<StresseurDevServer[]>();
  const [kind, setKind] = useState<DestinationKind>('files');
  const [dir, setDir] = useState<string>();
  const [repoSlug, setRepoSlug] = useState<string>();
  const [error, setError] = useState<string>();
  const [isPending, setIsPending] = useState(false);

  useEffect(() => {
    let live = true;
    void launchpadApi()
      ?.detect()
      .then((found) => {
        if (!live) return;
        setServers(found);
        const first = found[0];
        if (first) setUrl((_) => (_ ? _ : first.url));
      });
    return () => void (live = false);
  }, []);

  const cloud = account.kind === 'signed-in' && repos.data?.ok ? repos.data.data : undefined;
  const choices = cloud && apiUrl ? cloud.repos : [];
  const repo = choices.find((_) => _.slug === repoSlug) ?? choices[0];

  const destination = ((): Destination | undefined => {
    if (kind === 'files') return { kind: 'files' };
    if (kind === 'folder') return dir ? { dir, kind: 'folder' } : undefined;
    if (cloud && apiUrl && repo) return { kind: 'cloud', repo: { apiUrl, repo, workspaceSlug: cloud.workspace.slug } };
    return undefined;
  })();

  const start = async () => {
    if (!destination) return;
    setIsPending(true);
    const result = await startRun({ destination, mode: 'record', url: url.trim() });
    setIsPending(false);
    if (!result.ok) return void setError(result.message);
    openLaunchpad('run');
  };

  return (
    <Frame
      actions={
        <>
          <Button onPress={closeLaunchpad}>Cancel</Button>
          <Button
            isDisabled={!url.trim() || !destination}
            isPending={isPending}
            onPress={() => void start()}
            variant='primary'
          >
            Record
          </Button>
        </>
      }
      back='choose'
      title='Record it myself'
    >
      <div className={tw`flex flex-col gap-2`}>
        <TextInputField label='App URL' onChange={setUrl} placeholder='http://localhost:3000' value={url} />

        {servers === undefined ? (
          <span className={tw`text-sm/5 text-on-neutral-low`}>Looking for running dev servers…</span>
        ) : servers.length === 0 ? (
          <span className={tw`text-sm/5 text-on-neutral-low`}>
            No dev server on 3000, 5173, 4321, 8000, 5000 or 8080. Start yours, or enter its URL.
          </span>
        ) : (
          <div className={tw`flex flex-wrap items-center gap-1.5`}>
            <span className={tw`text-sm/5 text-on-neutral-low`}>Running:</span>
            {servers.map((server) => (
              <Button
                className={twJoin(tw`px-2 py-0.5 text-xs/4`, url === server.url && tw`border-accent`)}
                key={server.port}
                onPress={() => void setUrl(server.url)}
              >
                {`localhost:${String(server.port)}`}
                {server.title && <span className={tw`text-on-neutral-low`}>{` · ${server.title}`}</span>}
              </Button>
            ))}
          </div>
        )}
      </div>

      <RadioGroup label='Save flows to' onChange={(value) => void setKind(value as DestinationKind)} value={kind}>
        <Radio value='files'>Files (this computer)</Radio>

        <div className={tw`flex items-center gap-3`}>
          <Radio value='folder'>A repo folder</Radio>
          {kind === 'folder' && (
            <Button
              className={tw`min-w-0 gap-1.5 px-2 py-0.5 text-xs/4`}
              onPress={() =>
                void api?.pickFolder("Pick your app's repo").then((picked) => void (picked && setDir(picked)))
              }
            >
              <FiFolder className={tw`size-3 shrink-0`} />
              <span className={tw`truncate`}>{dir ?? 'Choose folder'}</span>
            </Button>
          )}
        </div>

        {choices.length > 0 && (
          <div className={tw`flex items-center gap-3`}>
            <Radio value='cloud'>Cloud repo</Radio>
            {kind === 'cloud' && repo && (
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
        )}
      </RadioGroup>

      {error && <p className={tw`text-danger`}>{error}</p>}
    </Frame>
  );
};

// A run's steps

const RunView = () => {
  const run = useInitRun();

  if (!run)
    return (
      <Frame back='choose' title='Record it myself'>
        <p className={tw`text-on-neutral-low`}>Nothing running.</p>
      </Frame>
    );

  const steps = runSteps(run);
  const running = run.status === 'running';
  const failed = failedStep(run);
  const isRecording = running && !!run.recording && !run.recorded;

  return (
    <Frame
      actions={
        running ? (
          <>
            <Button onPress={() => void cancelRun()}>Cancel</Button>
            {isRecording && (
              <Button onPress={() => void stopRecording()} variant='primary'>
                Done
              </Button>
            )}
          </>
        ) : (
          <>
            <Button
              onPress={() => {
                setRun(undefined);
                openLaunchpad('record');
              }}
            >
              Record again
            </Button>
            <Button onPress={closeLaunchpad} variant='primary'>
              Close
            </Button>
          </>
        )
      }
      back={running ? undefined : 'choose'}
      title={run.mode === 'demo' ? 'Sample shop' : 'Record it myself'}
    >
      <ol className={tw`flex flex-col gap-4`}>
        {steps.map((step) => (
          <StepRow command={step.command} detail={step.detail} key={step.key} label={step.label} state={step.state}>
            {step.state === 'failed' && run.error && step.key === failed?.key && (
              <p className={tw`text-sm/5 text-danger`}>
                {run.error.message}
                {run.error.hint && <span className={tw`text-on-neutral-low`}>{` ${run.error.hint}`}</span>}
              </p>
            )}
            {step.failures && (
              <ul className={tw`flex flex-col gap-1 text-sm/5`}>
                {step.failures.map((_) => (
                  <li key={`${_.flow}/${_.step}`}>
                    <span className={tw`font-medium`}>{`${_.flow} › ${_.step}: `}</span>
                    <span className={tw`text-danger`}>{_.reason}</span>
                    {_.hint && <span className={tw`text-on-neutral-low`}>{` ${_.hint}`}</span>}
                  </li>
                ))}
              </ul>
            )}
          </StepRow>
        ))}
      </ol>

      {!running && failed && <FixWithAgent run={run} />}
      {!running && <RunEnd run={run} />}
    </Frame>
  );
};

/** After the run: where the flows went, and the CI check / PR for a repo folder. */
const RunEnd = ({ run }: { run: InitRun }) => {
  const navigateToFlow = useNavigateToFlow();
  const importFlows = useImportFlows();

  if (run.status === 'importing')
    return (
      <div className={tw`flex items-center gap-2 text-on-neutral-low`}>
        <Spinner /> Opening in Studio
      </div>
    );

  if (run.importError) return <p className={tw`text-danger`}>{run.importError}</p>;

  const { flowId } = run.imported ?? {};

  return (
    <div className={tw`flex flex-col gap-3 border-t border-neutral pt-4`}>
      {run.imported && (
        <div className={tw`flex items-center gap-3`}>
          <span className={tw`flex-1 text-on-neutral-low`}>{`Saved to ${run.imported.where}`}</span>
          {flowId && (
            <Button
              onPress={() => {
                closeLaunchpad();
                void navigateToFlow(flowId);
              }}
            >
              Open flow
            </Button>
          )}
        </div>
      )}

      {run.destination.kind === 'folder' && (run.finished?.files.length ?? 0) > 0 && (
        <RepoActions dir={run.destination.dir} green={isGreen(run)} onOpen={() => void importFlows(run, 'files')} />
      )}
    </div>
  );
};

const RepoActions = ({ dir, green, onOpen }: { dir: string; green: boolean; onOpen: () => void }) => {
  const api = launchpadApi();
  const [ci, setCi] = useState<{ message: string; ok: boolean }>();
  const [pr, setPr] = useState<{ commands?: string[]; message?: string; url?: string }>();
  const [pending, setPending] = useState<'ci' | 'pr'>();

  const addCi = async () => {
    if (!api) return;
    setPending('ci');
    const result = await api.addCi(dir);
    setPending(undefined);
    track('launchpad_add_ci', { ok: result.ok });
    setCi(
      result.ok
        ? { message: result.data.output || 'CI check added', ok: true }
        : { message: result.message, ok: false },
    );
  };

  const openPr = async () => {
    if (!api) return;
    setPending('pr');
    const result = await api.openPr(dir);
    setPending(undefined);
    track('launchpad_open_pr', { ok: result.ok });
    if (!result.ok) return void setPr({ message: result.message });
    setPr(result.data);
    if (result.data.url) cloudApi()?.open(result.data.url);
  };

  return (
    <div className={tw`flex flex-col gap-2`}>
      <span className={tw`text-on-neutral-low`}>{`Flows are in ${dir}/api-tests`}</span>

      <div className={tw`flex flex-wrap gap-2`}>
        <Button onPress={onOpen}>Open in Studio</Button>
        <Button isDisabled={ci?.ok === true} isPending={pending === 'ci'} onPress={() => void addCi()}>
          Add CI check
        </Button>
        <Button
          isDisabled={!!pr?.url}
          isPending={pending === 'pr'}
          onPress={() => void openPr()}
          variant={green ? 'primary' : 'secondary'}
        >
          Open PR
        </Button>
      </div>

      {ci && <p className={twJoin(tw`text-sm/5`, ci.ok ? tw`text-on-neutral-low` : tw`text-danger`)}>{ci.message}</p>}
      {pr?.message && <p className={tw`text-sm/5 text-danger`}>{pr.message}</p>}
      {pr?.url && <p className={tw`text-sm/5 text-on-neutral-low`}>{`PR opened: ${pr.url}`}</p>}
      {pr?.commands && (
        <div className={tw`flex flex-col gap-1.5`}>
          <span className={tw`text-sm/5 text-on-neutral-low`}>No gh here. Run:</span>
          <CodeBlock>{pr.commands.join('\n')}</CodeBlock>
          <div>
            <CopyButton text={pr.commands.join('\n')} />
          </div>
        </div>
      )}
    </div>
  );
};

/** Installs the skill where the flows are (or in a repo the user picks) and hands over a ready prompt. */
const FixWithAgent = ({ run }: { run: InitRun }) => {
  const api = launchpadApi();
  const known =
    run.destination.kind === 'folder'
      ? run.destination.dir
      : run.finished?.temporary && run.finished.files.length > 0
        ? run.finished.dir
        : undefined;
  const [dir, setDir] = useState(known);
  const [agents, setAgents] = useState<string[]>();
  const [error, setError] = useState<string>();
  const [isPending, setIsPending] = useState(false);
  const prompt = fixPrompt(run);

  const fix = async () => {
    if (!api) return;
    const target = dir ?? (await api.pickFolder("Pick your app's repo"));
    if (!target) return;
    setDir(target);
    setIsPending(true);
    const result = await api.installSkill(target);
    setIsPending(false);
    track('launchpad_fix_with_agent', { ok: result.ok });
    if (!result.ok) return void setError(result.message);
    setError(undefined);
    setAgents(result.data.agents);
    await copy(prompt);
  };

  return (
    <div className={tw`flex flex-col gap-2 rounded-lg border border-neutral p-3`}>
      {agents && dir ? (
        <>
          <span className={tw`text-sm/5 text-on-neutral-low`}>
            {`Skill added${agents.length > 0 ? ` for ${agents.join(', ')}` : ''}. Prompt copied:`}
          </span>
          <PromptActions dir={dir} prompt={prompt} />
        </>
      ) : (
        <div className={tw`flex items-center gap-3`}>
          <span className={tw`flex-1 text-sm/5 text-on-neutral-low`}>
            Your coding agent can fix this. Studio adds the skill and copies a prompt with the failure.
          </span>
          <Button isPending={isPending} onPress={() => void fix()} variant='primary'>
            Fix with your coding agent
          </Button>
        </div>
      )}
      {error && <p className={tw`text-sm/5 text-danger`}>{error}</p>}
    </div>
  );
};
