import { createClient } from '@connectrpc/connect';
import { createQueryOptions } from '@connectrpc/connect-query';
import { useSuspenseQueries } from '@tanstack/react-query';
import { createFileRoute, redirect } from '@tanstack/react-router';
import {
  Background,
  BackgroundVariant,
  NodeTypes as NodeTypesCore,
  ReactFlow,
  ReactFlowProps,
  ReactFlowProvider,
  Panel as RFPanel,
  useReactFlow,
  useViewport,
} from '@xyflow/react';
import { Array, HashMap, Match, pipe, Record, Schema } from 'effect';
import { Ulid } from 'id128';
import { Suspense, useCallback, useMemo } from 'react';
import { MenuTrigger } from 'react-aria-components';
import { FiMinus, FiMoreHorizontal, FiPlus } from 'react-icons/fi';
import { Panel } from 'react-resizable-panels';

import { useConnectMutation, useConnectSuspenseQuery } from '@the-dev-tools/api/connect-query';
import { NodeKind, NodeKindJson, NodeNoOpKind } from '@the-dev-tools/spec/flow/node/v1/node_pb';
import { nodeGet } from '@the-dev-tools/spec/flow/node/v1/node-NodeService_connectquery';
import { FlowGetResponse, FlowService, NodeState } from '@the-dev-tools/spec/flow/v1/flow_pb';
import { flowDelete, flowGet, flowUpdate } from '@the-dev-tools/spec/flow/v1/flow-FlowService_connectquery';
import { Button } from '@the-dev-tools/ui/button';
import { PlayCircleIcon, Spinner } from '@the-dev-tools/ui/icons';
import { Menu, MenuItem, useContextMenuState } from '@the-dev-tools/ui/menu';
import { PanelResizeHandle } from '@the-dev-tools/ui/resizable-panel';
import { Separator } from '@the-dev-tools/ui/separator';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextField, useEditableTextState } from '@the-dev-tools/ui/text-field';

import { EndpointRouteSearch } from '../endpoint';
import { ReferenceContext } from '../reference';
import { ConnectionLine, Edge, edgesQueryOptions, edgeTypes, useMakeEdge, useOnEdgesChange } from './edge';
import { workspaceRoute } from './internal';
import { Node, nodesQueryOptions, useMakeNode, useOnNodesChange } from './node';
import { ConditionNode, ConditionPanel } from './nodes/condition';
import { ForNode, ForPanel } from './nodes/for';
import { ForEachNode, ForEachPanel } from './nodes/for-each';
import { NoOpNode } from './nodes/no-op';
import { RequestNode, RequestPanel } from './nodes/request';

class Search extends EndpointRouteSearch.extend<Search>('FlowRouteSearch')({
  selectedNodeIdCan: pipe(Schema.String, Schema.optional),
}) {}

export const Route = createFileRoute('/_authorized/workspace/$workspaceIdCan/flow/$flowIdCan')({
  component: RouteComponent,
  validateSearch: (_) => Schema.decodeSync(Search)(_),
  pendingComponent: () => (
    <div className={tw`flex h-full items-center justify-center`}>
      <Spinner className={tw`size-16`} />
    </div>
  ),
  loader: async ({ params: { flowIdCan }, context: { transport, queryClient } }) => {
    const flowId = Ulid.fromCanonical(flowIdCan).bytes;

    try {
      await Promise.all([
        queryClient.ensureQueryData(createQueryOptions(flowGet, { flowId }, { transport })),
        queryClient.ensureQueryData(edgesQueryOptions({ transport, flowId })),
        queryClient.ensureQueryData(nodesQueryOptions({ transport, flowId })),
      ]);
    } catch {
      redirect({
        from: Route.fullPath,
        to: '../..',
        throw: true,
      });
    }

    return { flowId };
  },
});

export const nodeTypes: Record<NodeKindJson, NodeTypesCore[string]> = {
  NODE_KIND_UNSPECIFIED: () => null,
  NODE_KIND_NO_OP: NoOpNode,
  NODE_KIND_REQUEST: RequestNode,
  NODE_KIND_CONDITION: ConditionNode,
  NODE_KIND_FOR: ForNode,
  NODE_KIND_FOR_EACH: ForEachNode,
};

function RouteComponent() {
  const { flowId } = Route.useLoaderData();
  const { selectedNodeIdCan } = Route.useSearch();
  const { transport } = Route.useRouteContext();

  const [flowQuery, edgesQuery, nodesQuery] = useSuspenseQueries({
    queries: [
      createQueryOptions(flowGet, { flowId }, { transport }),
      edgesQueryOptions({ transport, flowId }),
      nodesQueryOptions({ transport, flowId }),
    ],
  });

  return (
    <ReactFlowProvider>
      <Panel id='request' order={1} className='flex h-full flex-col'>
        <FlowView flow={flowQuery.data} edges={edgesQuery.data} nodes={nodesQuery.data} />
      </Panel>
      <Suspense>{selectedNodeIdCan !== undefined && <EditPanel nodeIdCan={selectedNodeIdCan} />}</Suspense>
    </ReactFlowProvider>
  );
}

interface FlowViewProps {
  flow: FlowGetResponse;
  edges: Edge[];
  nodes: Node[];
}

const minZoom = 0.5;
const maxZoom = 2;

const FlowView = ({ flow, edges, nodes }: FlowViewProps) => {
  const { addNodes, addEdges, screenToFlowPosition } = useReactFlow();

  const onEdgesChange = useOnEdgesChange();
  const onNodesChange = useOnNodesChange();

  const makeNode = useMakeNode();
  const makeEdge = useMakeEdge();

  const onConnectEnd = useCallback<NonNullable<ReactFlowProps['onConnectEnd']>>(
    async (event, { isValid, fromNode }) => {
      if (!(event instanceof MouseEvent)) return;
      if (isValid) return;
      if (fromNode === null) return;

      const node = await makeNode({
        position: screenToFlowPosition({ x: event.clientX, y: event.clientY }),
        kind: NodeKind.NO_OP,
        noOp: NodeNoOpKind.CREATE,
      });

      const edge = await makeEdge({
        sourceId: Ulid.fromCanonical(fromNode.id).bytes,
        targetId: node.nodeId,
      });

      pipe(Node.fromDTO(node), addNodes);
      pipe(Edge.fromDTO(edge), addEdges);
    },
    [addEdges, addNodes, makeEdge, makeNode, screenToFlowPosition],
  );

  return (
    <ReactFlow
      proOptions={{ hideAttribution: true }}
      colorMode='light'
      minZoom={minZoom}
      maxZoom={maxZoom}
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
      onConnectEnd={onConnectEnd}
    >
      <Background
        variant={BackgroundVariant.Dots}
        size={2}
        gap={20}
        color='currentColor'
        className={tw`text-slate-300`}
      />

      <TopBar flow={flow} />
      <ActionBar />
    </ReactFlow>
  );
};

interface TopBarProps {
  flow: FlowGetResponse;
}

const TopBar = ({ flow: { flowId, name } }: TopBarProps) => {
  const { zoomIn, zoomOut } = useReactFlow();
  const { zoom } = useViewport();

  const flowUpdateMutation = useConnectMutation(flowUpdate);
  const flowDeleteMutation = useConnectMutation(flowDelete);

  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    value: name,
    onSuccess: (_) => flowUpdateMutation.mutateAsync({ flowId, name: _ }),
  });

  return (
    <RFPanel className={tw`m-0 flex w-full items-center gap-2 border-b border-slate-200 bg-white px-3 py-3.5`}>
      {isEditing ? (
        <TextField
          inputClassName={tw`-my-1 py-1 text-md font-medium leading-none tracking-tight text-slate-800`}
          isDisabled={flowUpdateMutation.isPending}
          {...textFieldProps}
        />
      ) : (
        <div className={tw`text-md font-medium leading-5 tracking-tight text-slate-800`} onContextMenu={onContextMenu}>
          {name}
        </div>
      )}

      <div className={tw`flex-1`} />

      <Button
        variant='ghost'
        className={tw`p-0.5`}
        onPress={() => void zoomOut({ duration: 100 })}
        isDisabled={zoom <= minZoom}
      >
        <FiMinus className={tw`size-4 text-slate-500`} />
      </Button>

      <div className={tw`w-10 text-center text-sm font-medium leading-5 tracking-tight text-gray-900`}>
        {Math.floor(zoom * 100)}%
      </div>

      <Button
        variant='ghost'
        className={tw`p-0.5`}
        onPress={() => void zoomIn({ duration: 100 })}
        isDisabled={zoom >= maxZoom}
      >
        <FiPlus className={tw`size-4 text-slate-500`} />
      </Button>

      <MenuTrigger {...menuTriggerProps}>
        <Button variant='ghost' className={tw`bg-slate-200 p-0.5`}>
          <FiMoreHorizontal className={tw`size-4 text-slate-500`} />
        </Button>

        <Menu {...menuProps}>
          <MenuItem onAction={() => void edit()}>Rename</MenuItem>

          <Separator />

          <MenuItem variant='danger' onAction={() => void flowDeleteMutation.mutate({ flowId })}>
            Delete
          </MenuItem>
        </Menu>
      </MenuTrigger>
    </RFPanel>
  );
};

const ActionBar = () => {
  const { flowId } = Route.useLoaderData();
  const { transport } = Route.useRouteContext();
  const { flowRun } = useMemo(() => createClient(FlowService, transport), [transport]);
  const flow = useReactFlow<Node, Edge>();

  return (
    <RFPanel className={tw`mb-4 flex items-center gap-2 rounded-lg bg-slate-900 p-1 shadow`} position='bottom-center'>
      {/* <Button variant='ghost dark' className={tw`p-1`}>
        <TextBoxIcon className={tw`size-5 text-slate-300`} />
      </Button> */}

      {/* <Button variant='ghost dark' className={tw`p-1`}>
        <ChatAddIcon className={tw`size-5 text-slate-300`} />
      </Button> */}

      {/* <div className={tw`mx-2 h-5 w-px bg-white/20`} /> */}

      {/* TODO: implement add node action */}
      <Button variant='ghost dark' className={tw`px-1.5 py-1`}>
        <FiPlus className={tw`size-5 text-slate-300`} />
        Add Node
      </Button>

      <Button
        variant='primary'
        onPress={async () => {
          flow.getNodes().forEach((_) => void flow.updateNodeData(_.id, { ..._, state: NodeState.UNSPECIFIED }));
          flow.getEdges().forEach((_) => void flow.updateEdgeData(_.id, { ..._, state: NodeState.UNSPECIFIED }));

          const sourceEdges = pipe(
            flow.getEdges(),
            Array.groupBy((_) => _.source),
            Record.toEntries,
            HashMap.fromIterable,
          );

          for await (const { nodeId, state } of flowRun({ flowId })) {
            const nodeIdCan = Ulid.construct(nodeId).toCanonical();

            flow.updateNodeData(nodeIdCan, (_) => ({ ..._, state }));

            pipe(
              HashMap.get(sourceEdges, nodeIdCan),
              Array.fromOption,
              Array.flatten,
              Array.forEach((_) => void flow.updateEdgeData(_.id, (_) => ({ ..._, state }))),
            );
          }
        }}
      >
        <PlayCircleIcon className={tw`size-4`} />
        Run
      </Button>
    </RFPanel>
  );
};

interface EditPanelProps {
  nodeIdCan: string;
}

const EditPanel = ({ nodeIdCan }: EditPanelProps) => {
  const { workspaceId } = workspaceRoute.useLoaderData();

  const nodeId = Ulid.fromCanonical(nodeIdCan).bytes;

  const { data: node } = useConnectSuspenseQuery(nodeGet, { nodeId });

  const view = pipe(
    Match.value(node.kind),
    Match.when(NodeKind.REQUEST, () => <RequestPanel node={node} />),
    Match.when(NodeKind.CONDITION, () => <ConditionPanel node={node} />),
    Match.when(NodeKind.FOR, () => <ForPanel node={node} />),
    Match.when(NodeKind.FOR_EACH, () => <ForEachPanel node={node} />),
    Match.orElseAbsurd,
  );

  return (
    <ReferenceContext value={{ nodeId, workspaceId }}>
      <PanelResizeHandle direction='vertical' />
      <Panel id='response' order={2} defaultSize={40} className={tw`!overflow-auto`}>
        {view}
      </Panel>
    </ReferenceContext>
  );
};
