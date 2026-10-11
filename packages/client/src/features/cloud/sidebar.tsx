import { useQueryClient } from '@tanstack/react-query';
import { Ulid } from 'id128';
import { ReactNode, useEffect, useState } from 'react';
import { MenuTrigger, Text, Tree } from 'react-aria-components';
import { FiChevronDown, FiChevronRight, FiCloud, FiFileText, FiHardDrive, FiMoreHorizontal } from 'react-icons/fi';
import { TbPin } from 'react-icons/tb';
import { twJoin } from 'tailwind-merge';
import { CloudLinkService } from '@the-dev-tools/spec/buf/api/cloud_link/v1/cloud_link_pb';
import { Button } from '@the-dev-tools/ui/button';
import { FlowsIcon } from '@the-dev-tools/ui/icons';
import { Menu, MenuItem, useContextMenuState } from '@the-dev-tools/ui/menu';
import { Spinner } from '@the-dev-tools/ui/spinner';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextInputField } from '@the-dev-tools/ui/text-field';
import { useToastQueue } from '@the-dev-tools/ui/toast';
import { TreeItem } from '@the-dev-tools/ui/tree';
import { track } from '~/app/umami';
import { request } from '~/shared/api';
import { routes } from '~/shared/routes';
import { useDialogHost } from './dialogs';
import {
  accountApi,
  cloudApi,
  cloudErrorText,
  cloudKeys,
  connectErrorText,
  localCloneApi,
  readFlag,
  readPinned,
  useApiUrl,
  useClone,
  useCloudAccount,
  useCloudFlows,
  useCloudRepos,
  writeFlag,
  writePinned,
} from './lib';
import { CloudTarget, useSyncCloudFlow } from './open-flow';
import { RecordButton } from './record';

const openKey = 'stresseur.cloud.open';

/** Sidebar section below Files. Renders only in the desktop app. */
export const CloudSection = () => (cloudApi() ? <CloudSectionInner /> : null);

const CloudSectionInner = () => {
  const account = useCloudAccount();
  const repos = useCloudRepos();
  const [isOpen, setIsOpen] = useState(() => readFlag(openKey, true));

  useEffect(() => {
    if (isOpen) track('cloud_section_opened');
  }, [isOpen]);

  const workspace = account.kind === 'signed-in' ? account.account.workspace : undefined;
  const offline = repos.data?.ok === true && repos.data.offline === true;

  return (
    <div className={tw`flex flex-col gap-1`}>
      <div className={tw`flex items-center gap-2 px-2.5 py-1.5`}>
        <Button
          aria-label={isOpen ? 'Collapse Cloud' : 'Expand Cloud'}
          className={tw`flex min-w-0 flex-1 items-center justify-start gap-2 p-0`}
          onPress={() => {
            writeFlag(openKey, !isOpen);
            setIsOpen(!isOpen);
          }}
          variant='ghost'
        >
          <FiCloud className={tw`size-5 shrink-0 text-on-neutral-low`} />
          <h2 className={tw`text-md/5 font-semibold tracking-tight text-on-neutral`}>Cloud</h2>
          {workspace && (
            <span className={tw`truncate text-sm/5 font-normal text-on-neutral-low`}>{workspace.name}</span>
          )}
          {isOpen ? (
            <FiChevronDown className={tw`size-3.5 shrink-0 text-on-neutral-low`} />
          ) : (
            <FiChevronRight className={tw`size-3.5 shrink-0 text-on-neutral-low`} />
          )}
        </Button>

        {offline && <span className={tw`text-xs text-on-neutral-low`}>Offline</span>}

        <RecordButton />

        {workspace && (
          <MenuTrigger>
            <Button aria-label='Cloud options' className={tw`p-0.5`} variant='ghost'>
              <FiMoreHorizontal className={tw`size-4 text-on-neutral-low`} />
            </Button>

            <Menu>
              <MenuItem onAction={() => void accountApi()?.signIn()}>Switch workspace</MenuItem>
              <MenuItem onAction={() => void repos.refetch()}>Refresh</MenuItem>
            </Menu>
          </MenuTrigger>
        )}
      </div>

      {isOpen && <CloudBody />}
    </div>
  );
};

const StateLine = ({ children }: { children: ReactNode }) => (
  <div className={tw`px-3 py-1 text-sm/5 text-on-neutral-low`}>{children}</div>
);

const LinkButton = ({ children, onPress }: { children: ReactNode; onPress: () => void }) => (
  <Button className={tw`justify-start px-3 py-1 text-sm/5 font-normal text-accent`} onPress={onPress} variant='ghost'>
    {children}
  </Button>
);

const CloudBody = () => {
  const account = useCloudAccount();
  const repos = useCloudRepos();
  const apiUrl = useApiUrl();
  const signIn = () => void accountApi()?.signIn();

  if (account.kind === 'loading') return null;
  if (account.kind === 'signed-out') return <LinkButton onPress={signIn}>Sign in to see your repos</LinkButton>;

  if (repos.isPending)
    return (
      <StateLine>
        <Spinner />
      </StateLine>
    );

  if (!repos.data) return <StateLine>{repos.error.message}</StateLine>;

  if (!repos.data.ok) {
    const { error } = repos.data;
    if (error.kind === 'signed-out') return <LinkButton onPress={signIn}>Sign in to see your repos</LinkButton>;
    if (error.kind === 'sign-in-again') return <LinkButton onPress={signIn}>Sign in again</LinkButton>;
    if (error.kind === 'error' && error.fixUrl) {
      const { fixUrl } = error;
      return <LinkButton onPress={() => void cloudApi()?.open(fixUrl)}>{error.message}</LinkButton>;
    }
    return <StateLine>{cloudErrorText(error)}</StateLine>;
  }

  const { repos: items, workspace } = repos.data.data;

  if (items.length === 0)
    return (
      <LinkButton onPress={() => void (apiUrl && cloudApi()?.open(apiUrl))}>
        Connect a repo in Stresseur Cloud
      </LinkButton>
    );

  if (!apiUrl) return null;

  return <RepoTree apiUrl={apiUrl} repos={items} workspaceSlug={workspace.slug} />;
};

interface RepoTreeProps {
  apiUrl: string;
  repos: StresseurCloudRepo[];
  workspaceSlug: string;
}

const RepoTree = ({ apiUrl, repos, workspaceSlug }: RepoTreeProps) => {
  const queryClient = useQueryClient();
  const dialogs = useDialogHost();
  const sync = useSyncCloudFlow(dialogs);

  const [filter, setFilter] = useState('');
  const [pinned, setPinned] = useState(() => readPinned(workspaceSlug));
  const [opening, setOpening] = useState<string>();

  const togglePin = (slug: string) => {
    const next = pinned.includes(slug) ? pinned.filter((_) => _ !== slug) : [...pinned, slug];
    writePinned(workspaceSlug, next);
    setPinned(next);
  };

  const query = filter.trim().toLowerCase();

  const matchesFlow = (flow: StresseurCloudFlow) =>
    flow.name.toLowerCase().includes(query) || flow.path.toLowerCase().includes(query);

  const visible = repos
    .filter((repo) => {
      if (!query || repo.full_name.toLowerCase().includes(query)) return true;
      const flows = queryClient.getQueryData<StresseurCloudResult<StresseurCloudFlows>>(
        cloudKeys.flows(workspaceSlug, repo.slug),
      );
      return flows?.ok === true && flows.data.flows.some(matchesFlow);
    })
    .sort((a, b) => Number(pinned.includes(b.slug)) - Number(pinned.includes(a.slug)))
    .map((repo) => ({ id: repo.slug, repo }));

  const open = async (repo: StresseurCloudRepo, flow: StresseurCloudFlow) => {
    const key = `${repo.slug}:${flow.path}:${flow.name}`;
    setOpening(key);
    try {
      await sync({ apiUrl, repo, workspaceSlug }, flow.path, flow.name);
    } finally {
      setOpening(undefined);
    }
  };

  return (
    <>
      {dialogs.render}

      <TextInputField
        aria-label='Filter repos and flows'
        className={tw`px-1.5`}
        inputClassName={tw`py-1 text-sm`}
        onChange={setFilter}
        placeholder='Filter'
        value={filter}
      />

      <Tree aria-label='Cloud repos' items={visible}>
        {({ repo }) => (
          <RepoItem
            id={repo.slug}
            isPinned={pinned.includes(repo.slug)}
            onOpen={(flow) => void open(repo, flow)}
            onTogglePin={() => void togglePin(repo.slug)}
            opening={opening}
            query={query}
            target={{ apiUrl, repo, workspaceSlug }}
          />
        )}
      </Tree>

      {visible.length === 0 && <StateLine>No matches</StateLine>}
    </>
  );
};

interface RepoItemProps {
  id: string;
  isPinned: boolean;
  onOpen: (flow: StresseurCloudFlow) => void;
  onTogglePin: () => void;
  opening: string | undefined;
  query: string;
  target: CloudTarget;
}

const RepoItem = ({ id, isPinned, onOpen, onTogglePin, opening, query, target }: RepoItemProps) => {
  const { repo } = target;
  const { transport } = routes.root.useRouteContext();
  const { workspaceId } = routes.dashboard.workspace.route.useLoaderData();
  const queryClient = useQueryClient();
  const toastQueue = useToastQueue();

  const [isExpanded, setIsExpanded] = useState(false);
  const flows = useCloudFlows(repo.slug, isExpanded || !!query);
  const clone = useClone(repo.slug);
  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  const setLocalDir = async (localDir: string) => {
    try {
      await request({
        input: {
          apiUrl: target.apiUrl,
          cloudWorkspaceSlug: target.workspaceSlug,
          localDir,
          repoName: repo.full_name,
          repoSlug: repo.slug,
          workspaceId,
        },
        method: CloudLinkService.method.cloudLinkSetLocalDir,
        transport,
      });
    } catch (error) {
      toastQueue.add({ title: connectErrorText(error) }, { timeout: 5000 });
    }
    await queryClient.invalidateQueries({ queryKey: cloudKeys.clone(repo.slug) });
    await queryClient.invalidateQueries({ queryKey: cloudKeys.links(Ulid.construct(workspaceId).toCanonical()) });
  };

  const linkClone = async () => {
    const api = localCloneApi();
    if (!api) return;
    const picked = await api.pick({ full_name: repo.full_name, slug: repo.slug });
    if (!picked) return;
    if (!picked.ok) {
      toastQueue.add({ title: picked.message }, { timeout: 5000 });
      return;
    }
    track('cloud_clone_linked');
    await setLocalDir(picked.data);
  };

  const unlinkClone = async () => {
    await localCloneApi()?.unlink(repo.slug);
    await setLocalDir('');
  };

  let childItems: ReactNode;
  if (!flows.data) {
    childItems = (
      <TreeItem id={`${id}:loading`} isDisabled textValue='Loading'>
        {flows.isFetching ? <Spinner /> : <span />}
      </TreeItem>
    );
  } else if (!flows.data.ok) {
    childItems = (
      <TreeItem id={`${id}:error`} isDisabled textValue='Error'>
        <span className={tw`truncate text-sm text-on-neutral-low`}>{cloudErrorText(flows.data.error)}</span>
      </TreeItem>
    );
  } else {
    const repoMatches = !query || repo.full_name.toLowerCase().includes(query);
    const items = flows.data.data.flows.filter(
      (_) => repoMatches || _.name.toLowerCase().includes(query) || _.path.toLowerCase().includes(query),
    );
    const files = Map.groupBy(items, (_) => _.path);

    childItems =
      files.size === 0 ? (
        <TreeItem id={`${id}:empty`} isDisabled textValue='No flows'>
          <span className={tw`text-sm text-on-neutral-low`}>No flows</span>
        </TreeItem>
      ) : (
        [...files].map(([path, fileFlows]) => (
          <TreeItem
            childItems={fileFlows.map((flow) => {
              const key = `${repo.slug}:${flow.path}:${flow.name}`;
              return (
                <TreeItem id={key} key={key} onAction={() => void onOpen(flow)} textValue={flow.name}>
                  <CiDot flow={flow} />
                  <Text className={tw`flex-1 truncate`}>{flow.name}</Text>
                  {opening === key && <Spinner />}
                </TreeItem>
              );
            })}
            id={`${id}:${path}`}
            key={path}
            textValue={path}
          >
            <FiFileText className={tw`size-4 shrink-0 text-on-neutral-low`} />
            <Text className={tw`flex-1 truncate`}>{path.replace(/^api-tests\//, '')}</Text>
          </TreeItem>
        ))
      );
  }

  return (
    <TreeItem
      childItems={childItems}
      id={id}
      onContextMenu={onContextMenu}
      setIsExpanded={setIsExpanded}
      textValue={repo.full_name}
    >
      <FlowsIcon className={tw`size-4 shrink-0 text-on-neutral-low`} />
      <Text className={twJoin(tw`flex-1 truncate`, !repo.records_ci && tw`text-on-neutral-low`)}>{repo.full_name}</Text>
      {clone && <FiHardDrive aria-label='Local clone linked' className={tw`size-3.5 shrink-0 text-on-neutral-low`} />}
      {isPinned && <TbPin aria-label='Pinned' className={tw`size-3.5 shrink-0 text-on-neutral-low`} />}

      <MenuTrigger {...menuTriggerProps}>
        <Button aria-label='Repo options' className={tw`p-0.5`} variant='ghost'>
          <FiMoreHorizontal className={tw`size-4 text-on-neutral-low`} />
        </Button>

        <Menu {...menuProps}>
          <MenuItem onAction={onTogglePin}>{isPinned ? 'Unpin' : 'Pin'}</MenuItem>
          {localCloneApi() &&
            (clone ? (
              <MenuItem onAction={() => void unlinkClone()}>Unlink clone</MenuItem>
            ) : (
              <MenuItem onAction={() => void linkClone()}>Link local clone…</MenuItem>
            ))}
          <MenuItem onAction={() => void cloudApi()?.open(repo.url)}>Open in Stresseur Cloud</MenuItem>
        </Menu>
      </MenuTrigger>
    </TreeItem>
  );
};

const CiDot = ({ flow }: { flow: StresseurCloudFlow }) => {
  const status = flow.last_ci?.status;
  const label = status === 'passed' ? 'Last CI run passed' : status === 'failed' ? 'Last CI run failed' : 'No CI runs';
  return (
    <span
      aria-label={label}
      className={twJoin(
        tw`size-2 shrink-0 rounded-full`,
        status === 'passed' && tw`bg-success`,
        status === 'failed' && tw`bg-danger`,
        !status && tw`bg-neutral-high`,
      )}
      role='img'
      title={label}
    />
  );
};
