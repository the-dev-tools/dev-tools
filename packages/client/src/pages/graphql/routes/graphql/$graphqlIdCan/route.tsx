import { createFileRoute } from '@tanstack/react-router';
import { Ulid } from 'id128';

export const Route = createFileRoute(
  '/(dashboard)/(workspace)/workspace/$workspaceIdCan/(graphql)/graphql/$graphqlIdCan',
)({
  context: ({ params: { graphqlIdCan } }) => {
    const graphqlId = Ulid.fromCanonical(graphqlIdCan).bytes;
    return { graphqlId };
  },
});
