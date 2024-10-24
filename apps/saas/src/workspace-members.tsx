import { Schema } from '@effect/schema';
import { createFileRoute, getRouteApi } from '@tanstack/react-router';
import { Effect, pipe } from 'effect';
import { Form } from 'react-aria-components';

import { useCreateMutation } from '@the-dev-tools/api/query';
import {
  workspaceMemberCreate,
  workspaceMemberList,
} from '@the-dev-tools/spec/workspace/v1/workspace-WorkspaceService_connectquery';
import { Button } from '@the-dev-tools/ui/button';
import { TextField } from '@the-dev-tools/ui/text-field';

import { Runtime } from './runtime';

export const Route = createFileRoute('/_authorized/workspace/$workspaceIdCan/members')({
  component: Page,
});

const workspaceRoute = getRouteApi('/_authorized/workspace/$workspaceIdCan');

class InviteForm extends Schema.Class<InviteForm>('WorkspaceInviteForm')({
  email: Schema.String,
}) {}

function Page() {
  const { workspaceId } = workspaceRoute.useLoaderData();

  const workspaceMemberCreateMutation = useCreateMutation(workspaceMemberCreate, {
    key: 'memberId',
    listQuery: workspaceMemberList,
    listInput: { workspaceId },
  });

  return (
    <div className='p-4'>
      <h2 className='text-center text-2xl font-extrabold'>Members</h2>
      <Form
        className='flex flex-col items-start gap-2'
        onSubmit={(event) =>
          Effect.gen(function* () {
            event.preventDefault();

            const { email } = yield* pipe(
              new FormData(event.currentTarget),
              Object.fromEntries,
              Schema.decode(InviteForm),
            );

            workspaceMemberCreateMutation.mutate({ workspaceId, email });
          }).pipe(Runtime.runPromise)
        }
      >
        <TextField name='email' type='email' isRequired label='Invite new member:' inputPlaceholder='Email' />
        <Button kind='placeholder' variant='placeholder' type='submit'>
          Send invite
        </Button>
      </Form>
    </div>
  );
}
