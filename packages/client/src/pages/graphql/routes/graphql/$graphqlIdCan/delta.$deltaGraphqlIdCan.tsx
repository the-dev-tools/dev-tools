import { createFileRoute } from '@tanstack/react-router';
import { Ulid } from 'id128';
import { openTab } from '~/widgets/tabs';
import { GraphQLDeltaPage } from '../../../page';
import { GraphQLTab, graphqlTabId } from '../../../tab';

export const Route = createFileRoute(
  '/(dashboard)/(workspace)/workspace/$workspaceIdCan/(graphql)/graphql/$graphqlIdCan/delta/$deltaGraphqlIdCan',
)({
  component: GraphQLDeltaPage,
  context: ({ params: { deltaGraphqlIdCan } }) => {
    const deltaGraphqlId = Ulid.fromCanonical(deltaGraphqlIdCan).bytes;
    return { deltaGraphqlId };
  },
  onEnter: async (match) => {
    const { deltaGraphqlId, graphqlId } = match.context;

    await openTab({
      id: graphqlTabId({ deltaGraphqlId, graphqlId }),
      match,
      node: <GraphQLTab deltaGraphqlId={deltaGraphqlId} graphqlId={graphqlId} />,
    });
  },
});
