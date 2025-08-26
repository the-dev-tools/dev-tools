import { createFileRoute } from '@tanstack/react-router';
import { FlowHistoryPage } from '~flow/history';

export const Route = createFileRoute('/workspace/$workspaceIdCan/flow/$flowIdCan/history')({
  component: FlowHistoryPage,
});
