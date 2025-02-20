import { createFileRoute } from '@tanstack/react-router';
import { Ulid } from 'id128';

export const Route = createFileRoute('/_authorized/workspace/$workspaceIdCan/flow/$flowIdCan')({
  loader: ({ params: { flowIdCan } }) => {
    const flowId = Ulid.fromCanonical(flowIdCan).bytes;
    return { flowId };
  },
});
