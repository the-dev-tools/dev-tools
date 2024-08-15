import { Schema } from '@effect/schema';
import { getRouteApi, useRouter } from '@tanstack/react-router';
import { Effect, pipe } from 'effect';
import { FieldError, Form, Input, Label, TextField } from 'react-aria-components';

import * as Auth from '@the-dev-tools/api/auth';
import { Button } from '@the-dev-tools/ui/button';
import { Logo } from '@the-dev-tools/ui/illustrations';

import { Runtime } from './runtime';

const route = getRouteApi('/login');

class LoginFormData extends Schema.Class<LoginFormData>('LoginFormData')({
  email: Schema.String,
}) {}

export const LoginPage = () => {
  const router = useRouter();
  const { redirect } = route.useSearch();
  return (
    <Form
      className='flex size-full flex-col items-center justify-center gap-4'
      onSubmit={(event) => {
        void Effect.gen(function* () {
          event.preventDefault();
          const { email } = yield* pipe(
            new FormData(event.currentTarget),
            Object.fromEntries,
            Schema.decode(LoginFormData),
          );

          const loginResult = yield* Auth.login({ email }).pipe(
            Effect.catchTag('NoOrganizationSelectedError', Effect.succeed),
          );

          if (loginResult?._tag === 'NoOrganizationSelectedError') {
            yield* Effect.tryPromise(() => router.navigate({ to: '/organizations', search: { redirect } }));
          } else if (redirect) {
            router.history.push(redirect);
          } else {
            yield* Effect.tryPromise(() => router.navigate({ to: '/' }));
          }

          queueMicrotask(() => void location.reload());
        }).pipe(Runtime.runPromise);
      }}
    >
      <Logo className='mb-2 h-16 w-auto' />
      <TextField name='email'>
        <Label className='mb-2 block'>Email</Label>
        <Input className='border border-black' />
        <FieldError className='mt-2 text-red-700' />
      </TextField>
      <Button type='submit'>Login</Button>
    </Form>
  );
};
