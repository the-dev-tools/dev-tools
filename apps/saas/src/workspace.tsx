import {
  createQueryOptions,
  useMutation as useConnectMutation,
  useQuery as useConnectQuery,
  useTransport,
} from '@connectrpc/connect-query';
import { Schema } from '@effect/schema';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { getRouteApi, Outlet } from '@tanstack/react-router';
import { Effect, pipe, Struct } from 'effect';
import { useState } from 'react';
import { Form, MenuTrigger } from 'react-aria-components';
import { Panel, PanelGroup } from 'react-resizable-panels';

import { Workspace } from '@the-dev-tools/protobuf/workspace/v1/workspace_pb';
import {
  createWorkspace,
  deleteWorkspace,
  getWorkspace,
  getWorkspaces,
  updateWorkspace,
} from '@the-dev-tools/protobuf/workspace/v1/workspace-WorkspaceService_connectquery';
import { Button, ButtonAsLink } from '@the-dev-tools/ui/button';
import { Menu, MenuItem } from '@the-dev-tools/ui/menu';
import { PanelResizeHandle } from '@the-dev-tools/ui/resizable-panel';
import { TextField } from '@the-dev-tools/ui/text-field';

import { CollectionsTree } from './collection';
import { DashboardLayout } from './dashboard';
import { Runtime } from './runtime';

class WorkspaceRenameForm extends Schema.Class<WorkspaceRenameForm>('WorkspaceRenameForm')({
  name: Schema.String,
}) {}

export const WorkspacesPage = () => {
  const queryClient = useQueryClient();
  const transport = useTransport();

  const queryOptions = createQueryOptions(getWorkspaces, undefined, { transport });
  const query = useQuery({ ...queryOptions, enabled: true });
  const createMutation = useConnectMutation(createWorkspace);

  if (!query.isSuccess) return null;
  const { workspaces } = query.data;

  return (
    <div className='flex size-full flex-col items-center justify-center gap-4'>
      <div>
        <Button
          kind='placeholder'
          variant='placeholder'
          onPress={async () => {
            await createMutation.mutateAsync({ name: 'New workspace' });
            await queryClient.invalidateQueries(queryOptions);
          }}
        >
          Create workspace
        </Button>
      </div>

      {workspaces.map((_) => (
        <WorkspaceRow key={_.id} workspace={_} />
      ))}
    </div>
  );
};

interface WorkspaceRowProps {
  workspace: Workspace;
}

const WorkspaceRow = ({ workspace }: WorkspaceRowProps) => {
  const queryClient = useQueryClient();
  const transport = useTransport();

  const [renaming, setRenaming] = useState(false);

  const queryOptions = createQueryOptions(getWorkspaces, undefined, { transport });

  const updateMutation = useConnectMutation(updateWorkspace);
  const deleteMutation = useConnectMutation(deleteWorkspace);

  return (
    <div className='flex gap-4'>
      {renaming ? (
        <Form
          className='contents'
          onSubmit={(event) =>
            Effect.gen(function* () {
              event.preventDefault();

              const { name } = yield* pipe(
                new FormData(event.currentTarget),
                Object.fromEntries,
                Schema.decode(WorkspaceRenameForm),
              );

              const data = Struct.evolve(workspace, { name: () => name });
              yield* Effect.tryPromise(() => updateMutation.mutateAsync(data));
              yield* Effect.tryPromise(() => queryClient.invalidateQueries(queryOptions));

              setRenaming(false);
            }).pipe(Runtime.runPromise)
          }
        >
          {/* eslint-disable-next-line jsx-a11y/no-autofocus */}
          <TextField aria-label='Workspace name' name='name' defaultValue={workspace.name} autoFocus />
          <Button kind='placeholder' variant='placeholder' type='submit'>
            Save
          </Button>
        </Form>
      ) : (
        <>
          <ButtonAsLink
            kind='placeholder'
            variant='placeholder'
            href={{ to: '/workspace/$workspaceId', params: { workspaceId: workspace.id } }}
          >
            {workspace.name}
          </ButtonAsLink>
          <Button kind='placeholder' variant='placeholder' onPress={() => void setRenaming(true)}>
            Rename
          </Button>
        </>
      )}
      <Button
        kind='placeholder'
        variant='placeholder'
        onPress={async () => {
          await deleteMutation.mutateAsync({ id: workspace.id });
          await queryClient.invalidateQueries(queryOptions);
        }}
      >
        Delete
      </Button>
    </div>
  );
};

const workspaceRoute = getRouteApi('/_authorized/workspace/$workspaceId');

export const WorkspaceLayout = () => {
  const { workspaceId } = workspaceRoute.useParams();

  const query = useConnectQuery(getWorkspace, { id: workspaceId });
  if (!query.isSuccess) return;
  const { workspace } = query.data;

  return (
    <DashboardLayout
      leftChildren={
        <MenuTrigger>
          <Button kind='placeholder' className='bg-transparent text-white' variant='placeholder'>
            {workspace!.name}
          </Button>
          <Menu>
            <MenuItem href={{ to: '/workspace/$workspaceId', params: { workspaceId } }}>Home</MenuItem>
            <MenuItem href={{ to: '/workspace/$workspaceId/members', params: { workspaceId } }}>Members</MenuItem>
          </Menu>
        </MenuTrigger>
      }
    >
      <PanelGroup direction='horizontal'>
        <Panel
          className='flex flex-col gap-2 p-2'
          style={{ overflowY: 'auto' }}
          defaultSize={20}
          minSize={10}
          maxSize={40}
        >
          <h2 className='uppercase'>Overview</h2>
          <CollectionsTree />
        </Panel>
        <PanelResizeHandle direction='horizontal' />
        <Panel className='h-full !overflow-auto'>
          <Outlet />
        </Panel>
      </PanelGroup>
    </DashboardLayout>
  );
};
