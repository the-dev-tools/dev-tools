import { index, layout, rootRoute, route } from '@tanstack/virtual-file-routes';

export const routes = rootRoute('root.tsx', [
  route('login', 'login.tsx'),
  layout('authorized', 'authorized.tsx', [
    layout('dashboard', 'dashboard.tsx', [index('workspace-list.tsx')]),
    route('workspace/$workspaceIdCan', 'workspace-layout.tsx', [
      route('members', 'workspace-members.tsx'),
      route('endpoint/$endpointIdCan/example/$exampleIdCan', 'endpoint.tsx'),
      route('flow/$flowIdCan', 'flow.tsx'),
    ]),
  ]),
]);
