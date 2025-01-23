import { createFileRoute, Link } from '@tanstack/react-router';
import { DateTime, Duration, Effect, pipe, Runtime, Schema } from 'effect';
import { Ulid } from 'id128';
import { useState } from 'react';
import { Form, MenuTrigger } from 'react-aria-components';
import { FiMoreHorizontal } from 'react-icons/fi';

import { useConnectMutation, useConnectQuery } from '@the-dev-tools/api/connect-query';
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
import { TextField } from '@the-dev-tools/ui/text-field';

export const Route = createFileRoute('/_authorized/_dashboard/')({
  component: Page,
});

class RenameForm extends Schema.Class<RenameForm>('WorkspaceRenameForm')({
  name: Schema.String,
}) {}

function Page() {
  const workspaceListQuery = useConnectQuery(workspaceList);
  const workspaceCreateMutation = useConnectMutation(workspaceCreate);

  if (!workspaceListQuery.isSuccess) return null;
  const { items: workspaces } = workspaceListQuery.data;

  return (
    <div className={tw`container mx-auto my-12 grid gap-x-10 gap-y-6`}>
      <div className={tw`col-span-full`}>
        <span className={tw`mb-1 text-sm leading-5 tracking-tight text-slate-500`}>
          {pipe(DateTime.unsafeNow(), DateTime.formatLocal({ dateStyle: 'full' }))}
        </span>
        <h1 className={tw`text-2xl font-medium leading-8 tracking-tight text-slate-800`}>Welcome to Dev Tools 👋</h1>
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
  const { runtime } = Route.useRouteContext();

  const [renaming, setRenaming] = useState(false);

  const workspaceUpdateMutation = useConnectMutation(workspaceUpdate);
  const workspaceDeleteMutation = useConnectMutation(workspaceDelete);

  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  return (
    <div className={tw`flex items-center gap-3 px-5 py-4`} onContextMenu={onContextMenu}>
      <Avatar shape='square' size='md'>
        {workspace.name}
      </Avatar>

      <div
        className={tw`grid flex-1 grid-flow-col grid-cols-[1fr] grid-rows-2 gap-x-9 text-xs leading-5 tracking-tight text-slate-500`}
      >
        <div>
          {renaming ? (
            <Form
              className='flex flex-row gap-4'
              onSubmit={(event) =>
                Effect.gen(function* () {
                  event.preventDefault();

                  const { name } = yield* pipe(
                    new FormData(event.currentTarget),
                    Object.fromEntries,
                    Schema.decode(RenameForm),
                  );

                  workspaceUpdateMutation.mutate({ workspaceId, name });

                  setRenaming(false);
                }).pipe(Runtime.runPromise(runtime))
              }
            >
              <TextField
                aria-label='Workspace name'
                name='name'
                className={tw`text-md font-semibold tracking-tight text-slate-800`}
                defaultValue={workspace.name}
                // eslint-disable-next-line jsx-a11y/no-autofocus
                autoFocus
              />
              <Button type='submit'>Save</Button>
            </Form>
          ) : (
            <div className={tw`text-md font-semibold leading-5 tracking-tight text-slate-800`}>
              <Link to='/workspace/$workspaceIdCan' params={{ workspaceIdCan }}>
                {workspace.name}
              </Link>
            </div>
          )}
        </div>
        <div className={tw`flex items-center gap-2`}>
          {/* <span>
            by <strong className={tw`font-medium`}>N/A</strong>
          </span> */}
          {/* <div className={tw`size-0.5 rounded-full bg-slate-400`} /> */}
          <span>
            Created {pipe(Date.now() - workspaceUlid.time.getMilliseconds(), Duration.decode, Duration.format)} ago
          </span>
          <div className={tw`size-0.5 rounded-full bg-slate-400`} />
          {workspace.updated && (
            <span>Updated {pipe(Date.now() - workspace.updated.nanos, Duration.decode, Duration.format)} ago</span>
          )}
        </div>
        <span>Collection</span>
        <div className={tw`flex items-center gap-1`}>
          <CollectionIcon />
          <strong className={tw`font-semibold text-slate-800`}>N/A</strong> {/* TODO: implement */}
        </div>
        <span>Flows</span>
        <div className={tw`flex items-center gap-1`}>
          <FlowsIcon />
          <strong className={tw`font-semibold text-slate-800`}>N/A</strong> {/* TODO: implement */}
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
          <MenuItem onAction={() => void setRenaming(true)}>Rename</MenuItem>
          <MenuItem onAction={() => void workspaceDeleteMutation.mutate({ workspaceId })} variant='danger'>
            Delete
          </MenuItem>
        </Menu>
      </MenuTrigger>
    </div>
  );
};
