import { createQueryOptions } from '@connectrpc/connect-query';
import { createFileRoute, redirect } from '@tanstack/react-router';

import { getWorkspace } from '@the-dev-tools/protobuf/workspace/v1/workspace-WorkspaceService_connectquery';

import { queryClient, transport } from '../../../runtime';
import { WorkspaceLayout } from '../../../workspace';

export const Route = createFileRoute('/_authorized/workspace/$workspaceId')({
  component: WorkspaceLayout,
  loader: async ({ params: { workspaceId } }) => {
    const options = createQueryOptions(getWorkspace, { id: workspaceId }, { transport });
    await queryClient.ensureQueryData(options).catch(() => redirect({ to: '/', throw: true }));
  },
});
