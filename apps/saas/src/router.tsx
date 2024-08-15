import { createQueryOptions } from '@connectrpc/connect-query';
import { Schema } from '@effect/schema';
import { createRootRoute, createRoute, createRouter, redirect } from '@tanstack/react-router';
import { Effect, Option, pipe } from 'effect';

import * as Auth from '@the-dev-tools/api/auth';
import * as CollectionQuery from '@the-dev-tools/protobuf/collection/v1/collection-CollectionService_connectquery';

import { ApiCallEditPage, CollectionEditPage, CollectionListPage } from './collection';
import { DashboardIndexPage, DashboardLayout } from './dashboard';
import { LoginPage } from './login';
import { OrganizationsPage } from './organization';
import { queryClient, Runtime, transport } from './runtime';

const root = createRootRoute();

class RedirectSearch extends Schema.Class<RedirectSearch>('RedirectSearch')({
  redirect: Schema.optional(Schema.String),
}) {}

const login = createRoute({
  getParentRoute: () => root,
  path: '/login',
  validateSearch: Schema.decodeSync(RedirectSearch),
  component: LoginPage,
});

const user = createRoute({
  getParentRoute: () => root,
  id: 'user',
  beforeLoad: ({ location }) =>
    pipe(Effect.option(Auth.getUser), Runtime.runPromise, async (_) =>
      Option.getOrThrowWith(await _, () =>
        redirect({ to: '/login', search: new RedirectSearch({ redirect: location.href }) }),
      ),
    ),
  pendingComponent: () => 'Loading user...',
});

const organizations = createRoute({
  getParentRoute: () => user,
  path: '/organizations',
  validateSearch: Schema.decodeSync(RedirectSearch),
  component: OrganizationsPage,
});

const organization = createRoute({
  getParentRoute: () => user,
  id: 'org',
  beforeLoad: ({ location }) =>
    pipe(
      Auth.getOrganizationId,
      Effect.map((_) => ({ organizationId: _ })),
      Effect.option,
      Runtime.runPromise,
      async (_) =>
        Option.getOrThrowWith(await _, () =>
          redirect({ to: '/organizations', search: new RedirectSearch({ redirect: location.href }) }),
        ),
    ),
  pendingComponent: () => 'Loading organization...',
});

const dashboard = createRoute({
  getParentRoute: () => organization,
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

const apiCallEdit = createRoute({
  getParentRoute: () => dashboard,
  path: '/api-call/$id',
  component: ApiCallEditPage,
});

const routeTree = root.addChildren([
  login,
  user.addChildren([
    organizations,
    organization.addChildren([dashboard.addChildren([dashboardIndex, collectionList, collectionEdit, apiCallEdit])]),
  ]),
]);

export const router = createRouter({ routeTree });

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router;
  }
}
