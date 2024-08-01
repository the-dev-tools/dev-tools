import { getRouteApi, useRouter } from '@tanstack/react-router';
import { Effect } from 'effect';
import { useState } from 'react';

import * as Auth from '@the-dev-tools/api/auth';

import { Runtime } from './runtime';

const route = getRouteApi('/login');
const dashboardRoute = getRouteApi('/authenticated/dashboard');

export const LoginPage = () => {
  const [email, setEmail] = useState('');
  const router = useRouter();
  const { redirect } = route.useSearch();
  return (
    <>
      <input
        className='border border-black'
        value={email}
        onInput={(event) => void setEmail(event.currentTarget.value)}
      />
      <button
        onClick={() =>
          Effect.gen(function* () {
            yield* Auth.login({ email });
            router.history.push(redirect ?? dashboardRoute.id);
            queueMicrotask(() => void location.reload());
          }).pipe(Runtime.runPromise)
        }
      >
        Login
      </button>
    </>
  );
};
