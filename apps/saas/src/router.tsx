import { createQueryOptions } from '@connectrpc/connect-query';
import { Schema } from '@effect/schema';
import { createRootRoute, createRoute, createRouter, redirect } from '@tanstack/react-router';
import { Effect, Option, pipe } from 'effect';

import { getUser } from '@the-dev-tools/api/auth';
import { getCollection } from '@the-dev-tools/protobuf/collection/v1/collection-CollectionService_connectquery';
import { getWorkspace } from '@the-dev-tools/protobuf/workspace/v1/workspace-WorkspaceService_connectquery';

import { ApiCallPage, CollectionPage, CollectionsPage } from './collection';
import { DashboardLayout } from './dashboard';
import { LoginPage } from './login';
import { queryClient, Runtime, transport } from './runtime';
import { WorkspaceLayout, WorkspacesPage } from './workspace';

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

const authorized = createRoute({
  getParentRoute: () => root,
  id: 'authorized',
  beforeLoad: ({ location }) =>
    pipe(Effect.option(getUser), Runtime.runPromise, async (_) =>
      Option.getOrThrowWith(await _, () =>
        redirect({ to: '/login', search: new RedirectSearch({ redirect: location.href }) }),
      ),
    ),
  pendingComponent: () => 'Loading user...',
});

const dashboard = createRoute({
  getParentRoute: () => authorized,
  id: 'dashboard',
  component: DashboardLayout,
});

const workspaces = createRoute({
  getParentRoute: () => dashboard,
  path: '/',
  component: WorkspacesPage,
});

const workspace = createRoute({
  getParentRoute: () => authorized,
  path: '/workspace/$workspaceId',
  component: WorkspaceLayout,
  loader: async ({ params: { workspaceId } }) => {
    const options = createQueryOptions(getWorkspace, { id: workspaceId }, { transport });
    await queryClient.ensureQueryData(options).catch(() => redirect({ to: '/', throw: true }));
  },
});

const collections = createRoute({
  getParentRoute: () => workspace,
  path: '/',
  component: CollectionsPage,
});

const collection = createRoute({
  getParentRoute: () => workspace,
  path: '/collection/$collectionId',
  component: CollectionPage,
  loader: async ({ params: { collectionId } }) => {
    const options = createQueryOptions(getCollection, { id: collectionId }, { transport });
    await queryClient.ensureQueryData(options).catch(() => redirect({ to: '../../', throw: true }));
  },
});

const apiCall = createRoute({
  getParentRoute: () => workspace,
  path: '/api-call/$apiCallId',
  component: ApiCallPage,
});

const routeTree = root.addChildren([
  login,
  authorized.addChildren([
    dashboard.addChildren([workspaces]),
    workspace.addChildren([collections, collection, apiCall]),
  ]),
]);

export const router = createRouter({ routeTree });

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router;
  }
}
