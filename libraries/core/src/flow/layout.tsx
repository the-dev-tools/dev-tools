import { QueryErrorResetBoundary } from '@tanstack/react-query';
import { createFileRoute, Outlet } from '@tanstack/react-router';
import { Option, pipe, Schema, Struct } from 'effect';
import { Ulid } from 'id128';
import { Panel } from 'react-resizable-panels';

import { ErrorComponent } from '../error';

export class FlowSearch extends Schema.Class<FlowSearch>('FlowSearch')({
  node: pipe(Schema.String, Schema.optional),
}) {}

const makeRoute = createFileRoute('/_authorized/workspace/$workspaceIdCan/flow/$flowIdCan');

export const Route = makeRoute({
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
  component: () => (
    <QueryErrorResetBoundary>
      <Outlet />
    </QueryErrorResetBoundary>
  ),
  errorComponent: (props) => (
    <Panel id='main' order={2}>
      <ErrorComponent {...props} />
    </Panel>
  ),
});
