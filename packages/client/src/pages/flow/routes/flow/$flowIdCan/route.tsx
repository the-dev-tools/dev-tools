import { QueryErrorResetBoundary } from '@tanstack/react-query';
import { createFileRoute, ErrorComponent, Outlet } from '@tanstack/react-router';
import { Ulid } from 'id128';
import { openTab } from '~/widgets/tabs';
import { FlowTab, flowTabId } from '../../../tab';

/* eslint-disable perfectionist/sort-objects */
export const Route = createFileRoute('/(dashboard)/(workspace)/workspace/$workspaceIdCan/(flow)/flow/$flowIdCan')({
  loader: ({ params: { flowIdCan } }) => {
    const flowId = Ulid.fromCanonical(flowIdCan).bytes;
    return { flowId };
  },
  component: RouteComponent,
  errorComponent: (props) => <ErrorComponent {...props} />,
  onEnter: async (match) => {
    if (!match.loaderData) return;

    const { flowId } = match.loaderData;

    await openTab({
      id: flowTabId(flowId),
      match,
      node: <FlowTab flowId={flowId} />,
    });
  },
});
/* eslint-enable perfectionist/sort-objects */

function RouteComponent() {
  return (
    <QueryErrorResetBoundary>
      <Outlet />
    </QueryErrorResetBoundary>
  );
}
