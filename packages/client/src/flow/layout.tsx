import { QueryErrorResetBoundary } from '@tanstack/react-query';
import { createFileRoute, Outlet } from '@tanstack/react-router';
import { Option, pipe, Schema, Struct } from 'effect';
import { Ulid } from 'id128';
import { FlowGetEndpoint } from '@the-dev-tools/spec/meta/flow/v1/flow.endpoints.js';
import { FlowsIcon } from '@the-dev-tools/ui/icons';
import { addTab } from '@the-dev-tools/ui/router';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useQuery } from '~data-client';
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
  errorComponent: (props) => <ErrorComponent {...props} />,
  onEnter: (match) => {
    if (!match.loaderData) return;

    const { flowId } = match.loaderData;

    addTab({
      id: JSON.stringify({ flowId, route: Route.id }),
      match,
      node: <FlowTab flowId={flowId} />,
    });
  },
});

interface FlowTabProps {
  flowId: Uint8Array;
}

const FlowTab = ({ flowId }: FlowTabProps) => {
  const { name } = useQuery(FlowGetEndpoint, { flowId });
  return (
    <>
      <FlowsIcon className={tw`size-5 shrink-0 text-slate-500`} />
      <span className={tw`min-w-0 flex-1 truncate`}>{name}</span>
    </>
  );
};
