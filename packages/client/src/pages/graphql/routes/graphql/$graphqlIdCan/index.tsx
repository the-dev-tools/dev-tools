import { createFileRoute } from '@tanstack/react-router';
import { openTab } from '~/widgets/tabs';
import { GraphQLPage } from '../../../page';
import { GraphQLTab, graphqlTabId } from '../../../tab';

export const Route = createFileRoute(
  '/(dashboard)/(workspace)/workspace/$workspaceIdCan/(graphql)/graphql/$graphqlIdCan/',
)({
  component: GraphQLPage,
  onEnter: async (match) => {
    const { graphqlId } = match.context;

    await openTab({
      id: graphqlTabId({ graphqlId }),
      match,
      node: <GraphQLTab graphqlId={graphqlId} />,
    });
  },
});
