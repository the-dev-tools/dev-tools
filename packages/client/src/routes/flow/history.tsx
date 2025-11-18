import { createFileRoute } from '@tanstack/react-router';
import { FlowHistoryPage } from '~/features/flow';

export const Route = createFileRoute('/workspace/$workspaceIdCan/flow/$flowIdCan/history')({
  component: FlowHistoryPage,
});
