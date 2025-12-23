import { QueryErrorResetBoundary } from '@tanstack/react-query';
import { createFileRoute, ErrorComponent, Outlet } from '@tanstack/react-router';
import { Ulid } from 'id128';
import { addTab } from '@the-dev-tools/ui/router';
import { FlowTab, flowTabId } from '~/features/flow';

/* eslint-disable perfectionist/sort-objects */
export const Route = createFileRoute('/workspace/$workspaceIdCan/flow/$flowIdCan')({
  loader: ({ params: { flowIdCan } }) => {
    const flowId = Ulid.fromCanonical(flowIdCan).bytes;
    return { flowId };
  },
  component: RouteComponent,
  errorComponent: (props) => <ErrorComponent {...props} />,
  onEnter: (match) => {
    if (!match.loaderData) return;

    const { flowId } = match.loaderData;

    addTab({
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
