import { createFileRoute } from '@tanstack/react-router';

import { ApiCallPage } from '../../../collection';

export const Route = createFileRoute('/_authorized/workspace/$workspaceId/api-call/$apiCallId')({
  component: ApiCallPage,
});
