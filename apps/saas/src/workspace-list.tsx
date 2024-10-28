import { useQuery as useConnectQuery } from '@connectrpc/connect-query';
import { createFileRoute } from '@tanstack/react-router';
import { Effect, pipe, Schema } from 'effect';
import { Ulid } from 'id128';
import { useState } from 'react';
import { Form } from 'react-aria-components';

import { useSpecMutation } from '@the-dev-tools/api/query';
import { workspaceCreateSpec, workspaceDeleteSpec, workspaceUpdateSpec } from '@the-dev-tools/api/spec/workspace';
import { WorkspaceListItem } from '@the-dev-tools/spec/workspace/v1/workspace_pb';
import { workspaceList } from '@the-dev-tools/spec/workspace/v1/workspace-WorkspaceService_connectquery';
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
  const workspaceListQuery = useConnectQuery(workspaceList);
  const workspaceCreateMutation = useSpecMutation(workspaceCreateSpec);

  if (!workspaceListQuery.isSuccess) return null;
  const { items: workspaces } = workspaceListQuery.data;

  return (
    <div className='flex size-full flex-col items-center justify-center gap-4'>
      <div>
        <Button
          kind='placeholder'
          variant='placeholder'
          onPress={() => void workspaceCreateMutation.mutate({ name: 'New workspace' })}
        >
          Create workspace
        </Button>
      </div>

      {workspaces.map((_) => {
        const workspaceIdCan = Ulid.construct(_.workspaceId).toCanonical();
        return <Row key={workspaceIdCan} workspaceIdCan={workspaceIdCan} workspace={_} />;
      })}
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

              workspaceUpdateMutation.mutate({ workspaceId, name });

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
            href={{ to: '/workspace/$workspaceIdCan', params: { workspaceIdCan } }}
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
        onPress={() => void workspaceDeleteMutation.mutate({ workspaceId })}
      >
        Delete
      </Button>
    </div>
  );
};
