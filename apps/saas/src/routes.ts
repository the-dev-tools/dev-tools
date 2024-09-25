import { index, layout, rootRoute, route } from '@tanstack/virtual-file-routes';

export const routes = rootRoute('root.tsx', [
  route('login', 'login.tsx'),
  layout('authorized', 'authorized.tsx', [
    layout('dashboard', 'dashboard.tsx', [index('workspace-list.tsx')]),
    route('workspace/$workspaceId', 'workspace-layout.tsx', [
      route('members', 'workspace-members.tsx'),
      route('api-call/$apiCallId/example/$exampleId', 'api-call.tsx', [
        index('query.tsx'),
        route('headers', 'headers.tsx'),
        route('body', 'body.tsx'),
      ]),
    ]),
  ]),
]);
