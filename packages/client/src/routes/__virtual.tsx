import { index, rootRoute, route } from '@tanstack/virtual-file-routes';

export const routes = rootRoute('root.tsx', [
  index('welcome.tsx'),
  route('workspace/$workspaceIdCan', 'workspace.tsx', [
    index('overview.tsx'),
    route('http/$httpIdCan', 'http.tsx'),
    // route('flow/$flowIdCan', 'flow/layout.tsx', [index('flow/edit.tsx'), route('history', 'flow/history.tsx')]),
  ]),
]);
