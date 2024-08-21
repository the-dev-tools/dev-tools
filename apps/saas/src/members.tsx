import { useMutation as useConnectMutation } from '@connectrpc/connect-query';
import { Schema } from '@effect/schema';
import { getRouteApi } from '@tanstack/react-router';
import { Effect, pipe } from 'effect';
import { Button, Form, Input, Label, TextField } from 'react-aria-components';

import { inviteUser } from '@the-dev-tools/protobuf/workspace/v1/workspace-WorkspaceService_connectquery';

const membersRoute = getRouteApi('/authorized/workspace/$workspaceId/members');

class InviteForm extends Schema.Class<InviteForm>('InviteForm')({
  email: Schema.String,
}) {}

export const MembersPage = () => {
  const { workspaceId } = membersRoute.useParams();

  const inviteUserMutation = useConnectMutation(inviteUser);

  return (
    <>
      <h2 className='text-center text-2xl font-extrabold'>Members</h2>
      <Form
        className='flex gap-2'
        onSubmit={(event) =>
          Effect.gen(function* () {
            event.preventDefault();

            const { email } = yield* pipe(
              new FormData(event.currentTarget),
              Object.fromEntries,
              Schema.decode(InviteForm),
            );

            yield* Effect.tryPromise(() => inviteUserMutation.mutateAsync({ workspaceId, email }));
          })
        }
      >
        <TextField name='email' type='email' isRequired className='contents'>
          <Label>Invite new member:</Label>
          <Input placeholder='Email' />
        </TextField>
        <Button type='submit'>Send invite</Button>
      </Form>
    </>
  );
};
