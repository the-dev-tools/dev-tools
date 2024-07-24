import { Schema } from '@effect/schema';
import { createRootRoute, createRoute, createRouter, Outlet, redirect } from '@tanstack/react-router';
import { Cause, Effect, Option } from 'effect';
import { useState } from 'react';

import * as Auth from './auth';

const root = createRootRoute({
  component: () => (
    <div className='flex h-full flex-col items-center justify-center'>
      <Outlet />
    </div>
  ),
});

class LoginSearch extends Schema.Class<LoginSearch>('LoginSearch')({
  redirect: Schema.optional(Schema.String),
}) {}

const login = createRoute({
  getParentRoute: () => root,
  path: 'login',
  validateSearch: Schema.decodeSync(LoginSearch),
  component: () => {
    // eslint-disable-next-line react-hooks/rules-of-hooks
    const [email, setEmail] = useState('');
    const { redirect } = login.useSearch();
    return (
      <>
        <input
          className='border border-black'
          value={email}
          onInput={(event) => void setEmail(event.currentTarget.value)}
        />
        <button
          onClick={async () => {
            const loggedIn = await Auth.login({ email }).pipe(
              Effect.mapError(() => null),
              Effect.runPromise,
            );
            if (loggedIn === null) return;
            router.history.push(redirect ?? dashboard.fullPath);
          }}
        >
          Login
        </button>
      </>
    );
  },
});

const authenticated = createRoute({
  getParentRoute: () => root,
  id: 'authenticated',
  loader: ({ location }) =>
    Effect.gen(function* () {
      const isLoggedIn = yield* Auth.isLoggedIn;
      if (!isLoggedIn) return yield* Effect.fail(new Cause.RuntimeException('Not logged in'));
      return yield* Auth.getInfo;
    }).pipe(Effect.option, Effect.runPromise, async (_) =>
      Option.getOrThrowWith(await _, () =>
        redirect({ to: '/login', search: new LoginSearch({ redirect: location.href }) }),
      ),
    ),
  pendingComponent: () => 'Loading...',
});

const dashboard = createRoute({
  getParentRoute: () => authenticated,
  id: 'dashboard',
  component: () => {
    const userInfo = authenticated.useLoaderData();
    return (
      <>
        <div>The Dev Tools</div>
        <div>{userInfo.email}</div>
        <button
          onClick={async () => {
            await Auth.logout.pipe(Effect.ignoreLogged, Effect.runPromise);
            await router.navigate({ to: '/login' });
            await router.invalidate();
          }}
        >
          Log out
        </button>
        <Outlet />
      </>
    );
  },
});

const dashboardIndex = createRoute({
  getParentRoute: () => dashboard,
  path: '/',
  component: () => 'Dashboard Index',
});

const routeTree = root.addChildren([login, authenticated.addChildren([dashboard.addChildren([dashboardIndex])])]);

export const router = createRouter({ routeTree });

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router;
  }
}
