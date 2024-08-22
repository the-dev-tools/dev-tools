import { createFileRoute } from '@tanstack/react-router';

import { WorkspacesPage } from '../../../workspace';

export const Route = createFileRoute('/_authorized/_dashboard/')({
  component: WorkspacesPage,
});
