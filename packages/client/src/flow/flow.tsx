import { enumFromJson, isEnumJson } from '@bufbuild/protobuf';
import { createClient } from '@connectrpc/connect';
import { createQueryOptions } from '@connectrpc/connect-query';
import { useSuspenseQueries } from '@tanstack/react-query';
import { createFileRoute, redirect, useMatchRoute, useNavigate, useRouteContext } from '@tanstack/react-router';
import { ColumnDef, createColumnHelper } from '@tanstack/react-table';
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
import { Array, Boolean, HashMap, Match, MutableHashMap, Option, pipe, Record, Struct } from 'effect';
import { Ulid } from 'id128';
import { ReactNode, Suspense, use, useCallback, useMemo } from 'react';
import { MenuTrigger } from 'react-aria-components';
import { FiClock, FiMinus, FiMoreHorizontal, FiPlus, FiX } from 'react-icons/fi';
import { Panel, PanelGroup } from 'react-resizable-panels';

import { NodeKind, NodeKindJson, NodeNoOpKind, NodeState } from '@the-dev-tools/spec/flow/node/v1/node_pb';
import { nodeGet } from '@the-dev-tools/spec/flow/node/v1/node-NodeService_connectquery';
import { FlowService } from '@the-dev-tools/spec/flow/v1/flow_pb';
import { flowDelete, flowGet, flowUpdate } from '@the-dev-tools/spec/flow/v1/flow-FlowService_connectquery';
import { FlowVariableListItem, FlowVariableListItemSchema } from '@the-dev-tools/spec/flowvariable/v1/flowvariable_pb';
import {
  flowVariableCreate,
  flowVariableDelete,
  flowVariableList,
  flowVariableUpdate,
} from '@the-dev-tools/spec/flowvariable/v1/flowvariable-FlowVariableService_connectquery';
import { Button, ButtonAsLink } from '@the-dev-tools/ui/button';
import { DataTable } from '@the-dev-tools/ui/data-table';
import { PlayCircleIcon, Spinner } from '@the-dev-tools/ui/icons';
import { Menu, MenuItem, useContextMenuState } from '@the-dev-tools/ui/menu';
import { PanelResizeHandle } from '@the-dev-tools/ui/resizable-panel';
import { Separator } from '@the-dev-tools/ui/separator';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextField, TextFieldRHF, useEditableTextState } from '@the-dev-tools/ui/text-field';
import { useConnectMutation, useConnectQuery, useConnectSuspenseQuery } from '~/api/connect-query';
import { useQueryNormalizer } from '~/api/normalizer';
import { FormTableItem, genericFormTableActionColumn, genericFormTableEnableColumn, useFormTable } from '~form-table';

import { ReferenceContext } from '../reference';
import { StatusBar } from '../status-bar';
import { ConnectionLine, Edge, edgesQueryOptions, edgeTypes, useMakeEdge, useOnEdgesChange } from './edge';
import { FlowContext, flowRoute, HandleKind, HandleKindSchema, workspaceRoute } from './internal';
import { FlowSearch } from './layout';
import { Node, nodesQueryOptions, useMakeNode, useOnNodesChange } from './node';
import { ConditionNode, ConditionPanel } from './nodes/condition';
import { ForNode, ForPanel } from './nodes/for';
import { ForEachNode, ForEachPanel } from './nodes/for-each';
import { JavaScriptNode, JavaScriptPanel } from './nodes/javascript';
import { NoOpNode } from './nodes/no-op';
import { RequestNode, RequestPanel } from './nodes/request';

const makeRoute = createFileRoute('/_authorized/workspace/$workspaceIdCan/flow/$flowIdCan/');

export const Route = makeRoute({
  loader: async ({ context: { queryClient, transport }, parentMatchPromise }) => {
    const { loaderData } = await parentMatchPromise;
    if (!loaderData) return;
    const { flowId } = loaderData;

    try {
      await Promise.all([
        queryClient.ensureQueryData(createQueryOptions(flowGet, { flowId }, { transport })),
        queryClient.ensureQueryData(edgesQueryOptions({ flowId, transport })),
        queryClient.ensureQueryData(nodesQueryOptions({ flowId, transport })),
      ]);
    } catch {
      redirect({
        from: Route.fullPath,
        throw: true,
        to: '/workspace/$workspaceIdCan',
      });
    }
  },
  component: RouteComponent,
  pendingComponent: () => (
    <div className={tw`flex h-full items-center justify-center`}>
      <Spinner className={tw`size-16`} />
    </div>
  ),
});

export const nodeTypes: Record<NodeKindJson, NodeTypesCore[string]> = {
  NODE_KIND_CONDITION: ConditionNode,
  NODE_KIND_FOR: ForNode,
  NODE_KIND_FOR_EACH: ForEachNode,
  NODE_KIND_JS: JavaScriptNode,
  NODE_KIND_NO_OP: NoOpNode,
  NODE_KIND_REQUEST: RequestNode,
  NODE_KIND_UNSPECIFIED: () => null,
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
              <Panel className='flex h-full flex-col' id='flow' order={1}>
                <Flow flowId={flowId} key={Ulid.construct(flowId).toCanonical()}>
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
  children?: ReactNode;
  flowId: Uint8Array;
}

export const Flow = ({ children, flowId }: FlowProps) => {
  const { transport } = useRouteContext({ from: '__root__' });

  const [edgesQuery, nodesQuery] = useSuspenseQueries({
    queries: [edgesQueryOptions({ flowId, transport }), nodesQueryOptions({ flowId, transport })],
  });

  return (
    <FlowView edges={edgesQuery.data} nodes={nodesQuery.data}>
      {children}
    </FlowView>
  );
};

interface FlowViewProps {
  children?: ReactNode;
  edges: Edge[];
  isReadOnly?: boolean;
  nodes: Node[];
}

const minZoom = 0.5;
const maxZoom = 2;

const FlowView = ({ children, edges, nodes }: FlowViewProps) => {
  const { addEdges, addNodes, getEdges, getNode, screenToFlowPosition } = useReactFlow<Node, Edge>();
  const { isReadOnly = false } = use(FlowContext);

  const navigate = useNavigate();

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
    async (event, { fromNode, isValid }) => {
      if (!(event instanceof MouseEvent)) return;
      if (isValid) return;
      if (fromNode === null) return;

      const node = await makeNode({
        kind: NodeKind.NO_OP,
        noOp: NodeNoOpKind.CREATE,
        position: screenToFlowPosition({ x: event.clientX, y: event.clientY }),
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
    ({ edges, nodes }) => {
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
        edges: pipe(Record.fromEntries(deleteEdgeMap), Record.values),
        nodes: pipe(Record.fromEntries(deleteNodeMap), Record.values),
      });
    },
    [getEdges, getNode, isReadOnly],
  );

  return (
    <ReactFlow
      colorMode='light'
      connectionLineComponent={ConnectionLine}
      defaultEdgeOptions={{ type: 'default' }}
      deleteKeyCode={['Backspace', 'Delete']}
      edges={edges}
      edgeTypes={edgeTypes}
      fitView
      maxZoom={maxZoom}
      minZoom={minZoom}
      nodes={nodes}
      nodesConnectable={!isReadOnly}
      nodesDraggable={!isReadOnly}
      nodeTypes={nodeTypes}
      onBeforeDelete={onBeforeDelete}
      onConnect={onConnect}
      onConnectEnd={onConnectEnd}
      onEdgesChange={onEdgesChange}
      onlyRenderVisibleElements
      onNodeDoubleClick={(_, node) => void navigate({ search: (_) => ({ ..._, node: node.id }), to: '.' })}
      onNodesChange={onNodesChange}
      panOnDrag={[1, 2]}
      panOnScroll
      proOptions={{ hideAttribution: true }}
      selectionMode={SelectionMode.Partial}
      selectionOnDrag
      selectNodesOnDrag={false}
    >
      <Background
        className={tw`text-slate-300`}
        color='currentColor'
        gap={20}
        size={2}
        variant={BackgroundVariant.Dots}
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
  const navigate = useNavigate();

  const flowUpdateMutation = useConnectMutation(flowUpdate);
  const flowDeleteMutation = useConnectMutation(flowDelete, {
    onSuccess: async () => {
      if (
        matchRoute({
          params: { flowIdCan: Ulid.construct(flowId).toCanonical() },
          to: '/workspace/$workspaceIdCan/flow/$flowIdCan',
        })
      ) {
        await navigate({ from: Route.fullPath, to: '/workspace/$workspaceIdCan' });
      }
    },
  });

  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    onSuccess: (_) => flowUpdateMutation.mutateAsync({ flowId, name: _ }),
    value: name,
  });

  return (
    <div className={tw`flex items-center gap-2 border-b border-slate-200 bg-white px-3 py-2.5`}>
      {isEditing ? (
        <TextField
          inputClassName={tw`text-md -my-1 py-1 font-medium leading-none tracking-tight text-slate-800`}
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
        className={tw`p-0.5`}
        isDisabled={zoom <= minZoom}
        onPress={() => void zoomOut({ duration: 100 })}
        variant='ghost'
      >
        <FiMinus className={tw`size-4 text-slate-500`} />
      </Button>

      <div className={tw`w-10 text-center text-sm font-medium leading-5 tracking-tight text-gray-900`}>
        {Math.floor(zoom * 100)}%
      </div>

      <Button
        className={tw`p-0.5`}
        isDisabled={zoom >= maxZoom}
        onPress={() => void zoomIn({ duration: 100 })}
        variant='ghost'
      >
        <FiPlus className={tw`size-4 text-slate-500`} />
      </Button>

      <div className={tw`h-4 w-px bg-slate-200`} />

      <ButtonAsLink
        className={tw`px-2 py-1 text-slate-800`}
        href={{
          from: '/workspace/$workspaceIdCan/flow/$flowIdCan',
          to: matchRoute({ to: '/workspace/$workspaceIdCan/flow/$flowIdCan/history' })
            ? '/workspace/$workspaceIdCan/flow/$flowIdCan'
            : '/workspace/$workspaceIdCan/flow/$flowIdCan/history',
        }}
        variant='ghost'
      >
        <FiClock className={tw`size-4 text-slate-500`} /> Flows History
      </ButtonAsLink>

      <MenuTrigger {...menuTriggerProps}>
        <Button className={tw`bg-slate-200 p-0.5`} variant='ghost'>
          <FiMoreHorizontal className={tw`size-4 text-slate-500`} />
        </Button>

        <Menu {...menuProps}>
          <MenuItem onAction={() => void edit()}>Rename</MenuItem>

          <Separator />

          <MenuItem onAction={() => void flowDeleteMutation.mutate({ flowId })} variant='danger'>
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

  const normalizer = useQueryNormalizer();

  const makeNode = useMakeNode();

  return (
    <RFPanel
      className={tw`mb-4 flex items-center gap-2 rounded-lg bg-slate-900 p-1 shadow-sm`}
      position='bottom-center'
    >
      {/* <Button variant='ghost dark' className={tw`p-1`}>
        <TextBoxIcon className={tw`size-5 text-slate-300`} />
      </Button> */}

      {/* <Button variant='ghost dark' className={tw`p-1`}>
        <ChatAddIcon className={tw`size-5 text-slate-300`} />
      </Button> */}

      {/* <div className={tw`mx-2 h-5 w-px bg-white/20`} /> */}

      <Button
        className={tw`px-1.5 py-1`}
        onPress={async () => {
          const { domNode } = storeApi.getState();
          if (!domNode) return;
          const box = domNode.getBoundingClientRect();
          const position = flow.screenToFlowPosition({ x: box.x + box.width / 2, y: box.y + box.height * 0.1 });
          const node = await makeNode({ kind: NodeKind.NO_OP, noOp: NodeNoOpKind.CREATE, position });
          pipe(node, Node.fromDTO, flow.addNodes);
        }}
        variant='ghost dark'
      >
        <FiPlus className={tw`size-5 text-slate-300`} />
        Add Node
      </Button>

      <Button
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

          for await (const { changes, nodeId, state } of flowRun({ flowId })) {
            const nodeIdCan = Ulid.construct(nodeId).toCanonical();

            flow.updateNodeData(nodeIdCan, (_) => ({ ..._, state }));

            pipe(
              HashMap.get(sourceEdges, nodeIdCan),
              Array.fromOption,
              Array.flatten,
              Array.forEach((_) => void flow.updateEdgeData(_.id, (_) => ({ ..._, state }))),
            );

            await normalizer.setNormalizedData(changes);
          }
        }}
        variant='primary'
      >
        <PlayCircleIcon className={tw`size-4`} />
        Run
      </Button>
    </RFPanel>
  );
};

const variableColumnHelper = createColumnHelper<FormTableItem<FlowVariableListItem>>();

const variableColumns = [
  genericFormTableEnableColumn,
  variableColumnHelper.accessor('data.name', {
    cell: ({ row, table }) => (
      <TextFieldRHF control={table.options.meta!.control!} name={`items.${row.index}.data.name`} variant='table-cell' />
    ),
    header: 'Name',
    meta: { divider: false },
  }),
  variableColumnHelper.accessor('data.value', {
    cell: ({ row, table }) => (
      <TextFieldRHF
        control={table.options.meta!.control!}
        name={`items.${row.index}.data.value`}
        variant='table-cell'
      />
    ),
    header: 'Value',
  }),
  variableColumnHelper.accessor('data.description', {
    cell: ({ row, table }) => (
      <TextFieldRHF
        control={table.options.meta!.control!}
        name={`items.${row.index}.data.description`}
        variant='table-cell'
      />
    ),
    header: 'Description',
  }),
  genericFormTableActionColumn,
];

const SettingsPanel = () => {
  'use no memo';

  const { flowId } = flowRoute.useLoaderData();

  const create = useConnectMutation(flowVariableCreate);
  const delete$ = useConnectMutation(flowVariableDelete);
  const update = useConnectMutation(flowVariableUpdate);

  const {
    data: { items },
  } = useConnectSuspenseQuery(flowVariableList, { flowId });

  const table = useFormTable({
    columns: variableColumns as ColumnDef<FormTableItem<FlowVariableListItem>>[],
    items,
    onCreate: (_) => create.mutateAsync({ ...Struct.omit(_, '$typeName'), flowId }).then((_) => _.variableId),
    onDelete: (_) => delete$.mutateAsync(Struct.omit(_, '$typeName')),
    onUpdate: (_) => update.mutateAsync(Struct.omit(_, '$typeName')),
    schema: FlowVariableListItemSchema,
  });

  return (
    <>
      <div className={tw`sticky top-0 z-10 flex items-center border-b border-slate-200 bg-white px-5 py-2`}>
        <div className={tw`text-sm font-medium leading-5 text-slate-800`}>Flow settings</div>

        <div className={tw`flex-1`} />

        <ButtonAsLink
          className={tw`p-1`}
          href={{ search: (_: Partial<FlowSearch>) => ({ ..._, node: undefined }), to: '.' }}
          variant='ghost'
        >
          <FiX className={tw`size-5 text-slate-500`} />
        </ButtonAsLink>
      </div>

      <div className={tw`m-5`}>
        <DataTable table={table} />
      </div>
    </>
  );
};

export const EditPanel = () => {
  const { workspaceId } = workspaceRoute.useLoaderData();
  const { nodeId } = flowRoute.useLoaderData();

  const nodeQuery = useConnectQuery(
    nodeGet,
    { nodeId: Option.getOrUndefined(nodeId)! },
    { enabled: Option.isSome(nodeId) },
  );

  if (Option.isNone(nodeId) || !nodeQuery.data) return null;

  const view = pipe(
    Match.value(nodeQuery.data),
    Match.when({ kind: NodeKind.NO_OP, noOp: NodeNoOpKind.START }, () => <SettingsPanel />),
    Match.when({ kind: NodeKind.CONDITION }, () => <ConditionPanel node={nodeQuery.data} />),
    Match.when({ kind: NodeKind.FOR_EACH }, () => <ForEachPanel node={nodeQuery.data} />),
    Match.when({ kind: NodeKind.FOR }, () => <ForPanel node={nodeQuery.data} />),
    Match.when({ kind: NodeKind.JS }, () => <JavaScriptPanel node={nodeQuery.data} />),
    Match.when({ kind: NodeKind.REQUEST }, () => <RequestPanel node={nodeQuery.data} />),
    Match.orElse(() => null),
  );

  if (!view) return null;

  return (
    <ReferenceContext value={{ nodeId: nodeId.value, workspaceId }}>
      <PanelResizeHandle direction='vertical' />
      <Panel className={tw`!overflow-auto`} defaultSize={40} id='node' order={2}>
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
