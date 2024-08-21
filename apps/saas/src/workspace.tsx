import {
  createQueryOptions,
  useMutation as useConnectMutation,
  useQuery as useConnectQuery,
  useTransport,
} from '@connectrpc/connect-query';
import { Schema } from '@effect/schema';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { getRouteApi, Link } from '@tanstack/react-router';
import { Effect, pipe, Struct } from 'effect';
import { useState } from 'react';
import { Button, Form, Input, TextField } from 'react-aria-components';

import { Workspace } from '@the-dev-tools/protobuf/workspace/v1/workspace_pb';
import {
  createWorkspace,
  deleteWorkspace,
  getWorkspace,
  getWorkspaces,
  updateWorkspace,
} from '@the-dev-tools/protobuf/workspace/v1/workspace-WorkspaceService_connectquery';

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
          <TextField aria-label='Workspace name' name='name' defaultValue={workspace.name} autoFocus>
            <Input />
          </TextField>
          <Button type='submit'>Save</Button>
        </Form>
      ) : (
        <>
          <Link to='/workspace/$workspaceId' params={{ workspaceId: workspace.id }}>
            {workspace.name}
          </Link>
          <Button onPress={() => void setRenaming(true)}>Rename</Button>
        </>
      )}
      <Button
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

const workspaceRoute = getRouteApi('/authorized/workspace/$workspaceId');

export const WorkspaceLayout = () => {
  const { workspaceId } = workspaceRoute.useParams();

  const query = useConnectQuery(getWorkspace, { id: workspaceId });
  if (!query.isSuccess) return;
  const { workspace } = query.data;

  return (
    <DashboardLayout>
      <Link to='/workspace/$workspaceId/members' params={{ workspaceId }}>
        Manage members
      </Link>
      <Link to='/workspace/$workspaceId' params={{ workspaceId }}>
        {workspace!.name}
      </Link>
    </DashboardLayout>
  );
};
