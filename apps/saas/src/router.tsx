import { createQueryOptions } from '@connectrpc/connect-query';
import { Schema } from '@effect/schema';
import { createRootRoute, createRoute, createRouter, redirect } from '@tanstack/react-router';
import { Effect, Option, pipe } from 'effect';

import * as Auth from '@the-dev-tools/api/auth';
import * as CollectionQuery from '@the-dev-tools/protobuf/collection/v1/collection-CollectionService_connectquery';

import { CollectionEditPage, CollectionListPage } from './collection';
import { DashboardIndexPage, DashboardLayout } from './dashboard';
import { LoginPage } from './login';
import { queryClient, Runtime, transport } from './runtime';

const root = createRootRoute();

class LoginSearch extends Schema.Class<LoginSearch>('LoginSearch')({
  redirect: Schema.optional(Schema.String),
}) {}

const login = createRoute({
  getParentRoute: () => root,
  path: '/login',
  validateSearch: Schema.decodeSync(LoginSearch),
  component: LoginPage,
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
  component: DashboardLayout,
});

const dashboardIndex = createRoute({
  getParentRoute: () => dashboard,
  path: '/',
  component: DashboardIndexPage,
});

const collectionList = createRoute({
  getParentRoute: () => dashboard,
  path: '/collections',
  component: CollectionListPage,
});

const collectionEdit = createRoute({
  getParentRoute: () => dashboard,
  path: '/collection/$id',
  component: CollectionEditPage,
  loader: async ({ params: { id } }) => {
    const options = createQueryOptions(CollectionQuery.getCollection, { id }, { transport });
    await queryClient.ensureQueryData(options).catch(() => redirect({ to: '/collections', throw: true }));
  },
});

const routeTree = root.addChildren([
  login,
  authenticated.addChildren([dashboard.addChildren([dashboardIndex, collectionList, collectionEdit])]),
]);

export const router = createRouter({ routeTree });

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router;
  }
}
