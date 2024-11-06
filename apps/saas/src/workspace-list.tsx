import { useQuery as useConnectQuery } from '@connectrpc/connect-query';
import { createFileRoute, Link } from '@tanstack/react-router';
import { DateTime, Effect, pipe, Schema } from 'effect';
import { Ulid } from 'id128';
import { useState } from 'react';
import { Form, MenuTrigger } from 'react-aria-components';
import { FiMoreHorizontal } from 'react-icons/fi';

import { useSpecMutation } from '@the-dev-tools/api/query';
import { workspaceCreateSpec, workspaceDeleteSpec, workspaceUpdateSpec } from '@the-dev-tools/api/spec/workspace';
import { WorkspaceListItem } from '@the-dev-tools/spec/workspace/v1/workspace_pb';
import { workspaceList } from '@the-dev-tools/spec/workspace/v1/workspace-WorkspaceService_connectquery';
import { AddButton } from '@the-dev-tools/ui/add-button';
import { Avatar } from '@the-dev-tools/ui/avatar';
import { Button } from '@the-dev-tools/ui/button';
import { CollectionIcon, FlowsIcon } from '@the-dev-tools/ui/icons';
import { Menu, MenuItem } from '@the-dev-tools/ui/menu';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextField } from '@the-dev-tools/ui/text-field';

import { Runtime } from './runtime';

export const Route = createFileRoute('/_authorized/_dashboard/')({
  component: Page,
});

class RenameForm extends Schema.Class<RenameForm>('WorkspaceRenameForm')({
  name: Schema.String,
}) {}

function Page() {
  const { email } = Route.useRouteContext();

  const workspaceListQuery = useConnectQuery(workspaceList);
  const workspaceCreateMutation = useSpecMutation(workspaceCreateSpec);

  if (!workspaceListQuery.isSuccess) return null;
  const { items: workspaces } = workspaceListQuery.data;

  return (
    <div className={tw`container mx-auto my-12 grid gap-x-10 gap-y-6`}>
      <div className={tw`col-span-full`}>
        <span className={tw`mb-1 text-sm leading-5 tracking-tight text-slate-500`}>
          {pipe(DateTime.unsafeNow(), DateTime.formatLocal({ dateStyle: 'full' }))}
        </span>
        <h1 className={tw`text-2xl font-medium leading-8 tracking-tight text-slate-800`}>Good morning, {email} 👋</h1>
      </div>

      <div className={tw`divide-y divide-slate-200 rounded-lg border border-slate-200`}>
        <div className={tw`flex items-center gap-2 px-5 py-3`}>
          <span className={tw`flex-1 font-semibold tracking-tight text-slate-800`}>Your Recently Edited</span>
          <Button>View All Workspaces</Button> {/* TODO: implement */}
          <Button variant='primary' onPress={() => void workspaceCreateMutation.mutate({ name: 'New Workspace' })}>
            Add Workspace
          </Button>
        </div>

        {workspaces.map((_) => {
          const workspaceIdCan = Ulid.construct(_.workspaceId).toCanonical();
          return <Row key={workspaceIdCan} workspaceIdCan={workspaceIdCan} workspace={_} />;
        })}
      </div>
    </div>
  );
}

interface RowProps {
  workspaceIdCan: string;
  workspace: WorkspaceListItem;
}

const Row = ({ workspaceIdCan, workspace }: RowProps) => {
  const { workspaceId } = workspace;

  const [renaming, setRenaming] = useState(false);

  const workspaceUpdateMutation = useSpecMutation(workspaceUpdateSpec);
  const workspaceDeleteMutation = useSpecMutation(workspaceDeleteSpec);

  return (
    <div className={tw`flex items-center gap-3 px-5 py-4`}>
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
                }).pipe(Runtime.runPromise)
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
          <span>
            by <strong className={tw`font-medium`}>N/A</strong> {/* TODO: implement */}
          </span>
          <div className={tw`size-0.5 rounded-full bg-slate-400`} />
          <span>Created N/A ago</span> {/* TODO: implement */}
          <div className={tw`size-0.5 rounded-full bg-slate-400`} />
          <span>Updated N/A ago</span> {/* TODO: implement */}
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
        <span>N/A Members</span> {/* TODO: implement */}
        <div className={tw`flex gap-2`}>
          <div className={tw`flex`}>
            {/* TODO: implement */}
            {['A', 'B', 'C', 'D'].map((_) => (
              <Avatar key={_} className={tw`-ml-1.5 first:ml-0`}>
                {_}
              </Avatar>
            ))}
          </div>
          <AddButton />
        </div>
      </div>

      <MenuTrigger>
        <Button className={tw`ml-6 p-1`} variant='ghost'>
          <FiMoreHorizontal className={tw`size-4 stroke-[1.2px] text-slate-500`} />
        </Button>

        {/* TODO: new menu */}
        <Menu>
          <MenuItem onAction={() => void setRenaming(true)}>Rename</MenuItem>
          <MenuItem onAction={() => void workspaceDeleteMutation.mutate({ workspaceId })} variant='danger'>
            Delete
          </MenuItem>
        </Menu>
      </MenuTrigger>
    </div>
  );
};
