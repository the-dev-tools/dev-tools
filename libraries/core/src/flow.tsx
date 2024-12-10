import { createQueryOptions, useQuery as useConnectQuery } from '@connectrpc/connect-query';
import { createFileRoute, redirect } from '@tanstack/react-router';
import { Background, BackgroundVariant, Edge, Node, ReactFlow, useEdgesState, useNodesState } from '@xyflow/react';
import { Array, Match, Option, pipe } from 'effect';
import { Ulid } from 'id128';

import { EdgeListItem } from '@the-dev-tools/spec/flow/edge/v1/edge_pb';
import { edgeList } from '@the-dev-tools/spec/flow/edge/v1/edge-EdgeService_connectquery';
import { NodeBase, NodeKind, NodeListItem } from '@the-dev-tools/spec/flow/node/v1/node_pb';
import { nodeList } from '@the-dev-tools/spec/flow/node/v1/node-NodeService_connectquery';
import { FlowGetResponse } from '@the-dev-tools/spec/flow/v1/flow_pb';
import { flowGet } from '@the-dev-tools/spec/flow/v1/flow-FlowService_connectquery';
import { tw } from '@the-dev-tools/ui/tailwind-literal';

export const Route = createFileRoute('/_authorized/workspace/$workspaceIdCan/flow/$flowIdCan')({
  component: RouteComponent,
  pendingComponent: () => 'Loading flow...',
  loader: async ({ params: { flowIdCan }, context: { transport, queryClient }, route }) => {
    const flowId = Ulid.fromCanonical(flowIdCan).bytes;

    try {
      await Promise.all([
        queryClient.ensureQueryData(createQueryOptions(flowGet, { flowId }, { transport })),
        queryClient.ensureQueryData(createQueryOptions(nodeList, { flowId }, { transport })),
        queryClient.ensureQueryData(createQueryOptions(edgeList, { flowId }, { transport })),
      ]);
    } catch {
      redirect({
        from: route.fullPath,
        to: '../..',
        throw: true,
      });
    }

    return { flowId };
  },
});

function RouteComponent() {
  const { flowId } = Route.useLoaderData();

  const flowQuery = useConnectQuery(flowGet, { flowId });
  const edgeListQuery = useConnectQuery(edgeList, { flowId });
  const nodeListQuery = useConnectQuery(nodeList, { flowId });

  if (!flowQuery.data || !edgeListQuery.data || !nodeListQuery.data) return null;

  return <FlowView flow={flowQuery.data} edges={edgeListQuery.data.items} nodes={nodeListQuery.data.items} />;
}

const mapEdgeToClient = (edge: EdgeListItem) =>
  ({
    id: Ulid.construct(edge.edgeId).toCanonical(),
    source: Ulid.construct(edge.sourceId).toCanonical(),
    target: Ulid.construct(edge.targetId).toCanonical(),
  }) satisfies Edge;

const mapNodeBaseToClient = (node: Omit<NodeBase, '$typeName'>) =>
  ({
    id: Ulid.construct(node.nodeId).toCanonical(),
    position: node.position!,
  }) satisfies Partial<Node>;

const mapNodeToClient = (node: NodeListItem) =>
  pipe(
    Match.value(node),
    Match.when({ kind: NodeKind.CONDITION }, (_) => {
      const data = _.condition!;
      return Option.some({ ...mapNodeBaseToClient(data), data } satisfies Node);
    }),
    Match.when({ kind: NodeKind.FOR }, (_) => {
      const data = _.for!;
      return Option.some({ ...mapNodeBaseToClient(data), data } satisfies Node);
    }),
    Match.when({ kind: NodeKind.REQUEST }, (_) => {
      const data = _.request!;
      return Option.some({ ...mapNodeBaseToClient(data), data } satisfies Node);
    }),
    Match.orElse(() => Option.none()),
  );

interface FlowViewProps {
  flow: FlowGetResponse;
  edges: EdgeListItem[];
  nodes: NodeListItem[];
}

const FlowView = ({ edges: serverEdges, nodes: serverNodes }: FlowViewProps) => {
  const [edges, _setEdges, onEdgesChange] = pipe(serverEdges, Array.map(mapEdgeToClient), useEdgesState);
  const [nodes, _setNodes, onNodesChange] = pipe(
    serverNodes,
    Array.map(mapNodeToClient),
    Array.getSomes,
    useNodesState,
  );

  return (
    <ReactFlow
      proOptions={{ hideAttribution: true }}
      colorMode='light'
      nodes={nodes}
      edges={edges}
      onNodesChange={onNodesChange}
      onEdgesChange={onEdgesChange}
    >
      <Background
        variant={BackgroundVariant.Dots}
        size={2}
        gap={20}
        color='currentColor'
        className={tw`text-slate-300`}
      />
    </ReactFlow>
  );
};
