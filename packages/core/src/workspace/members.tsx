import { createFileRoute, getRouteApi } from '@tanstack/react-router';
import { Effect, pipe, Runtime, Schema } from 'effect';
import { Form } from 'react-aria-components';

import { useConnectMutation } from '@the-dev-tools/api/connect-query';
import { workspaceMemberCreate } from '@the-dev-tools/spec/workspace/v1/workspace-WorkspaceService_connectquery';
import { Button } from '@the-dev-tools/ui/button';
import { TextField } from '@the-dev-tools/ui/text-field';

const makeRoute = createFileRoute('/_authorized/workspace/$workspaceIdCan/members');

export const Route = makeRoute({ component: Page });

const workspaceRoute = getRouteApi('/_authorized/workspace/$workspaceIdCan');

class InviteForm extends Schema.Class<InviteForm>('WorkspaceInviteForm')({
  email: Schema.String,
}) {}

function Page() {
  const { workspaceId } = workspaceRoute.useLoaderData();
  const { runtime } = workspaceRoute.useRouteContext();

  const workspaceMemberCreateMutation = useConnectMutation(workspaceMemberCreate);

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

            workspaceMemberCreateMutation.mutate({ email, workspaceId });
          }).pipe(Runtime.runPromise(runtime))
        }
      >
        <TextField inputPlaceholder='Email' isRequired label='Invite new member:' name='email' type='email' />
        <Button type='submit'>Send invite</Button>
      </Form>
    </div>
  );
}
