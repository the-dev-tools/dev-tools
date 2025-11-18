import { QueryErrorResetBoundary } from '@tanstack/react-query';
import { createFileRoute, ErrorComponent, Outlet } from '@tanstack/react-router';
import { Option, pipe, Schema, Struct } from 'effect';
import { Ulid } from 'id128';
import { addTab } from '@the-dev-tools/ui/router';
import { FlowTab, flowTabId } from '~/features/flow';

export class FlowSearch extends Schema.Class<FlowSearch>('FlowSearch')({
  node: pipe(Schema.String, Schema.optional),
}) {}

/* eslint-disable perfectionist/sort-objects */
export const Route = createFileRoute('/workspace/$workspaceIdCan/flow/$flowIdCan')({
  validateSearch: (_) => Schema.decodeSync(FlowSearch)(_),
  loaderDeps: (_) => Struct.pick(_.search, 'node'),
  loader: ({ deps: { node }, params: { flowIdCan } }) => {
    const flowId = Ulid.fromCanonical(flowIdCan).bytes;
    const nodeId = pipe(
      Option.fromNullable(node),
      Option.map((_) => Ulid.fromCanonical(_).bytes),
    );
    return { flowId, nodeId };
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
