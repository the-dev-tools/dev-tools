import { createFileRoute } from '@tanstack/react-router';

import { ApiCallHeaderTab } from '../../../../collection';

export const Route = createFileRoute('/_authorized/workspace/$workspaceId/api-call/$apiCallId/headers')({
  component: ApiCallHeaderTab,
});
