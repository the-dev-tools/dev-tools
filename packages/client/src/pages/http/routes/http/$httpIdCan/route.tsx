import { createFileRoute } from '@tanstack/react-router';
import { Ulid } from 'id128';

export const Route = createFileRoute('/(dashboard)/(workspace)/workspace/$workspaceIdCan/(http)/http/$httpIdCan')({
  context: ({ params: { httpIdCan } }) => {
    const httpId = Ulid.fromCanonical(httpIdCan).bytes;
    return { httpId };
  },
});
