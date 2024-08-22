import { createQueryOptions } from '@connectrpc/connect-query';
import { createFileRoute, redirect } from '@tanstack/react-router';

import { getCollection } from '@the-dev-tools/protobuf/collection/v1/collection-CollectionService_connectquery';

import { CollectionPage } from '../../../collection';
import { queryClient, transport } from '../../../runtime';

export const Route = createFileRoute('/_authorized/workspace/$workspaceId/collection/$collectionId')({
  component: CollectionPage,
  loader: async ({ params: { collectionId } }) => {
    const options = createQueryOptions(getCollection, { id: collectionId }, { transport });
    await queryClient.ensureQueryData(options).catch(() => redirect({ to: '../../', throw: true }));
  },
});
