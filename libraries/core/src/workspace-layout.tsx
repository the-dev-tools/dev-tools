import { createClient } from '@connectrpc/connect';
import { createConnectQueryKey, createQueryOptions, useQuery as useConnectQuery } from '@connectrpc/connect-query';
import { useQuery } from '@tanstack/react-query';
import { createFileRoute, Outlet, redirect } from '@tanstack/react-router';
import { Array, Effect, pipe, Runtime, Schema } from 'effect';
import { Ulid } from 'id128';
import { useMemo, useRef, useState } from 'react';
import { Button as AriaButton, FileTrigger, Form, ListBox, MenuTrigger, Text } from 'react-aria-components';
import { FiChevronDown, FiMoreHorizontal, FiPlus, FiTerminal, FiTrash2, FiX } from 'react-icons/fi';
import { Panel, PanelGroup } from 'react-resizable-panels';
import { twMerge } from 'tailwind-merge';

import { useSpecMutation } from '@the-dev-tools/api/query';
import {
  collectionCreateSpec,
  collectionImportHarSpec,
  collectionImportPostmanSpec,
} from '@the-dev-tools/api/spec/collection';
import { flowCreateSpec, flowDeleteSpec, flowUpdateSpec } from '@the-dev-tools/api/spec/flow';
import { FlowListItem } from '@the-dev-tools/spec/flow/v1/flow_pb';
import { flowList } from '@the-dev-tools/spec/flow/v1/flow-FlowService_connectquery';
import { LogService, LogStreamResponse } from '@the-dev-tools/spec/log/v1/log_pb';
import { workspaceGet } from '@the-dev-tools/spec/workspace/v1/workspace-WorkspaceService_connectquery';
import { Avatar } from '@the-dev-tools/ui/avatar';
import { Button, ButtonAsLink } from '@the-dev-tools/ui/button';
import { ArrowToLeftIcon, CollectionIcon, FileImportIcon, FlowsIcon, OverviewIcon } from '@the-dev-tools/ui/icons';
import { ListBoxItem } from '@the-dev-tools/ui/list-box';
import { Menu, MenuItem } from '@the-dev-tools/ui/menu';
import { Popover } from '@the-dev-tools/ui/popover';
import { PanelResizeHandle, panelResizeHandleStyles } from '@the-dev-tools/ui/resizable-panel';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextField } from '@the-dev-tools/ui/text-field';

import { DashboardLayout } from './authorized';
import { CollectionListTree } from './collection';
import { EnvironmentsWidget } from './environment';

export class WorkspaceRouteSearch extends Schema.Class<WorkspaceRouteSearch>('WorkspaceRouteSearch')({
  showLogs: pipe(Schema.Boolean, Schema.optional),
}) {}

export const Route = createFileRoute('/_authorized/workspace/$workspaceIdCan')({
  component: Layout,
  pendingComponent: () => 'Loading workspace...',
  validateSearch: (_) => Schema.decodeSync(WorkspaceRouteSearch)(_),
  loader: async ({ params: { workspaceIdCan }, context: { transport, queryClient } }) => {
    const workspaceId = Ulid.fromCanonical(workspaceIdCan).bytes;
    const options = createQueryOptions(workspaceGet, { workspaceId }, { transport });
    await queryClient.ensureQueryData(options).catch(() => redirect({ to: '/', throw: true }));
    return { workspaceId };
  },
});

function Layout() {
  const { workspaceId } = Route.useLoaderData();
  const { workspaceIdCan } = Route.useParams();

  const collectionCreateMutation = useSpecMutation(collectionCreateSpec);
  const collectionImportPostmanMutation = useSpecMutation(collectionImportPostmanSpec);
  const collectionImportHarMutation = useSpecMutation(collectionImportHarSpec);

  const postmanFileTriggerRef = useRef<HTMLInputElement>(null);
  const harFileTriggerRef = useRef<HTMLInputElement>(null);

  const workspaceGetQuery = useConnectQuery(workspaceGet, { workspaceId });
  if (!workspaceGetQuery.isSuccess) return;
  const workspace = workspaceGetQuery.data;

  return (
    <DashboardLayout
      navbar={
        <>
          <MenuTrigger>
            <Button variant='ghost dark' className={tw`-ml-3 gap-2 px-2 py-1`}>
              <Avatar shape='square' size='base'>
                {workspace.name}
              </Avatar>
              <span className={tw`text-xs font-semibold leading-5 tracking-tight`}>{workspace.name}</span>
              <FiChevronDown className={tw`size-4`} />
            </Button>

            <Menu>
              <MenuItem
                href={{
                  to: '/workspace/$workspaceIdCan',
                  params: { workspaceIdCan },
                }}
              >
                Home
              </MenuItem>
              <MenuItem
                href={{
                  to: '/workspace/$workspaceIdCan/members',
                  params: { workspaceIdCan },
                }}
              >
                Members
              </MenuItem>
            </Menu>
          </MenuTrigger>
          <div className='flex-1' />
        </>
      }
    >
      <PanelGroup direction='horizontal'>
        <Panel
          className={tw`flex flex-col bg-slate-50`}
          style={{ overflowY: 'auto' }}
          defaultSize={20}
          minSize={10}
          maxSize={40}
        >
          <EnvironmentsWidget />

          <div className={tw`flex flex-col gap-2 p-1.5`}>
            <div className={tw`flex items-center gap-2 px-2.5 py-1.5`}>
              <OverviewIcon className={tw`size-5 text-slate-500`} />
              <h2 className={tw`text-md font-semibold leading-5 tracking-tight text-slate-800`}>Overview</h2>
            </div>

            <div className={tw`flex items-center gap-2 px-2.5 py-1.5`}>
              <CollectionIcon className={tw`size-5 text-slate-500`} />
              <h2 className={tw`flex-1 text-md font-semibold leading-5 tracking-tight text-slate-800`}>Collections</h2>

              <MenuTrigger>
                <Button className={tw`p-0.5`} variant='ghost'>
                  <FileImportIcon className={tw`size-4 text-slate-500`} />
                </Button>

                <Menu popoverPlacement='bottom'>
                  <MenuItem onAction={() => void postmanFileTriggerRef.current?.click()}>Postman</MenuItem>

                  <MenuItem onAction={() => void harFileTriggerRef.current?.click()}>HAR</MenuItem>
                </Menu>
              </MenuTrigger>

              <FileTrigger
                ref={postmanFileTriggerRef}
                onSelect={async (_) => {
                  const file = _?.item(0);
                  if (!file) return;
                  const data = new Uint8Array(await file.arrayBuffer());
                  collectionImportPostmanMutation.mutate({ workspaceId, name: file.name, data });
                }}
              >
                <AriaButton className={tw`hidden`} />
              </FileTrigger>

              <FileTrigger
                ref={harFileTriggerRef}
                onSelect={async (_) => {
                  const file = _?.item(0);
                  if (!file) return;
                  const data = new Uint8Array(await file.arrayBuffer());
                  collectionImportHarMutation.mutate({ workspaceId, name: file.name, data });
                }}
              >
                <AriaButton className={tw`hidden`} />
              </FileTrigger>

              <Button
                className={tw`bg-slate-200 p-0.5`}
                variant='ghost'
                onPress={() => void collectionCreateMutation.mutate({ workspaceId, name: 'New collection' })}
              >
                <FiPlus className={tw`size-4 stroke-[1.2px] text-slate-500`} />
              </Button>
            </div>

            <CollectionListTree navigate showControls />

            <FlowList />
          </div>
        </Panel>
        <PanelResizeHandle direction='horizontal' />
        <Panel className='flex h-full flex-col !overflow-auto'>
          <PanelGroup direction='vertical'>
            <Outlet />
            <StatusBar />
          </PanelGroup>
        </Panel>
      </PanelGroup>
    </DashboardLayout>
  );
}

const FlowList = () => {
  const { workspaceId } = Route.useLoaderData();

  const flowListQuery = useConnectQuery(flowList, { workspaceId });
  const flowCreateMutation = useSpecMutation(flowCreateSpec);

  if (!flowListQuery.isSuccess) return null;
  const flows = flowListQuery.data.items;

  return (
    <>
      <div className={tw`flex items-center gap-2 px-2.5 py-1.5`}>
        <FlowsIcon className={tw`size-5 text-slate-500`} />
        <h2 className={tw`flex-1 text-md font-semibold leading-5 tracking-tight text-slate-800`}>Flows</h2>

        <Button
          className={tw`bg-slate-200 p-0.5`}
          variant='ghost'
          onPress={() => void flowCreateMutation.mutate({ workspaceId, name: 'New flow' })}
        >
          <FiPlus className={tw`size-4 stroke-[1.2px] text-slate-500`} />
        </Button>
      </div>

      <ListBox aria-label='Flow list' selectionMode='single' items={flows} className={tw`w-full`}>
        {(_) => {
          const id = Ulid.construct(_.flowId).toCanonical();
          return <FlowItem id={id} flow={_} />;
        }}
      </ListBox>
    </>
  );
};

interface FlowItemProps {
  id: string;
  flow: FlowListItem;
}

const FlowItem = ({ id: flowIdCan, flow: { flowId, name } }: FlowItemProps) => {
  const { runtime } = Route.useRouteContext();
  const { workspaceId } = Route.useLoaderData();
  const { workspaceIdCan } = Route.useParams();

  const flowDeleteMutation = useSpecMutation(flowDeleteSpec);
  const flowUpdateMutation = useSpecMutation(flowUpdateSpec);

  const triggerRef = useRef(null);

  const [isRenaming, setIsRenaming] = useState(false);

  return (
    <ListBoxItem
      id={flowIdCan}
      textValue={name}
      href={{ to: '/workspace/$workspaceIdCan/flow/$flowIdCan', params: { workspaceIdCan, flowIdCan } }}
      className={tw`rounded-md pl-9 text-md font-medium leading-5`}
      showSelectIndicator={false}
    >
      <Text ref={triggerRef} className='flex-1 truncate' slot='label'>
        {name}
      </Text>

      <MenuTrigger>
        <Button variant='ghost' className={tw`p-0.5`}>
          <FiMoreHorizontal className={tw`size-4 text-slate-500`} />
        </Button>

        <Menu>
          <MenuItem onAction={() => void setIsRenaming(true)}>Rename</MenuItem>

          <MenuItem variant='danger' onAction={() => void flowDeleteMutation.mutate({ workspaceId, flowId })}>
            Delete
          </MenuItem>
        </Menu>
      </MenuTrigger>

      <Popover
        triggerRef={triggerRef}
        isOpen={isRenaming}
        onOpenChange={setIsRenaming}
        dialogAria-label='Rename collection'
      >
        <Form
          className='flex flex-1 items-center gap-2'
          onSubmit={(event) =>
            Effect.gen(function* () {
              event.preventDefault();

              const { name } = yield* pipe(
                new FormData(event.currentTarget),
                Object.fromEntries,
                Schema.decode(Schema.Struct({ name: Schema.String })),
              );

              flowUpdateMutation.mutate({ workspaceId, flowId, name });

              setIsRenaming(false);
            }).pipe(Runtime.runPromise(runtime))
          }
        >
          <TextField
            name='name'
            defaultValue={name}
            // eslint-disable-next-line jsx-a11y/no-autofocus
            autoFocus
            label='New name:'
            className={tw`contents`}
            labelClassName={tw`text-nowrap`}
            inputClassName={tw`w-full bg-transparent`}
          />

          <Button type='submit'>Save</Button>
        </Form>
      </Popover>
    </ListBoxItem>
  );
};

const StatusBar = () => {
  const { showLogs } = Route.useSearch();
  const { transport, queryClient } = Route.useRouteContext();

  const { logStream } = useMemo(() => createClient(LogService, transport), [transport]);

  const queryKey = useMemo(
    () =>
      createConnectQueryKey({
        schema: { ...LogService.method.logStream, methodKind: 'unary' },
        cardinality: 'infinite',
        transport,
      }),
    [transport],
  );

  const { data: logs } = useQuery({
    queryKey,
    initialData: [],
    queryFn: async ({ queryKey, signal }) => {
      for await (const log of logStream({})) {
        queryClient.setQueryData(queryKey, Array.append(log));
        if (signal.aborted) break;
      }
      return queryClient.getQueryData<LogStreamResponse[]>(queryKey)!;
    },
  });

  const separator = <div className={tw`h-3.5 w-px bg-slate-200`} />;

  const bar = (
    <div className={twMerge(tw`flex items-center gap-2 bg-slate-50 px-2 py-1`, showLogs && tw`bg-white`)}>
      {/* TODO: implement sidebar collapse */}
      <Button variant='ghost' className={tw`p-0.5`}>
        <ArrowToLeftIcon className={tw`size-4 text-slate-500`} />
      </Button>

      {separator}

      <ButtonAsLink
        variant='ghost'
        className={tw`px-2 py-1 text-xs leading-4 tracking-tight text-slate-800`}
        href={{
          to: '.',
          search: (_: Partial<WorkspaceRouteSearch>) =>
            ({ ..._, showLogs: true }) satisfies Partial<WorkspaceRouteSearch>,
        }}
      >
        <FiTerminal className={tw`size-3`} />
        <span>Logs</span>
      </ButtonAsLink>

      <div className={tw`flex-1`} />

      {showLogs && (
        <>
          <Button
            variant='ghost'
            className={tw`px-2 py-1 text-xs leading-4 tracking-tight text-slate-800`}
            onPress={() => void queryClient.setQueryData(queryKey, [])}
          >
            <FiTrash2 className={tw`size-3 text-slate-500`} />
            <span>Clear Logs</span>
          </Button>

          {separator}

          <ButtonAsLink
            variant='ghost'
            className={tw`p-0.5`}
            href={{
              to: '.',
              search: (_: Partial<WorkspaceRouteSearch>) =>
                ({ ..._, showLogs: undefined }) satisfies Partial<WorkspaceRouteSearch>,
            }}
          >
            <FiX className={tw`size-4 text-slate-500`} />
          </ButtonAsLink>
        </>
      )}
    </div>
  );

  return (
    <>
      {showLogs ? (
        <PanelResizeHandle direction='vertical' />
      ) : (
        <div className={panelResizeHandleStyles({ direction: 'vertical' })} />
      )}

      {bar}

      {showLogs && (
        <Panel order={100} className={tw`p-4 pt-0`}>
          <div
            className={tw`flex size-full flex-col-reverse overflow-auto rounded-md border border-slate-200 bg-slate-800 p-3 font-mono text-sm leading-5 text-slate-200 shadow-sm`}
          >
            <div>
              {logs.map((_) => {
                const ulid = Ulid.construct(_.logId);
                return (
                  <div key={ulid.toCanonical()}>
                    {ulid.time.toLocaleTimeString()}: {_.value}
                  </div>
                );
              })}
            </div>
          </div>
        </Panel>
      )}
    </>
  );
};
