import { Effect, pipe, Runtime, Schema } from 'effect';
import { Form } from 'react-aria-components';
import { WorkspaceMemberCreateEndpoint } from '@the-dev-tools/spec/data-client/workspace/v1/workspace.endpoints.ts';
import { Button } from '@the-dev-tools/ui/button';
import { TextInputField } from '@the-dev-tools/ui/text-field';
import { rootRouteApi, workspaceRouteApi } from '~routes';

class InviteForm extends Schema.Class<InviteForm>('WorkspaceInviteForm')({
  email: Schema.String,
}) {}

export function Page() {
  const { workspaceId } = workspaceRouteApi.useLoaderData();

  const { dataClient, runtime } = rootRouteApi.useRouteContext();

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
        <TextInputField isRequired label='Invite new member:' name='email' placeholder='Email' type='email' />
        <Button type='submit'>Send invite</Button>
      </Form>
    </div>
  );
}
