import { createFileRoute } from '@tanstack/react-router';
import { FlowHistoryPage } from '../../../history';

export const Route = createFileRoute(
  '/(dashboard)/(workspace)/workspace/$workspaceIdCan/(flow)/flow/$flowIdCan/history',
)({
  component: FlowHistoryPage,
});
