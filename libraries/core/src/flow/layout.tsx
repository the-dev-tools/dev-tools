import { createFileRoute } from '@tanstack/react-router';
import { Option, pipe, Schema, Struct } from 'effect';
import { Ulid } from 'id128';

export class FlowSearch extends Schema.Class<FlowSearch>('FlowSearch')({
  node: pipe(Schema.String, Schema.optional),
}) {}

export const Route = createFileRoute('/_authorized/workspace/$workspaceIdCan/flow/$flowIdCan')({
  validateSearch: (_) => Schema.decodeSync(FlowSearch)(_),
  loaderDeps: (_) => Struct.pick(_.search, 'node'),
  loader: ({ params: { flowIdCan }, deps: { node } }) => {
    const flowId = Ulid.fromCanonical(flowIdCan).bytes;
    const nodeId = pipe(
      Option.fromNullable(node),
      Option.map((_) => Ulid.fromCanonical(_).bytes),
    );
    return { flowId, nodeId };
  },
});
