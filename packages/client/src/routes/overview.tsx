import { createFileRoute } from '@tanstack/react-router';
import { OverviewPage } from '~/workspace/overview';

export const Route = createFileRoute('/workspace/$workspaceIdCan/')({
  component: OverviewPage,
});
