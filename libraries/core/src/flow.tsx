import { enumToJson } from '@bufbuild/protobuf';
import { createQueryOptions, useQuery as useConnectQuery } from '@connectrpc/connect-query';
import { createFileRoute, redirect } from '@tanstack/react-router';
import {
  Background,
  BackgroundVariant,
  Handle as BaseHandle,
  ConnectionLineComponentProps,
  Edge,
  EdgeProps,
  getSmoothStepPath,
  HandleProps,
  Node,
  NodeProps,
  NodeTypes,
  Position,
  ReactFlow,
  useEdgesState,
  useNodesState,
} from '@xyflow/react';
import { Array, Option, pipe, String } from 'effect';
import { Ulid } from 'id128';

import { EdgeListItem } from '@the-dev-tools/spec/flow/edge/v1/edge_pb';
import { edgeList } from '@the-dev-tools/spec/flow/edge/v1/edge-EdgeService_connectquery';
import { NodeKindJson, NodeKindSchema, NodeListItem, NodeStart } from '@the-dev-tools/spec/flow/node/v1/node_pb';
import { nodeList } from '@the-dev-tools/spec/flow/node/v1/node-NodeService_connectquery';
import { FlowGetResponse } from '@the-dev-tools/spec/flow/v1/flow_pb';
import { flowGet } from '@the-dev-tools/spec/flow/v1/flow-FlowService_connectquery';
import { PlayIcon } from '@the-dev-tools/ui/icons';
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

const mapNodeToClient = (node: NodeListItem) => {
  const kind = pipe(
    enumToJson(NodeKindSchema, node.kind),
    String.substring('NODE_KIND_'.length),
    (_) => _ as NodeKindJson extends `NODE_KIND_${infer Kind}` ? Kind : never,
    String.toLowerCase,
  );

  if (kind === 'unspecified') return Option.none();

  const data = node[kind]!;
  return Option.some({
    id: Ulid.construct(data.nodeId).toCanonical(),
    position: data.position!,
    type: kind,
    data,
  } satisfies Partial<Node>);
};

interface StartNode extends Node<NodeStart, 'start'> {}

// eslint-disable-next-line @typescript-eslint/no-unused-vars
const StartNodeView = (_: NodeProps<StartNode>) => (
  <>
    <div className={tw`flex w-40 items-center gap-2 rounded-md bg-slate-800 px-2 text-white shadow-sm`}>
      <PlayIcon className={tw`size-4`} />
      <div className={tw`w-px self-stretch bg-slate-700`} />
      <span className={tw`flex-1 py-1 text-xs font-medium leading-5`}>Manual start</span>
    </div>
    <Handle type='source' position={Position.Bottom} />
  </>
);

const nodeTypes: NodeTypes = {
  start: StartNodeView,
};

const EdgeView = ({ sourceX, sourceY, sourcePosition, targetX, targetY, targetPosition }: EdgeProps) => (
  <ConnectionLine
    fromX={sourceX}
    fromY={sourceY}
    fromPosition={sourcePosition}
    toX={targetX}
    toY={targetY}
    toPosition={targetPosition}
  />
);

const edgeTypes = {
  default: EdgeView,
};

const ConnectionLine = ({
  fromX,
  fromY,
  fromPosition,
  toX,
  toY,
  toPosition,
}: Pick<ConnectionLineComponentProps, 'fromX' | 'fromY' | 'fromPosition' | 'toX' | 'toY' | 'toPosition'>) => {
  const [edgePath] = getSmoothStepPath({
    sourceX: fromX,
    sourceY: fromY,
    sourcePosition: fromPosition,
    targetX: toX,
    targetY: toY,
    targetPosition: toPosition,
    borderRadius: 8,
    offset: 8,
  });

  return <path className={tw`fill-none stroke-slate-800 stroke-1`} d={edgePath} />;
};

const Handle = (props: HandleProps) => (
  <BaseHandle
    className={tw`-z-10 flex size-5 items-center justify-center rounded-full border border-slate-300 bg-slate-200 shadow-sm`}
    {...props}
  >
    <div className={tw`size-2 rounded-full bg-slate-800`} />
  </BaseHandle>
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
      onInit={(reactFlow) => {
        void reactFlow.fitView();
      }}
      connectionLineComponent={ConnectionLine}
      nodeTypes={nodeTypes}
      edgeTypes={edgeTypes}
      defaultEdgeOptions={{ type: 'default' }}
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
