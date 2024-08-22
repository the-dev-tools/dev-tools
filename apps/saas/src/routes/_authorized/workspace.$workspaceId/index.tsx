import { createFileRoute } from '@tanstack/react-router';

import { CollectionsPage } from '../../../collection';

export const Route = createFileRoute('/_authorized/workspace/$workspaceId/')({
  component: CollectionsPage,
});
