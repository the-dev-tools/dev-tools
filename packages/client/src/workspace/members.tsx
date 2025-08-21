import { createFileRoute, getRouteApi, useRouteContext } from '@tanstack/react-router';
import { Effect, pipe, Runtime, Schema } from 'effect';
import { Form } from 'react-aria-components';
import { WorkspaceMemberCreateEndpoint } from '@the-dev-tools/spec/meta/workspace/v1/workspace.endpoints.ts';
import { Button } from '@the-dev-tools/ui/button';
import { TextField } from '@the-dev-tools/ui/text-field';

export const Route = createFileRoute('/_authorized/workspace/$workspaceIdCan/members')({ component: Page });

const workspaceRoute = getRouteApi('/_authorized/workspace/$workspaceIdCan');

class InviteForm extends Schema.Class<InviteForm>('WorkspaceInviteForm')({
  email: Schema.String,
}) {}

function Page() {
  const { workspaceId } = workspaceRoute.useLoaderData();

  const { dataClient, runtime } = useRouteContext({ from: '__root__' });

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

            void dataClient.fetch(WorkspaceMemberCreateEndpoint, { email, workspaceId });
          }).pipe(Runtime.runPromise(runtime))
        }
      >
        <TextField inputPlaceholder='Email' isRequired label='Invite new member:' name='email' type='email' />
        <Button type='submit'>Send invite</Button>
      </Form>
    </div>
  );
}
