import { createFileRoute, Outlet, useMatchRoute, useNavigate, useRouteContext } from '@tanstack/react-router';
import { pipe, Schema } from 'effect';
import { Ulid } from 'id128';
import { RefObject, useRef } from 'react';
import { ListBox, MenuTrigger, Text, Tooltip, TooltipTrigger } from 'react-aria-components';
import { FiMoreHorizontal, FiPlus } from 'react-icons/fi';
import { Panel, PanelGroup } from 'react-resizable-panels';
import { twJoin } from 'tailwind-merge';

import { export$ } from '@the-dev-tools/spec/export/v1/export-ExportService_connectquery';
import { FlowListItem } from '@the-dev-tools/spec/flow/v1/flow_pb';
import { CollectionCreateEndpoint } from '@the-dev-tools/spec/meta/collection/v1/collection.endpoints.ts';
import {
  FlowCreateEndpoint,
  FlowDeleteEndpoint,
  FlowListEndpoint,
  FlowUpdateEndpoint,
} from '@the-dev-tools/spec/meta/flow/v1/flow.endpoints.ts';
import { WorkspaceGetEndpoint } from '@the-dev-tools/spec/meta/workspace/v1/workspace.endpoints.ts';
import { Avatar } from '@the-dev-tools/ui/avatar';
import { Button, ButtonAsLink } from '@the-dev-tools/ui/button';
import { CollectionIcon, FlowsIcon, OverviewIcon } from '@the-dev-tools/ui/icons';
import { ListBoxItem } from '@the-dev-tools/ui/list-box';
import { Menu, MenuItem, useContextMenuState } from '@the-dev-tools/ui/menu';
import { PanelResizeHandle } from '@the-dev-tools/ui/resizable-panel';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextField, useEditableTextState } from '@the-dev-tools/ui/text-field';
import { saveFile, useEscapePortal } from '@the-dev-tools/ui/utils';
import { useConnectMutation } from '~/api/connect-query';
import { useMutate, useQuery } from '~data-client';

import { DashboardLayout } from '../authorized';
import { CollectionListTree } from '../collection';
import { EnvironmentsWidget } from '../environment';
import { StatusBar } from '../status-bar';

export class WorkspaceRouteSearch extends Schema.Class<WorkspaceRouteSearch>('WorkspaceRouteSearch')({
  showLogs: pipe(Schema.Boolean, Schema.optional),
}) {}

const makeRoute = createFileRoute('/_authorized/workspace/$workspaceIdCan');

export const Route = makeRoute({
  validateSearch: (_) => Schema.decodeSync(WorkspaceRouteSearch)(_),
  loader: ({ params: { workspaceIdCan } }) => {
    const workspaceId = Ulid.fromCanonical(workspaceIdCan).bytes;
    return { workspaceId };
  },
  component: Layout,
});

function Layout() {
  const { dataClient } = useRouteContext({ from: '__root__' });

  const { workspaceId } = Route.useLoaderData();
  const { workspaceIdCan } = Route.useParams();

  const workspace = useQuery(WorkspaceGetEndpoint, { workspaceId });

  return (
    <DashboardLayout
      navbar={
        <>
          <ButtonAsLink
            className={tw`-ml-3 gap-2 px-2 py-1`}
            href={{ params: { workspaceIdCan }, to: '/workspace/$workspaceIdCan' }}
            variant='ghost dark'
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
          className={tw`flex flex-col bg-slate-50`}
          defaultSize={20}
          maxSize={40}
          minSize={10}
          style={{ overflowY: 'auto' }}
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
              <h2 className={tw`text-md flex-1 font-semibold leading-5 tracking-tight text-slate-800`}>Collections</h2>

              <TooltipTrigger delay={750}>
                <Button
                  className={tw`bg-slate-200 p-0.5`}
                  onPress={() => dataClient.fetch(CollectionCreateEndpoint, { name: 'New collection', workspaceId })}
                  variant='ghost'
                >
                  <FiPlus className={tw`size-4 stroke-[1.2px] text-slate-500`} />
                </Button>
                <Tooltip className={tw`rounded-md bg-slate-800 px-2 py-1 text-xs text-white`}>
                  Add New Collection
                </Tooltip>
              </TooltipTrigger>
            </div>

            <CollectionListTree navigate showControls />

            <FlowList />
          </div>
        </Panel>

        <PanelResizeHandle direction='horizontal' />

        <Panel>
          <PanelGroup direction='vertical'>
            <Panel>
              <Outlet />
            </Panel>
            <StatusBar />
          </PanelGroup>
        </Panel>
      </PanelGroup>
    </DashboardLayout>
  );
}

const FlowList = () => {
  const { dataClient } = useRouteContext({ from: '__root__' });

  const navigate = useNavigate();

  const { workspaceId } = Route.useLoaderData();

  const { items: flows } = useQuery(FlowListEndpoint, { workspaceId });

  const listRef = useRef<HTMLDivElement>(null);

  return (
    <>
      <div className={tw`flex items-center gap-2 px-2.5 py-1.5`}>
        <FlowsIcon className={tw`size-5 text-slate-500`} />
        <h2 className={tw`text-md flex-1 font-semibold leading-5 tracking-tight text-slate-800`}>Flows</h2>

        <TooltipTrigger delay={750}>
          <Button
            className={tw`bg-slate-200 p-0.5`}
            onPress={async () => {
              const { flowId } = await dataClient.fetch(FlowCreateEndpoint, { name: 'New flow', workspaceId });

              const flowIdCan = Ulid.construct(flowId).toCanonical();

              await navigate({
                from: Route.fullPath,
                to: '/workspace/$workspaceIdCan/flow/$flowIdCan',

                params: { flowIdCan },
              });
            }}
            variant='ghost'
          >
            <FiPlus className={tw`size-4 stroke-[1.2px] text-slate-500`} />
          </Button>
          <Tooltip className={tw`rounded-md bg-slate-800 px-2 py-1 text-xs text-white`}>Add New Flow</Tooltip>
        </TooltipTrigger>
      </div>

      <div className={tw`relative`} ref={listRef}>
        <ListBox aria-label='Flow list' className={tw`w-full`} items={flows} selectionMode='single'>
          {(_) => {
            const id = Ulid.construct(_.flowId).toCanonical();
            return <FlowItem flow={_} id={id} listRef={listRef} />;
          }}
        </ListBox>
      </div>
    </>
  );
};

interface FlowItemProps {
  flow: FlowListItem;
  id: string;
  listRef: RefObject<HTMLDivElement | null>;
}

const FlowItem = ({ flow: { flowId, name }, id: flowIdCan, listRef }: FlowItemProps) => {
  const { dataClient } = useRouteContext({ from: '__root__' });

  const { workspaceIdCan } = Route.useParams();
  const { workspaceId } = Route.useLoaderData();

  const matchRoute = useMatchRoute();
  const navigate = useNavigate();

  const [flowUpdate, flowUpdateLoading] = useMutate(FlowUpdateEndpoint);

  // TODO: switch to Data Client Endpoint
  const exportMutation = useConnectMutation(export$);

  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  const escape = useEscapePortal(listRef);

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    onSuccess: (_) => flowUpdate({ flowId, name: _ }),
    value: name,
  });

  return (
    <ListBoxItem
      className={tw`text-md rounded-md pl-9 font-medium leading-5`}
      href={{ params: { flowIdCan, workspaceIdCan }, to: '/workspace/$workspaceIdCan/flow/$flowIdCan' }}
      id={flowIdCan}
      showSelectIndicator={false}
      textValue={name}
    >
      <div className={tw`contents`} onContextMenu={onContextMenu}>
        <Text className={twJoin(tw`flex-1 truncate`, isEditing && tw`opacity-0`)} ref={escape.ref} slot='label'>
          {name}
        </Text>

        {isEditing &&
          escape.render(
            <TextField
              aria-label='Flow name'
              className={tw`w-full`}
              inputClassName={tw`-my-1 py-1`}
              isDisabled={flowUpdateLoading}
              {...textFieldProps}
            />,
          )}

        <MenuTrigger {...menuTriggerProps}>
          <Button className={tw`p-0.5`} variant='ghost'>
            <FiMoreHorizontal className={tw`size-4 text-slate-500`} />
          </Button>

          <Menu {...menuProps}>
            <MenuItem onAction={() => void edit()}>Rename</MenuItem>

            <MenuItem
              onAction={async () => {
                const { data, name } = await exportMutation.mutateAsync({ flowIds: [flowId], workspaceId });
                saveFile({ blobParts: [data], name });
              }}
            >
              Export
            </MenuItem>

            <MenuItem
              onAction={async () => {
                await dataClient.fetch(FlowDeleteEndpoint, { flowId });
                if (!matchRoute({ params: { flowIdCan }, to: '/workspace/$workspaceIdCan/flow/$flowIdCan' })) return;
                await navigate({ from: Route.fullPath, to: '/workspace/$workspaceIdCan' });
              }}
              variant='danger'
            >
              Delete
            </MenuItem>
          </Menu>
        </MenuTrigger>
      </div>
    </ListBoxItem>
  );
};
