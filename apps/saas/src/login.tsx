import { Schema } from '@effect/schema';
import { getRouteApi, useRouter } from '@tanstack/react-router';
import { Effect, pipe } from 'effect';
import { Form } from 'react-aria-components';

import * as Auth from '@the-dev-tools/api/auth';
import { Button } from '@the-dev-tools/ui/button';
import { Logo } from '@the-dev-tools/ui/illustrations';
import { TextField } from '@the-dev-tools/ui/text-field';

import { Runtime } from './runtime';

const route = getRouteApi('/login');

class LoginForm extends Schema.Class<LoginForm>('LoginForm')({
  email: Schema.String,
}) {}

export const LoginPage = () => {
  const router = useRouter();
  const { redirect } = route.useSearch();
  return (
    <div className='mx-auto h-full max-w-64'>
      <Form
        className='flex size-full flex-col justify-center gap-4'
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
              yield* Effect.tryPromise(() => router.navigate({ to: '/' }));
            }

            queueMicrotask(() => void location.reload());
          }).pipe(Runtime.runPromise);
        }}
      >
        <Logo className='h-16 w-auto' />
        <TextField name='email' type='email' isRequired label='Email' />
        <Button kind='placeholder' variant='placeholder' type='submit'>
          Login
        </Button>
      </Form>
    </div>
  );
};
