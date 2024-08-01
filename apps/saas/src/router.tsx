import { useQuery } from '@connectrpc/connect-query';
import { Schema } from '@effect/schema';
import { createRootRoute, createRoute, createRouter, Outlet, redirect } from '@tanstack/react-router';
import { Effect, Option, pipe } from 'effect';
import { useState } from 'react';

import * as Auth from '@the-dev-tools/api/auth';
import * as CollectionQuery from '@the-dev-tools/protobuf/collection/v1/collection-CollectionService_connectquery';
import { Button } from '@the-dev-tools/ui/button';

import { Runtime } from './runtime';

const root = createRootRoute();

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
          onClick={() =>
            Effect.gen(function* () {
              yield* Auth.login({ email });
              router.history.push(redirect ?? dashboard.fullPath);
              queueMicrotask(() => void location.reload());
            }).pipe(Runtime.runPromise)
          }
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
    pipe(Effect.option(Auth.getUser), Runtime.runPromise, async (_) =>
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
      <div className='flex h-full'>
        <div className='flex h-full w-80 flex-col overflow-auto border-r-4 border-black p-2'>
          <h1 className='text-center text-2xl font-extrabold'>The Dev Tools</h1>
          <div className='flex-1' />
          <div className='mb-1'>User: {userInfo.email}</div>
          <Button
            onPress={async () => {
              await Auth.logout.pipe(Effect.ignoreLogged, Runtime.runPromise);
              queueMicrotask(() => void location.reload());
            }}
          >
            Log out
          </Button>
        </div>
        <div className='h-full flex-1 overflow-auto p-2'>
          <Outlet />
        </div>
      </div>
    );
  },
});

const dashboardIndex = createRoute({
  getParentRoute: () => dashboard,
  path: '/',
  component: () => {
    // eslint-disable-next-line react-hooks/rules-of-hooks
    const collections = useQuery(CollectionQuery.listCollections);
    return (
      <>
        <h2 className='text-center text-2xl font-extrabold'>Collections</h2>
        <div>{collections.error?.code}</div>
        <div>{JSON.stringify(collections.data)}</div>
      </>
    );
  },
});

const routeTree = root.addChildren([login, authenticated.addChildren([dashboard.addChildren([dashboardIndex])])]);

export const router = createRouter({ routeTree });

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router;
  }
}
