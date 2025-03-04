import { enumFromJson, isEnumJson } from '@bufbuild/protobuf';
import { createClient } from '@connectrpc/connect';
import { createQueryOptions } from '@connectrpc/connect-query';
import { useSuspenseQueries } from '@tanstack/react-query';
import { createFileRoute, redirect, useMatchRoute, useRouteContext } from '@tanstack/react-router';
import {
  Background,
  BackgroundVariant,
  NodeTypes as NodeTypesCore,
  OnBeforeDelete,
  ReactFlow,
  ReactFlowProps,
  ReactFlowProvider,
  Panel as RFPanel,
  SelectionMode,
  useReactFlow,
  useStoreApi,
  useViewport,
} from '@xyflow/react';
import { Array, Boolean, HashMap, Match, MutableHashMap, Option, pipe, Record } from 'effect';
import { Ulid } from 'id128';
import { ReactNode, Suspense, use, useCallback, useMemo } from 'react';
import { MenuTrigger } from 'react-aria-components';
import { FiClock, FiMinus, FiMoreHorizontal, FiPlus } from 'react-icons/fi';
import { Panel, PanelGroup } from 'react-resizable-panels';

import { useConnectMutation, useConnectQuery, useConnectSuspenseQuery } from '@the-dev-tools/api/connect-query';
import { NodeKind, NodeKindJson, NodeNoOpKind, NodeState } from '@the-dev-tools/spec/flow/node/v1/node_pb';
import { nodeGet } from '@the-dev-tools/spec/flow/node/v1/node-NodeService_connectquery';
import { FlowService } from '@the-dev-tools/spec/flow/v1/flow_pb';
import { flowDelete, flowGet, flowUpdate } from '@the-dev-tools/spec/flow/v1/flow-FlowService_connectquery';
import { Button, ButtonAsLink } from '@the-dev-tools/ui/button';
import { PlayCircleIcon, Spinner } from '@the-dev-tools/ui/icons';
import { Menu, MenuItem, useContextMenuState } from '@the-dev-tools/ui/menu';
import { PanelResizeHandle } from '@the-dev-tools/ui/resizable-panel';
import { Separator } from '@the-dev-tools/ui/separator';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextField, useEditableTextState } from '@the-dev-tools/ui/text-field';

import { ReferenceContext } from '../reference';
import { StatusBar } from '../status-bar';
import { ConnectionLine, Edge, edgesQueryOptions, edgeTypes, useMakeEdge, useOnEdgesChange } from './edge';
import { FlowContext, flowRoute, HandleKind, HandleKindSchema, useSelectedNodeId, workspaceRoute } from './internal';
import { Node, nodesQueryOptions, useMakeNode, useOnNodesChange } from './node';
import { ConditionNode, ConditionPanel } from './nodes/condition';
import { ForNode, ForPanel } from './nodes/for';
import { ForEachNode, ForEachPanel } from './nodes/for-each';
import { NoOpNode } from './nodes/no-op';
import { RequestNode, RequestPanel } from './nodes/request';

export const Route = createFileRoute('/_authorized/workspace/$workspaceIdCan/flow/$flowIdCan/')({
  component: RouteComponent,
  pendingComponent: () => (
    <div className={tw`flex h-full items-center justify-center`}>
      <Spinner className={tw`size-16`} />
    </div>
  ),
  loader: async ({ context: { transport, queryClient }, parentMatchPromise }) => {
    const { loaderData } = await parentMatchPromise;
    if (!loaderData) return;
    const { flowId } = loaderData;

    try {
      await Promise.all([
        queryClient.ensureQueryData(createQueryOptions(flowGet, { flowId }, { transport })),
        queryClient.ensureQueryData(edgesQueryOptions({ transport, flowId })),
        queryClient.ensureQueryData(nodesQueryOptions({ transport, flowId })),
      ]);
    } catch {
      redirect({
        from: Route.fullPath,
        to: '/workspace/$workspaceIdCan',
        throw: true,
      });
    }
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
  const { flowId } = flowRoute.useLoaderData();

  return (
    <Panel id='main' order={2}>
      <Suspense
        fallback={
          <div className={tw`flex h-full items-center justify-center`}>
            <Spinner className={tw`size-16`} />
          </div>
        }
      >
        <PanelGroup direction='vertical'>
          <FlowContext.Provider value={{ flowId }}>
            <ReactFlowProvider>
              <TopBar />
              <Panel id='flow' order={1} className='flex h-full flex-col'>
                <Flow key={Ulid.construct(flowId).toCanonical()} flowId={flowId}>
                  <ActionBar />
                </Flow>
              </Panel>
              <EditPanel />
            </ReactFlowProvider>
          </FlowContext.Provider>
          <StatusBar />
        </PanelGroup>
      </Suspense>
    </Panel>
  );
}

interface FlowProps {
  flowId: Uint8Array;
  children?: ReactNode;
}

export const Flow = ({ flowId, children }: FlowProps) => {
  const { transport } = useRouteContext({ from: '__root__' });

  const [edgesQuery, nodesQuery] = useSuspenseQueries({
    queries: [edgesQueryOptions({ transport, flowId }), nodesQueryOptions({ transport, flowId })],
  });

  return (
    <FlowView edges={edgesQuery.data} nodes={nodesQuery.data}>
      {children}
    </FlowView>
  );
};

interface FlowViewProps {
  edges: Edge[];
  nodes: Node[];
  children?: ReactNode;
  isReadOnly?: boolean;
}

const minZoom = 0.5;
const maxZoom = 2;

const FlowView = ({ edges, nodes, children }: FlowViewProps) => {
  const { addNodes, addEdges, getEdges, getNode, screenToFlowPosition } = useReactFlow<Node, Edge>();
  const { isReadOnly = false } = use(FlowContext);

  const onEdgesChange = useOnEdgesChange();
  const onNodesChange = useOnNodesChange();

  const makeNode = useMakeNode();
  const makeEdge = useMakeEdge();

  const onConnect = useCallback<NonNullable<ReactFlowProps['onConnect']>>(
    async (connection) => {
      const edge = await pipe(connection, Edge.toDTO, makeEdge);
      pipe(edge, Edge.fromDTO, addEdges);
    },
    [addEdges, makeEdge],
  );

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

  const onBeforeDelete = useCallback<OnBeforeDelete<Node, Edge>>(
    ({ nodes, edges }) => {
      if (isReadOnly) return Promise.resolve(false);

      const deleteNodeMap = pipe(
        nodes.map((_) => [_.id, _] as const),
        MutableHashMap.fromIterable,
      );

      const deleteEdgeMap = pipe(
        edges.map((_) => [_.id, _] as const),
        MutableHashMap.fromIterable,
      );

      const edgesWithHandles = pipe(
        getEdges(),
        Array.map(
          Option.liftPredicate(
            (_) =>
              isEnumJson(HandleKindSchema, _.sourceHandle) &&
              enumFromJson(HandleKindSchema, _.sourceHandle) !== HandleKind.UNSPECIFIED,
          ),
        ),
        Array.getSomes,
      );

      for (const edge of edgesWithHandles) {
        if (
          !Boolean.some([
            MutableHashMap.has(deleteEdgeMap, edge.id),
            MutableHashMap.has(deleteNodeMap, edge.source),
            MutableHashMap.has(deleteNodeMap, edge.target),
          ])
        ) {
          continue;
        }

        MutableHashMap.set(deleteEdgeMap, edge.id, edge);

        const source = getNode(edge.source);
        if (source) MutableHashMap.set(deleteNodeMap, source.id, source);

        const target = getNode(edge.target);
        if (target) MutableHashMap.set(deleteNodeMap, target.id, target);
      }

      return Promise.resolve({
        nodes: pipe(Record.fromEntries(deleteNodeMap), Record.values),
        edges: pipe(Record.fromEntries(deleteEdgeMap), Record.values),
      });
    },
    [getEdges, getNode, isReadOnly],
  );

  return (
    <ReactFlow
      proOptions={{ hideAttribution: true }}
      colorMode='light'
      minZoom={minZoom}
      maxZoom={maxZoom}
      fitView
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
      onBeforeDelete={onBeforeDelete}
      nodesConnectable={!isReadOnly}
      nodesDraggable={!isReadOnly}
      selectNodesOnDrag={false}
      panOnScroll
      selectionOnDrag
      panOnDrag={[1, 2]}
      selectionMode={SelectionMode.Partial}
    >
      <Background
        variant={BackgroundVariant.Dots}
        size={2}
        gap={20}
        color='currentColor'
        className={tw`text-slate-300`}
      />
      {children}
    </ReactFlow>
  );
};

export const TopBar = () => {
  const { flowId } = flowRoute.useLoaderData();

  const {
    data: { name },
  } = useConnectSuspenseQuery(flowGet, { flowId });

  const { zoomIn, zoomOut } = useReactFlow();
  const { zoom } = useViewport();

  const matchRoute = useMatchRoute();

  const flowUpdateMutation = useConnectMutation(flowUpdate);
  const flowDeleteMutation = useConnectMutation(flowDelete);

  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    value: name,
    onSuccess: (_) => flowUpdateMutation.mutateAsync({ flowId, name: _ }),
  });

  return (
    <div className={tw`flex items-center gap-2 border-b border-slate-200 bg-white px-3 py-2.5`}>
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

      <div className={tw`h-4 w-px bg-slate-200`} />

      <ButtonAsLink
        variant='ghost'
        className={tw`px-2 py-1 text-slate-800`}
        href={{
          from: '/workspace/$workspaceIdCan/flow/$flowIdCan',
          to: matchRoute({ to: '/workspace/$workspaceIdCan/flow/$flowIdCan/history' })
            ? '/workspace/$workspaceIdCan/flow/$flowIdCan'
            : '/workspace/$workspaceIdCan/flow/$flowIdCan/history',
        }}
      >
        <FiClock className={tw`size-4 text-slate-500`} /> Flows History
      </ButtonAsLink>

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
    </div>
  );
};

const ActionBar = () => {
  const { flowId } = use(FlowContext);
  const { transport } = useRouteContext({ from: '__root__' });
  const { flowRun } = useMemo(() => createClient(FlowService, transport), [transport]);
  const flow = useReactFlow<Node, Edge>();
  const storeApi = useStoreApi<Node, Edge>();

  const makeNode = useMakeNode();

  return (
    <RFPanel className={tw`mb-4 flex items-center gap-2 rounded-lg bg-slate-900 p-1 shadow`} position='bottom-center'>
      {/* <Button variant='ghost dark' className={tw`p-1`}>
        <TextBoxIcon className={tw`size-5 text-slate-300`} />
      </Button> */}

      {/* <Button variant='ghost dark' className={tw`p-1`}>
        <ChatAddIcon className={tw`size-5 text-slate-300`} />
      </Button> */}

      {/* <div className={tw`mx-2 h-5 w-px bg-white/20`} /> */}

      <Button
        variant='ghost dark'
        className={tw`px-1.5 py-1`}
        onPress={async () => {
          const { domNode } = storeApi.getState();
          if (!domNode) return;
          const box = domNode.getBoundingClientRect();
          const position = flow.screenToFlowPosition({ x: box.x + box.width / 2, y: box.y + box.height * 0.1 });
          const node = await makeNode({ kind: NodeKind.NO_OP, noOp: NodeNoOpKind.CREATE, position });
          pipe(node, Node.fromDTO, flow.addNodes);
        }}
      >
        <FiPlus className={tw`size-5 text-slate-300`} />
        Add Node
      </Button>

      <Button
        variant='primary'
        onPress={async () => {
          flow.getNodes().forEach(
            (_) =>
              void flow.updateNodeData(_.id, {
                ..._,
                state: NodeState.UNSPECIFIED,
              }),
          );
          flow.getEdges().forEach(
            (_) =>
              void flow.updateEdgeData(_.id, {
                ..._,
                state: NodeState.UNSPECIFIED,
              }),
          );

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

export const EditPanel = () => {
  const { workspaceId } = workspaceRoute.useLoaderData();

  const selectedNodeId = useSelectedNodeId();

  const nodeQuery = useConnectQuery(nodeGet, { nodeId: selectedNodeId! }, { enabled: selectedNodeId !== undefined });

  if (!selectedNodeId || !nodeQuery.data) return null;

  const view = pipe(
    Match.value(nodeQuery.data.kind),
    Match.when(NodeKind.REQUEST, () => <RequestPanel node={nodeQuery.data} />),
    Match.when(NodeKind.CONDITION, () => <ConditionPanel node={nodeQuery.data} />),
    Match.when(NodeKind.FOR, () => <ForPanel node={nodeQuery.data} />),
    Match.when(NodeKind.FOR_EACH, () => <ForEachPanel node={nodeQuery.data} />),
    Match.orElse(() => null),
  );

  if (!view) return null;

  return (
    <ReferenceContext value={{ nodeId: selectedNodeId, workspaceId }}>
      <PanelResizeHandle direction='vertical' />
      <Panel id='node' order={2} defaultSize={40} className={tw`!overflow-auto`}>
        <Suspense
          fallback={
            <div className={tw`flex h-full items-center justify-center`}>
              <Spinner className={tw`size-12`} />
            </div>
          }
        >
          {view}
        </Suspense>
      </Panel>
    </ReferenceContext>
  );
};
