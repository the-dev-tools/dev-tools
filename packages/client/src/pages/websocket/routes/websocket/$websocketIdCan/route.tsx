import { createFileRoute } from '@tanstack/react-router';
import { Ulid } from 'id128';

export const Route = createFileRoute(
  '/(dashboard)/(workspace)/workspace/$workspaceIdCan/(websocket)/websocket/$websocketIdCan',
)({
  context: ({ params: { websocketIdCan } }) => {
    const websocketId = Ulid.fromCanonical(websocketIdCan).bytes;
    return { websocketId };
  },
});
