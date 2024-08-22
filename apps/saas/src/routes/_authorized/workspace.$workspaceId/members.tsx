import { createFileRoute } from '@tanstack/react-router';

import { MembersPage } from '../../../members';

export const Route = createFileRoute('/_authorized/workspace/$workspaceId/members')({
  component: MembersPage,
});
