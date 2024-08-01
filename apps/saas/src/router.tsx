import { Schema } from '@effect/schema';
import { createRootRoute, createRoute, createRouter, redirect } from '@tanstack/react-router';
import { Effect, Option, pipe } from 'effect';

import * as Auth from '@the-dev-tools/api/auth';

import { CollectionListPage } from './collection';
import { DashboardLayout } from './dashboard';
import { LoginPage } from './login';
import { Runtime } from './runtime';

const root = createRootRoute();

class LoginSearch extends Schema.Class<LoginSearch>('LoginSearch')({
  redirect: Schema.optional(Schema.String),
}) {}

const login = createRoute({
  getParentRoute: () => root,
  path: 'login',
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
  component: CollectionListPage,
});

const routeTree = root.addChildren([login, authenticated.addChildren([dashboard.addChildren([dashboardIndex])])]);

export const router = createRouter({ routeTree });

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router;
  }
}
