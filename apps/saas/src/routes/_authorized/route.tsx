import { createFileRoute, redirect } from '@tanstack/react-router';
import { Effect, Option, pipe } from 'effect';

import { getUser } from '@the-dev-tools/api/auth';

import { Runtime } from '../../runtime';
import { LoginSearch } from '../login';

export const Route = createFileRoute('/_authorized')({
  beforeLoad: ({ location }) =>
    pipe(Effect.option(getUser), Runtime.runPromise, async (_) =>
      Option.getOrThrowWith(await _, () =>
        redirect({ to: '/login', search: new LoginSearch({ redirect: location.href }) }),
      ),
    ),
  pendingComponent: () => 'Loading user...',
});
