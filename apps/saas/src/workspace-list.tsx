import { createQueryOptions, useMutation as useConnectMutation, useTransport } from '@connectrpc/connect-query';
import { Schema } from '@effect/schema';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { createFileRoute } from '@tanstack/react-router';
import { Effect, pipe, Struct } from 'effect';
import { useState } from 'react';
import { Form } from 'react-aria-components';

import { Workspace } from '@the-dev-tools/protobuf/workspace/v1/workspace_pb';
import {
  createWorkspace,
  deleteWorkspace,
  getWorkspaces,
  updateWorkspace,
} from '@the-dev-tools/protobuf/workspace/v1/workspace-WorkspaceService_connectquery';
import { Button, ButtonAsLink } from '@the-dev-tools/ui/button';
import { TextField } from '@the-dev-tools/ui/text-field';

import { Runtime } from './runtime';

export const Route = createFileRoute('/_authorized/_dashboard/')({
  component: Page,
});

class RenameForm extends Schema.Class<RenameForm>('WorkspaceRenameForm')({
  name: Schema.String,
}) {}

function Page() {
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
        <Row key={_.id} workspace={_} />
      ))}
    </div>
  );
}

interface RowProps {
  workspace: Workspace;
}

const Row = ({ workspace }: RowProps) => {
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
                Schema.decode(RenameForm),
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
