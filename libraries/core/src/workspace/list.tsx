import { timestampDate } from '@bufbuild/protobuf/wkt';
import { createFileRoute, Link } from '@tanstack/react-router';
import { DateTime, pipe } from 'effect';
import { Ulid } from 'id128';
import { MenuTrigger } from 'react-aria-components';
import { FiMoreHorizontal } from 'react-icons/fi';
import TimeAgo from 'react-timeago';

import { useConnectMutation, useConnectSuspenseQuery } from '@the-dev-tools/api/connect-query';
import { WorkspaceListItem } from '@the-dev-tools/spec/workspace/v1/workspace_pb';
import {
  workspaceCreate,
  workspaceDelete,
  workspaceList,
  workspaceUpdate,
} from '@the-dev-tools/spec/workspace/v1/workspace-WorkspaceService_connectquery';
import { Avatar } from '@the-dev-tools/ui/avatar';
import { Button } from '@the-dev-tools/ui/button';
import { CollectionIcon, FlowsIcon } from '@the-dev-tools/ui/icons';
import { Menu, MenuItem, useContextMenuState } from '@the-dev-tools/ui/menu';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextField, useEditableTextState } from '@the-dev-tools/ui/text-field';

export const Route = createFileRoute('/_authorized/_dashboard/')({
  component: Page,
});

function Page() {
  const {
    data: { items: workspaces },
  } = useConnectSuspenseQuery(workspaceList);
  const workspaceCreateMutation = useConnectMutation(workspaceCreate);

  return (
    <div className={tw`container mx-auto my-12 grid gap-x-10 gap-y-6`}>
      <div className={tw`col-span-full`}>
        <span className={tw`mb-1 text-sm leading-5 tracking-tight text-slate-500`}>
          {pipe(DateTime.unsafeNow(), DateTime.formatLocal({ dateStyle: 'full' }))}
        </span>
        <h1 className={tw`text-2xl font-medium leading-8 tracking-tight text-slate-800`}>Welcome to DevTools 👋</h1>
      </div>

      <div className={tw`divide-y divide-slate-200 rounded-lg border border-slate-200`}>
        <div className={tw`flex items-center gap-2 px-5 py-3`}>
          <span className={tw`flex-1 font-semibold tracking-tight text-slate-800`}>Your Workspaces</span>
          {/* <Button>View All Workspaces</Button> */}
          <Button variant='primary' onPress={() => void workspaceCreateMutation.mutate({ name: 'New Workspace' })}>
            Add Workspace
          </Button>
        </div>

        {workspaces.map((_) => {
          const workspaceUlid = Ulid.construct(_.workspaceId);
          const workspaceIdCan = workspaceUlid.toCanonical();
          return (
            <Row key={workspaceIdCan} workspace={_} workspaceIdCan={workspaceIdCan} workspaceUlid={workspaceUlid} />
          );
        })}
      </div>
    </div>
  );
}

interface RowProps {
  workspace: WorkspaceListItem;
  workspaceIdCan: string;
  workspaceUlid: Ulid;
}

const Row = ({ workspace: { workspaceId, ...workspace }, workspaceIdCan, workspaceUlid }: RowProps) => {
  const workspaceUpdateMutation = useConnectMutation(workspaceUpdate);
  const workspaceDeleteMutation = useConnectMutation(workspaceDelete);

  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    value: workspace.name,
    onSuccess: (_) => workspaceUpdateMutation.mutateAsync({ workspaceId, name: _ }),
  });

  return (
    <div className={tw`flex items-center gap-3 px-5 py-4`} onContextMenu={onContextMenu}>
      <Avatar shape='square' size='md'>
        {workspace.name}
      </Avatar>

      <div
        className={tw`grid flex-1 grid-flow-col grid-cols-[1fr] grid-rows-2 gap-x-9 text-xs leading-5 tracking-tight text-slate-500`}
      >
        {isEditing ? (
          <TextField
            className={tw`justify-self-start`}
            inputClassName={tw`text-md -my-1 py-1 font-semibold leading-none tracking-tight text-slate-800`}
            isDisabled={workspaceUpdateMutation.isPending}
            {...textFieldProps}
          />
        ) : (
          <div className={tw`text-md font-semibold leading-5 tracking-tight text-slate-800`}>
            <Link to='/workspace/$workspaceIdCan' params={{ workspaceIdCan }}>
              {workspace.name}
            </Link>
          </div>
        )}

        <div className={tw`flex items-center gap-2`}>
          {/* <span>
            by <strong className={tw`font-medium`}>N/A</strong>
          </span> */}
          {/* <div className={tw`size-0.5 rounded-full bg-slate-400`} /> */}
          <span>
            Created <TimeAgo date={workspaceUlid.time} minPeriod={60} />
          </span>
          {workspace.updated && (
            <>
              <div className={tw`size-0.5 rounded-full bg-slate-400`} />
              <span>
                Updated <TimeAgo date={timestampDate(workspace.updated)} minPeriod={60} />
              </span>
            </>
          )}
        </div>
        <span>Collection</span>
        <div className={tw`flex items-center gap-1`}>
          <CollectionIcon />
          <strong className={tw`font-semibold text-slate-800`}>{workspace.collectionCount}</strong>
        </div>
        <span>Flows</span>
        <div className={tw`flex items-center gap-1`}>
          <FlowsIcon />
          <strong className={tw`font-semibold text-slate-800`}>{workspace.flowCount}</strong>
        </div>
        {/* <span>N/A Members</span> */}
        {/* <div className={tw`flex gap-2`}>
          <div className={tw`flex`}>
            {['A', 'B', 'C', 'D'].map((_) => (
              <Avatar key={_} className={tw`-ml-1.5 first:ml-0`}>
                {_}
              </Avatar>
            ))}
          </div>
          <AddButton />
        </div> */}
      </div>

      <MenuTrigger {...menuTriggerProps}>
        <Button className={tw`ml-6 p-1`} variant='ghost'>
          <FiMoreHorizontal className={tw`size-4 stroke-[1.2px] text-slate-500`} />
        </Button>

        <Menu {...menuProps}>
          <MenuItem onAction={() => void edit()}>Rename</MenuItem>
          <MenuItem onAction={() => void workspaceDeleteMutation.mutate({ workspaceId })} variant='danger'>
            Delete
          </MenuItem>
        </Menu>
      </MenuTrigger>
    </div>
  );
};
