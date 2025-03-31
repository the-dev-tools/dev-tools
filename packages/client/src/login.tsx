import { createFileRoute, useRouter } from '@tanstack/react-router';
import { Effect, pipe, Runtime, Schema } from 'effect';
import { Form } from 'react-aria-components';

import { Button } from '@the-dev-tools/ui/button';
import { Logo } from '@the-dev-tools/ui/illustrations';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextField } from '@the-dev-tools/ui/text-field';
import * as Auth from '~/api/auth';

export class LoginSearch extends Schema.Class<LoginSearch>('LoginSearch')({
  redirect: Schema.optional(Schema.String),
}) {}

const makeRoute = createFileRoute('/login');

export const Route = makeRoute({
  validateSearch: Schema.decodeSync(LoginSearch),
  component: LoginPage,
});

class LoginForm extends Schema.Class<LoginForm>('LoginForm')({
  email: Schema.String,
}) {}

function LoginPage() {
  const router = useRouter();
  const { redirect } = Route.useSearch();
  const { runtime } = Route.useRouteContext();
  return (
    <div className='container mx-auto flex h-full flex-col items-center text-center'>
      <Logo className={tw`mb-10 mt-24 h-16 w-auto`} />

      <h1 className={tw`mb-1 text-xl font-semibold leading-6 tracking-tight text-slate-800`}>Welcome to DevTools</h1>
      <span className={tw`text-md mb-6 leading-5 tracking-tight text-slate-500`}>
        Please enter your account details
      </span>

      <Form
        className='w-80'
        onSubmit={(event) => {
          void Effect.gen(function* () {
            event.preventDefault();
            const { email } = yield* pipe(
              new FormData(event.currentTarget),
              Object.fromEntries,
              Schema.decode(LoginForm),
            );

            yield* Auth.login({ email });

            if (redirect) {
              router.history.push(redirect);
            } else {
              yield* Effect.tryPromise(async () => router.navigate({ to: '/' }));
            }

            queueMicrotask(() => void location.reload());
          }).pipe(Runtime.runPromise(runtime));
        }}
      >
        <TextField
          className={tw`mb-4`}
          inputPlaceholder='Enter email...'
          isRequired
          label='Email'
          labelClassName={tw`mb-1 text-start text-sm font-medium leading-5 tracking-tight text-slate-800`}
          name='email'
          type='email'
        />

        <Button className={tw`w-full py-2.5`} type='submit' variant='primary'>
          Login
        </Button>
      </Form>

      <div className={tw`h-10 flex-1`} />

      {/* TODO: implement TOS */}
      <span className={tw`mb-10 text-sm leading-5 text-slate-500`}>
        By clicking “Login” you agree with our{' '}
        <a className={tw`text-violet-600 underline`} href='.'>
          Terms of use
        </a>{' '}
        and{' '}
        <a className={tw`text-violet-600 underline`} href='.'>
          Privacy policy
        </a>
      </span>
    </div>
  );
}
