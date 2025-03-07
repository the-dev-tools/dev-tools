import { createQueryOptions } from '@connectrpc/connect-query';
import { createFileRoute, Outlet, redirect } from '@tanstack/react-router';
import { pipe, Schema } from 'effect';
import { Ulid } from 'id128';
import { RefObject, useRef } from 'react';
import { ListBox, MenuTrigger, Text } from 'react-aria-components';
import { FiMoreHorizontal, FiPlus } from 'react-icons/fi';
import { Panel, PanelGroup } from 'react-resizable-panels';
import { twJoin } from 'tailwind-merge';

import { useConnectMutation, useConnectSuspenseQuery } from '@the-dev-tools/api/connect-query';
import { collectionCreate } from '@the-dev-tools/spec/collection/v1/collection-CollectionService_connectquery';
import { FlowListItem } from '@the-dev-tools/spec/flow/v1/flow_pb';
import {
  flowCreate,
  flowDelete,
  flowList,
  flowUpdate,
} from '@the-dev-tools/spec/flow/v1/flow-FlowService_connectquery';
import { workspaceGet } from '@the-dev-tools/spec/workspace/v1/workspace-WorkspaceService_connectquery';
import { Avatar } from '@the-dev-tools/ui/avatar';
import { Button, ButtonAsLink } from '@the-dev-tools/ui/button';
import { CollectionIcon, FlowsIcon, OverviewIcon } from '@the-dev-tools/ui/icons';
import { ListBoxItem } from '@the-dev-tools/ui/list-box';
import { Menu, MenuItem, useContextMenuState } from '@the-dev-tools/ui/menu';
import { PanelResizeHandle } from '@the-dev-tools/ui/resizable-panel';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextField, useEditableTextState } from '@the-dev-tools/ui/text-field';
import { useEscapePortal } from '@the-dev-tools/ui/utils';

import { DashboardLayout } from '../authorized';
import { CollectionListTree } from '../collection';
import { EnvironmentsWidget } from '../environment';
import { useLogsQuery } from '../status-bar';

export class WorkspaceRouteSearch extends Schema.Class<WorkspaceRouteSearch>('WorkspaceRouteSearch')({
  showLogs: pipe(Schema.Boolean, Schema.optional),
}) {}

export const Route = createFileRoute('/_authorized/workspace/$workspaceIdCan')({
  component: Layout,
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

  const collectionCreateMutation = useConnectMutation(collectionCreate);

  const { data: workspace } = useConnectSuspenseQuery(workspaceGet, { workspaceId });

  // Keep the query alive while in workspace
  useLogsQuery();

  return (
    <DashboardLayout
      navbar={
        <>
          <ButtonAsLink
            variant='ghost dark'
            className={tw`-ml-3 gap-2 px-2 py-1`}
            href={{ to: '/workspace/$workspaceIdCan', params: { workspaceIdCan } }}
          >
            <Avatar shape='square' size='base'>
              {workspace.name}
            </Avatar>
            <span className={tw`text-xs font-semibold leading-5 tracking-tight`}>{workspace.name}</span>
            {/* <FiChevronDown className={tw`size-4`} /> */}
          </ButtonAsLink>

          {/* <MenuTrigger>
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
          </MenuTrigger> */}
          <div className='flex-1' />
        </>
      }
    >
      <PanelGroup direction='horizontal'>
        <Panel
          id='sidebar'
          order={1}
          className={tw`flex flex-col bg-slate-50`}
          style={{ overflowY: 'auto' }}
          defaultSize={20}
          minSize={10}
          maxSize={40}
        >
          <EnvironmentsWidget />

          <div className={tw`flex flex-col gap-2 p-1.5`}>
            <ButtonAsLink
              className={tw`flex items-center justify-start gap-2 px-2.5 py-1.5`}
              href={{ from: Route.fullPath, to: '/workspace/$workspaceIdCan' }}
              variant='ghost'
            >
              <OverviewIcon className={tw`size-5 text-slate-500`} />
              <h2 className={tw`text-md font-semibold leading-5 tracking-tight text-slate-800`}>Overview</h2>
            </ButtonAsLink>

            <div className={tw`flex items-center gap-2 px-2.5 py-1.5`}>
              <CollectionIcon className={tw`size-5 text-slate-500`} />
              <h2 className={tw`flex-1 text-md font-semibold leading-5 tracking-tight text-slate-800`}>Collections</h2>

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
        <Outlet />
      </PanelGroup>
    </DashboardLayout>
  );
}

const FlowList = () => {
  const { workspaceId } = Route.useLoaderData();

  const {
    data: { items: flows },
  } = useConnectSuspenseQuery(flowList, { workspaceId });

  const flowCreateMutation = useConnectMutation(flowCreate);

  const listRef = useRef<HTMLDivElement>(null);

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

      <div ref={listRef} className={tw`relative`}>
        <ListBox aria-label='Flow list' selectionMode='single' items={flows} className={tw`w-full`}>
          {(_) => {
            const id = Ulid.construct(_.flowId).toCanonical();
            return <FlowItem id={id} flow={_} listRef={listRef} />;
          }}
        </ListBox>
      </div>
    </>
  );
};

interface FlowItemProps {
  id: string;
  flow: FlowListItem;
  listRef: RefObject<HTMLDivElement | null>;
}

const FlowItem = ({ id: flowIdCan, flow: { flowId, name }, listRef }: FlowItemProps) => {
  const { workspaceIdCan } = Route.useParams();

  const flowDeleteMutation = useConnectMutation(flowDelete);
  const flowUpdateMutation = useConnectMutation(flowUpdate);

  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  const escape = useEscapePortal(listRef);

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    value: name,
    onSuccess: (_) => flowUpdateMutation.mutateAsync({ flowId, name: _ }),
  });

  return (
    <ListBoxItem
      id={flowIdCan}
      textValue={name}
      href={{ to: '/workspace/$workspaceIdCan/flow/$flowIdCan', params: { workspaceIdCan, flowIdCan } }}
      className={tw`rounded-md pl-9 text-md font-medium leading-5`}
      showSelectIndicator={false}
    >
      <div className={tw`contents`} onContextMenu={onContextMenu}>
        <Text ref={escape.ref} className={twJoin(tw`flex-1 truncate`, isEditing && tw`opacity-0`)} slot='label'>
          {name}
        </Text>

        {isEditing &&
          escape.render(
            <TextField
              className={tw`w-full`}
              inputClassName={tw`-my-1 py-1`}
              isDisabled={flowUpdateMutation.isPending}
              {...textFieldProps}
            />,
          )}

        <MenuTrigger {...menuTriggerProps}>
          <Button variant='ghost' className={tw`p-0.5`}>
            <FiMoreHorizontal className={tw`size-4 text-slate-500`} />
          </Button>

          <Menu {...menuProps}>
            <MenuItem onAction={() => void edit()}>Rename</MenuItem>

            <MenuItem variant='danger' onAction={() => void flowDeleteMutation.mutate({ flowId })}>
              Delete
            </MenuItem>
          </Menu>
        </MenuTrigger>
      </div>
    </ListBoxItem>
  );
};
