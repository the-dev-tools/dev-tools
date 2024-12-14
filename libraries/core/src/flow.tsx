import { enumToJson } from '@bufbuild/protobuf';
import { createQueryOptions, useQuery as useConnectQuery } from '@connectrpc/connect-query';
import { createFileRoute, redirect } from '@tanstack/react-router';
import {
  addEdge,
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
  ReactFlowProps,
  ReactFlowProvider,
  useEdgesState,
  useNodesState,
  useReactFlow,
} from '@xyflow/react';
import { Array, Option, pipe, String } from 'effect';
import { Ulid } from 'id128';
import { ComponentProps, useCallback } from 'react';
import { Header, ListBoxSection } from 'react-aria-components';
import { IconType } from 'react-icons';
import { FiTerminal } from 'react-icons/fi';

import { EdgeListItem } from '@the-dev-tools/spec/flow/edge/v1/edge_pb';
import { edgeList } from '@the-dev-tools/spec/flow/edge/v1/edge-EdgeService_connectquery';
import { NodeKindJson, NodeKindSchema, NodeListItem, NodeStart } from '@the-dev-tools/spec/flow/node/v1/node_pb';
import { nodeList } from '@the-dev-tools/spec/flow/node/v1/node-NodeService_connectquery';
import { FlowGetResponse } from '@the-dev-tools/spec/flow/v1/flow_pb';
import { flowGet } from '@the-dev-tools/spec/flow/v1/flow-FlowService_connectquery';
import {
  CollectIcon,
  DataSourceIcon,
  DelayIcon,
  ForIcon,
  IfIcon,
  PlayIcon,
  SendRequestIcon,
} from '@the-dev-tools/ui/icons';
import { ListBox, ListBoxItem, ListBoxItemProps } from '@the-dev-tools/ui/list-box';
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

  return (
    <ReactFlowProvider>
      <FlowView flow={flowQuery.data} edges={edgeListQuery.data.items} nodes={nodeListQuery.data.items} />
    </ReactFlowProvider>
  );
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
  const { x, y } = data.position!;
  return Option.some({
    id: Ulid.construct(data.nodeId).toCanonical(),
    position: { x, y },
    type: kind as typeof kind | 'create',
    data: data as typeof data | Record<string, never>,
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

interface CreateNode extends Node<Record<string, never>, 'create'> {}

let createNodeCount = 0;

const CreateNodeHeader = (props: Omit<ComponentProps<'div'>, 'className'>) => (
  <Header {...props} className={tw`px-3 pt-2 text-xs font-semibold leading-5 tracking-tight text-slate-500`} />
);

interface CreateNodeItemProps extends Omit<ListBoxItemProps, 'children' | 'className' | 'textValue'> {
  Icon: IconType;
  title: string;
  description: string;
}

const CreateNodeItem = ({ Icon, title, description, ...props }: CreateNodeItemProps) => (
  <ListBoxItem {...props} className={tw`grid grid-cols-[auto_1fr] gap-x-2 gap-y-0 px-3 py-2`} textValue={title}>
    <div className={tw`row-span-2 rounded-md border border-slate-200 p-1.5`}>
      <Icon className={tw`size-5 text-slate-500`} />
    </div>
    <span className={tw`text-md font-semibold leading-5 tracking-tight`}>{title}</span>
    <span className={tw`text-xs leading-4 tracking-tight text-slate-500`}>{description}</span>
  </ListBoxItem>
);

// eslint-disable-next-line @typescript-eslint/no-unused-vars
const CreateNodeView = (_: NodeProps<CreateNode>) => (
  <>
    <ListBox aria-label='Create node type' onAction={() => void {}} className={tw`w-80 divide-y divide-slate-200 pt-0`}>
      <ListBoxSection>
        <CreateNodeHeader>Task</CreateNodeHeader>

        <CreateNodeItem
          id='request'
          Icon={SendRequestIcon}
          title='Send Request'
          description='Send request from your collection'
        />

        <CreateNodeItem id='data' Icon={DataSourceIcon} title='Data Source' description='Import data from .xlsx file' />

        <CreateNodeItem id='delay' Icon={DelayIcon} title='Delay' description='Wait specific time' />

        <CreateNodeItem id='javascript' Icon={FiTerminal} title='JavaScript' description='Custom Javascript block' />
      </ListBoxSection>

      <ListBoxSection>
        <CreateNodeHeader>Logic</CreateNodeHeader>

        <CreateNodeItem id='condition' Icon={IfIcon} title='If' description='Add true/false' />
      </ListBoxSection>

      <ListBoxSection>
        <CreateNodeHeader>Looping</CreateNodeHeader>

        <CreateNodeItem id='collect' Icon={CollectIcon} title='Collect' description='Collect all result' />

        <CreateNodeItem id='for' Icon={ForIcon} title='For Loop' description='Loop' />

        <CreateNodeItem id='foreach' Icon={ForIcon} title='For Each Loop' description='Loop' />
      </ListBoxSection>
    </ListBox>

    <Handle type='target' position={Position.Top} />
  </>
);

const nodeTypes: NodeTypes = {
  start: StartNodeView,
  create: CreateNodeView,
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
    <div className={tw`pointer-events-none size-2 rounded-full bg-slate-800`} />
  </BaseHandle>
);

interface FlowViewProps {
  flow: FlowGetResponse;
  edges: EdgeListItem[];
  nodes: NodeListItem[];
}

const FlowView = ({ edges: serverEdges, nodes: serverNodes }: FlowViewProps) => {
  const { screenToFlowPosition } = useReactFlow();

  const [edges, setEdges, onEdgesChange] = pipe(serverEdges, Array.map(mapEdgeToClient), useEdgesState);
  const [nodes, setNodes, onNodesChange] = pipe(serverNodes, Array.map(mapNodeToClient), Array.getSomes, useNodesState);

  const onConnect = useCallback<NonNullable<ReactFlowProps['onConnect']>>(
    (params) => void setEdges((_) => addEdge(params, _)),
    [setEdges],
  );

  const onConnectEnd = useCallback<NonNullable<ReactFlowProps['onConnectEnd']>>(
    (event, { isValid, fromNode }) => {
      if (!(event instanceof MouseEvent)) return;
      if (isValid) return;
      if (fromNode === null) return;

      const id = `create-${createNodeCount}`;
      createNodeCount++;
      const newNode = {
        id,
        position: screenToFlowPosition({ x: event.clientX, y: event.clientY }),
        data: {},
        origin: [0.5, 0.0],
        type: 'create' as const,
      } satisfies Node;

      setNodes((_) => _.concat(newNode));
      setEdges((_) => _.concat({ id, source: fromNode.id, target: id }));
    },
    [screenToFlowPosition, setEdges, setNodes],
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
      onConnect={onConnect}
      onConnectEnd={onConnectEnd}
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
