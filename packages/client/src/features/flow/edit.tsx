import { create } from '@bufbuild/protobuf';
import { eq, useLiveQuery } from '@tanstack/react-db';
import { useMatchRoute, useNavigate } from '@tanstack/react-router';
import * as XF from '@xyflow/react';
import { Array, Boolean, Duration, HashSet, Match, MutableHashMap, Option, pipe, Record } from 'effect';
import { Ulid } from 'id128';
import { PropsWithChildren, ReactNode, use, useRef } from 'react';
import { useDrop } from 'react-aria';
import { Button as AriaButton, MenuTrigger, useDragAndDrop } from 'react-aria-components';
import { createPortal } from 'react-dom';
import { FiClock, FiMinus, FiMoreHorizontal, FiPlus, FiStopCircle, FiX } from 'react-icons/fi';
import { Panel, PanelGroup } from 'react-resizable-panels';
import { twJoin } from 'tailwind-merge';
import { FileKind } from '@the-dev-tools/spec/buf/api/file_system/v1/file_system_pb';
import {
  EdgeKind,
  FlowSchema,
  FlowService,
  FlowVariable,
  HandleKind,
  NodeKind,
  NodeNoOpKind,
  NodeNoOpSchema,
  NodeSchema,
} from '@the-dev-tools/spec/buf/api/flow/v1/flow_pb';
import { FileCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/file_system';
import {
  EdgeCollectionSchema,
  FlowCollectionSchema,
  FlowVariableCollectionSchema,
  NodeCollectionSchema,
  NodeHttpCollectionSchema,
  NodeNoOpCollectionSchema,
} from '@the-dev-tools/spec/tanstack-db/v1/api/flow';
import { Button, ButtonAsLink } from '@the-dev-tools/ui/button';
import { DataTable, useReactTable } from '@the-dev-tools/ui/data-table';
import { PlayCircleIcon } from '@the-dev-tools/ui/icons';
import { Menu, MenuItem, useContextMenuState } from '@the-dev-tools/ui/menu';
import { DropIndicatorHorizontal } from '@the-dev-tools/ui/reorder';
import { PanelResizeHandle } from '@the-dev-tools/ui/resizable-panel';
import { Separator } from '@the-dev-tools/ui/separator';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextInputField, useEditableTextState } from '@the-dev-tools/ui/text-field';
import { Connect, useApiCollection } from '~/api';
import {
  columnActionsCommon,
  columnCheckboxField,
  columnReferenceField,
  columnTextField,
  useFormTable,
  useFormTableAddRow,
} from '~/form-table';
import { ReferenceContext } from '~/reference';
import { flowHistoryRouteApi, flowLayoutRouteApi, rootRouteApi, workspaceRouteApi } from '~/routes';
import { getNextOrder, handleCollectionReorder } from '~/utils/order';
import { pick } from '~/utils/tanstack-db';
import { FlowContext } from './context';
import { ConnectionLine, edgeTypes, useEdgeState } from './edge';
import { NodeStateContext, useNodesState } from './node';
import { ConditionNode, ConditionPanel } from './nodes/condition';
import { ForNode, ForPanel } from './nodes/for';
import { ForEachNode, ForEachPanel } from './nodes/for-each';
import { HttpNode, HttpPanel } from './nodes/http';
import { JavaScriptNode, JavaScriptPanel } from './nodes/javascript';
import { NoOpNode } from './nodes/no-op';
import { useViewport, VIEWPORT_MAX_ZOOM, VIEWPORT_MIN_ZOOM } from './viewport';

export const nodeTypes: XF.NodeTypes = {
  [NodeKind.CONDITION]: ConditionNode,
  [NodeKind.FOR]: ForNode,
  [NodeKind.FOR_EACH]: ForEachNode,
  [NodeKind.HTTP]: HttpNode,
  [NodeKind.JS]: JavaScriptNode,
  [NodeKind.NO_OP]: NoOpNode,
  [NodeKind.UNSPECIFIED]: () => null,
};

export const FlowEditPage = () => {
  const { flowId, nodeId } = flowLayoutRouteApi.useLoaderData();

  const flow = (
    <Flow key={Ulid.construct(flowId).toCanonical()}>
      <ActionBar />
    </Flow>
  );

  return (
    <FlowContext.Provider value={{ flowId }}>
      <XF.ReactFlowProvider>
        {Option.isNone(nodeId) ? (
          <PanelGroup direction='vertical'>
            <TopBarWithControls />
            <Panel className={tw`flex h-full flex-col`} defaultSize={100} id='flow' minSize={100} order={1}>
              {flow}
            </Panel>
            <PanelResizeHandle direction='vertical' />
            <Panel defaultSize={0} id='node' maxSize={0} order={2} />
          </PanelGroup>
        ) : (
          <PanelGroup autoSaveId='flow-edit-node' direction='vertical'>
            <TopBarWithControls />
            <Panel className={tw`flex h-full flex-col`} defaultSize={60} id='flow' order={1}>
              {flow}
            </Panel>
            <EditPanel nodeId={nodeId.value} />
          </PanelGroup>
        )}
      </XF.ReactFlowProvider>
    </FlowContext.Provider>
  );
};

export const Flow = ({ children }: PropsWithChildren) => {
  const fileCollection = useApiCollection(FileCollectionSchema);
  const flowCollection = useApiCollection(FlowCollectionSchema);
  const edgeCollection = useApiCollection(EdgeCollectionSchema);
  const nodeCollection = useApiCollection(NodeCollectionSchema);
  const nodeNoOpCollection = useApiCollection(NodeNoOpCollectionSchema);
  const nodeHttpCollection = useApiCollection(NodeHttpCollectionSchema);

  const { getEdges, getNode, getNodes, screenToFlowPosition } = XF.useReactFlow();

  const { flowId, isReadOnly = false } = use(FlowContext);

  const { duration } =
    useLiveQuery(
      (_) =>
        _.from({ item: flowCollection })
          .where((_) => eq(_.item.flowId, flowId))
          .select((_) => pick(_.item, 'duration'))
          .findOne(),
      [flowCollection, flowId],
    ).data ?? create(FlowSchema);

  const ref = useRef<HTMLDivElement>(null);

  const navigate = useNavigate();

  const { nodes, onNodesChange, setNodeSelection } = useNodesState();
  const { edges, onEdgesChange } = useEdgeState();
  const { onViewportChange, viewport } = useViewport();

  const onConnect: XF.OnConnect = (_) =>
    void edgeCollection.utils.insert({
      edgeId: Ulid.generate().bytes,
      flowId,
      sourceHandle: _.sourceHandle ? parseInt(_.sourceHandle) : 0,
      sourceId: Ulid.fromCanonical(_.source).bytes,
      targetId: Ulid.fromCanonical(_.target).bytes,
    });

  const onConnectEnd: XF.OnConnectEnd = (event, { fromNode, isValid }) => {
    if (!(event instanceof MouseEvent)) return;
    if (isValid) return;
    if (fromNode === null) return;

    const nodeUlid = Ulid.generate();

    setNodeSelection(() => HashSet.make(nodeUlid.toCanonical()));

    nodeNoOpCollection.utils.insert({ kind: NodeNoOpKind.CREATE, nodeId: nodeUlid.bytes });

    nodeCollection.utils.insert({
      flowId,
      kind: NodeKind.NO_OP,
      nodeId: nodeUlid.bytes,
      position: screenToFlowPosition({ x: event.clientX, y: event.clientY }),
    });

    edgeCollection.utils.insert({
      edgeId: Ulid.generate().bytes,
      flowId,
      kind: EdgeKind.NO_OP,
      sourceId: Ulid.fromCanonical(fromNode.id).bytes,
      targetId: nodeUlid.bytes,
    });
  };

  const onBeforeDelete: XF.OnBeforeDelete = ({ edges, nodes }) => {
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
      Array.map(Option.liftPredicate((_) => !!_.sourceHandle && _.sourceHandle !== HandleKind.UNSPECIFIED.toString())),
      Array.getSomes,
    );

    // Do 2 passes to find indirectly referenced edges through edge source
    for (let i = 0; i < 2; i++) {
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
    }

    return Promise.resolve({
      edges: pipe(Record.fromEntries(deleteEdgeMap), Record.values),
      nodes: pipe(Record.fromEntries(deleteNodeMap), Record.values),
    });
  };

  const { dropProps } = useDrop({
    onDrop: async ({ items, x, y }) => {
      const [item] = items;
      if (!item || item.kind !== 'text' || !item.types.has('key') || items.length !== 1) return;

      const file = fileCollection.get(await item.getText('key'));

      const canvas = ref.current?.getBoundingClientRect() ?? { x: 0, y: 0 };
      const position = screenToFlowPosition({ x: x + canvas.x, y: y + canvas.y });

      if (file?.kind === FileKind.HTTP) {
        const nodeId = Ulid.generate().bytes;

        nodeHttpCollection.utils.insert({
          httpId: file.fileId,
          nodeId,
        });

        nodeCollection.utils.insert({
          flowId,
          kind: NodeKind.HTTP,
          name: `http_${getNodes().length + 1}`,
          nodeId,
          position,
        });
      }

      if (file?.kind === FileKind.HTTP_DELTA) {
        const nodeId = Ulid.generate().bytes;

        nodeHttpCollection.utils.insert({
          deltaHttpId: file.fileId,
          httpId: file.parentId!,
          nodeId,
        });

        nodeCollection.utils.insert({
          flowId,
          kind: NodeKind.HTTP,
          name: `http_${getNodes().length + 1}`,
          nodeId,
          position,
        });
      }
    },
    ref,
  });

  const statusBarEndSlot = document.getElementById('statusBarEndSlot');

  return (
    <NodeStateContext value={{ setNodeSelection }}>
      {statusBarEndSlot &&
        createPortal(
          <div className={tw`flex gap-4 text-xs leading-none text-slate-800`}>
            <NodeSelectionIndicator />
            {duration && <div>Time: {pipe(duration, Duration.millis, Duration.format)}</div>}
          </div>,
          statusBarEndSlot,
        )}

      <XF.ReactFlow
        {...(dropProps as object)}
        colorMode='light'
        connectionLineComponent={ConnectionLine}
        defaultEdgeOptions={{ type: EdgeKind.UNSPECIFIED.toString() }}
        deleteKeyCode={['Backspace', 'Delete']}
        edges={edges}
        edgeTypes={edgeTypes}
        maxZoom={VIEWPORT_MAX_ZOOM}
        minZoom={VIEWPORT_MIN_ZOOM}
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
        onViewportChange={onViewportChange}
        panOnDrag={[1, 2]}
        panOnScroll
        proOptions={{ hideAttribution: true }}
        ref={ref}
        selectionMode={XF.SelectionMode.Partial}
        selectionOnDrag
        selectNodesOnDrag={false}
        viewport={viewport}
      >
        <XF.Background
          className={tw`text-slate-300`}
          color='currentColor'
          gap={20}
          size={2}
          variant={XF.BackgroundVariant.Dots}
        />
        {children}
      </XF.ReactFlow>
    </NodeStateContext>
  );
};

const NodeSelectionIndicator = () => {
  const count = XF.useStore((_) => _.nodes.filter((_) => _.selected).length);
  if (count === 0) return null;
  return <div>{count} nodes selected</div>;
};

interface TopBarProps {
  children?: ReactNode;
}

export const TopBar = ({ children }: TopBarProps) => {
  const { flowId } = flowLayoutRouteApi.useLoaderData();
  const { flowIdCan, workspaceIdCan } = flowLayoutRouteApi.useParams();

  const collection = useApiCollection(FlowCollectionSchema);

  const { name } =
    useLiveQuery(
      (_) =>
        _.from({ item: collection })
          .where((_) => eq(_.item.flowId, flowId))
          .select((_) => pick(_.item, 'name'))
          .findOne(),
      [collection, flowId],
    ).data ?? create(FlowSchema);

  const matchRoute = useMatchRoute();

  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    onSuccess: (_) => collection.utils.update({ flowId, name: _ }),
    value: name,
  });

  return (
    <div className={tw`flex items-center gap-2 border-b border-slate-200 bg-white px-3 py-2.5`}>
      {isEditing ? (
        <TextInputField
          aria-label='Flow name'
          inputClassName={tw`-my-1 py-1 text-md leading-none font-medium tracking-tight text-slate-800`}
          {...textFieldProps}
        />
      ) : (
        <AriaButton
          className={tw`cursor-text text-md leading-5 font-medium tracking-tight text-slate-800`}
          onContextMenu={onContextMenu}
          onPress={() => void edit()}
        >
          {name}
        </AriaButton>
      )}

      <div className={tw`flex-1`} />

      {children}

      <ButtonAsLink
        className={twJoin(tw`px-1 py-0 text-slate-800`, matchRoute({ to: flowHistoryRouteApi.id }) && tw`bg-slate-200`)}
        params={{ flowIdCan, workspaceIdCan }}
        to={matchRoute({ to: flowHistoryRouteApi.id }) ? flowLayoutRouteApi.id : flowHistoryRouteApi.id}
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

          <MenuItem onAction={() => void collection.utils.delete({ flowId })} variant='danger'>
            Delete
          </MenuItem>
        </Menu>
      </MenuTrigger>
    </div>
  );
};

export const TopBarWithControls = () => {
  const { zoomIn, zoomOut } = XF.useReactFlow();
  const { zoom } = XF.useViewport();

  return (
    <TopBar>
      <Button
        className={tw`p-0.5`}
        isDisabled={zoom <= VIEWPORT_MIN_ZOOM}
        onPress={() => void zoomOut({ duration: 100 })}
        variant='ghost'
      >
        <FiMinus className={tw`size-4 text-slate-500`} />
      </Button>

      <div className={tw`w-10 text-center text-sm leading-5 font-medium tracking-tight text-gray-900`}>
        {Math.floor(zoom * 100)}%
      </div>

      <Button
        className={tw`p-0.5`}
        isDisabled={zoom >= VIEWPORT_MAX_ZOOM}
        onPress={() => void zoomIn({ duration: 100 })}
        variant='ghost'
      >
        <FiPlus className={tw`size-4 text-slate-500`} />
      </Button>

      <div className={tw`h-4 w-px bg-slate-200`} />
    </TopBar>
  );
};

const ActionBar = () => {
  const { flowId } = use(FlowContext);
  const { setNodeSelection } = use(NodeStateContext);
  const { transport } = rootRouteApi.useRouteContext();
  const flow = XF.useReactFlow();
  const storeApi = XF.useStoreApi();

  const flowCollection = useApiCollection(FlowCollectionSchema);
  const nodeCollection = useApiCollection(NodeCollectionSchema);
  const noOpCollection = useApiCollection(NodeNoOpCollectionSchema);

  const { running } =
    useLiveQuery(
      (_) =>
        _.from({ item: flowCollection })
          .where((_) => eq(_.item.flowId, flowId))
          .select((_) => pick(_.item, 'running'))
          .findOne(),
      [flowCollection, flowId],
    ).data ?? create(FlowSchema);

  return (
    <XF.Panel
      className={tw`mb-4 flex items-center gap-2 rounded-lg bg-slate-900 p-1 shadow-sm`}
      position='bottom-center'
    >
      <Button
        className={tw`px-1.5 py-1`}
        onPress={() => {
          const { domNode } = storeApi.getState();
          if (!domNode) return;
          const box = domNode.getBoundingClientRect();
          const position = flow.screenToFlowPosition({ x: box.x + box.width / 2, y: box.y + box.height * 0.1 });

          const nodeUlid = Ulid.generate();
          setNodeSelection(() => HashSet.make(nodeUlid.toCanonical()));
          noOpCollection.utils.insert({ kind: NodeNoOpKind.CREATE, nodeId: nodeUlid.bytes });
          nodeCollection.utils.insert({ flowId, kind: NodeKind.NO_OP, nodeId: nodeUlid.bytes, position });
        }}
        variant='ghost dark'
      >
        <FiPlus className={tw`size-5 text-slate-300`} />
        Add Node
      </Button>

      {running ? (
        <Button
          onPress={() => Connect.request({ input: { flowId }, method: FlowService.method.flowStop, transport })}
          variant='primary'
        >
          <FiStopCircle className={tw`size-4`} />
          Stop
        </Button>
      ) : (
        <Button
          onPress={() => Connect.request({ input: { flowId }, method: FlowService.method.flowRun, transport })}
          variant='primary'
        >
          <PlayCircleIcon className={tw`size-4`} />
          Run
        </Button>
      )}
    </XF.Panel>
  );
};

const SettingsPanel = () => {
  const { flowId } = use(FlowContext);

  const collection = useApiCollection(FlowVariableCollectionSchema);

  const { data: variables } = useLiveQuery(
    (_) =>
      _.from({ variable: collection })
        .where((_) => eq(_.variable.flowId, flowId))
        .orderBy((_) => _.variable.order),
    [collection, flowId],
  );

  const table = useReactTable({
    columns: [
      columnCheckboxField<FlowVariable>('enabled', { meta: { divider: false } }),
      columnReferenceField<FlowVariable>('key', { meta: { isRowHeader: true } }),
      columnReferenceField<FlowVariable>('value', { allowFiles: true }),
      columnTextField<FlowVariable>('description', { meta: { divider: false } }),
      columnActionsCommon<FlowVariable>({
        onDelete: (_) => collection.utils.delete(collection.utils.getKeyObject(_)),
      }),
    ],
    data: variables,
    getRowId: (_) => collection.utils.getKey(_),
  });

  const formTable = useFormTable<FlowVariable>({
    onUpdate: ({ $typeName: _, ...item }) => collection.utils.update(item),
  });

  const addRow = useFormTableAddRow({
    createLabel: 'New variable',
    items: variables,
    onCreate: async () =>
      collection.utils.insert({
        enabled: true,
        flowId,
        flowVariableId: Ulid.generate().bytes,
        key: `FLOW_VARIABLE_${variables.length}`,
        order: await getNextOrder(collection),
      }),
    primaryColumn: 'key',
  });

  const { dragAndDropHooks } = useDragAndDrop({
    getItems: (keys) => [...keys].map((key) => ({ key: key.toString() })),
    onReorder: handleCollectionReorder(collection),
    renderDropIndicator: () => <DropIndicatorHorizontal as='tr' />,
  });

  return (
    <>
      <div className={tw`sticky top-0 z-10 flex items-center border-b border-slate-200 bg-white px-5 py-2`}>
        <div className={tw`text-sm leading-5 font-medium text-slate-800`}>Flow variables</div>

        <div className={tw`flex-1`} />

        <ButtonAsLink className={tw`p-1`} search={(_) => ({ ..._, node: undefined })} to='.' variant='ghost'>
          <FiX className={tw`size-5 text-slate-500`} />
        </ButtonAsLink>
      </div>

      <div className={tw`m-5`}>
        <DataTable
          {...formTable}
          {...addRow}
          aria-label='Flow variables'
          dragAndDropHooks={dragAndDropHooks}
          table={table}
        />
      </div>
    </>
  );
};

interface EditPanelProps {
  nodeId: Uint8Array;
}

export const EditPanel = ({ nodeId }: EditPanelProps) => {
  const { workspaceId } = workspaceRouteApi.useLoaderData();

  const nodeCollection = useApiCollection(NodeCollectionSchema);
  const noOpCollection = useApiCollection(NodeNoOpCollectionSchema);

  const { kind } =
    useLiveQuery(
      (_) =>
        _.from({ item: nodeCollection })
          .where((_) => eq(_.item.nodeId, nodeId))
          .select((_) => pick(_.item, 'kind'))
          .findOne(),
      [nodeCollection, nodeId],
    ).data ?? create(NodeSchema);

  const { kind: noOpKind } =
    useLiveQuery(
      (_) =>
        _.from({ item: noOpCollection })
          .where((_) => eq(_.item.nodeId, nodeId))
          .select((_) => pick(_.item, 'kind'))
          .findOne(),
      [noOpCollection, nodeId],
    ).data ?? create(NodeNoOpSchema);

  const view = pipe(
    Match.value({ kind, noOpKind }),
    Match.when({ kind: NodeKind.NO_OP, noOpKind: NodeNoOpKind.START }, () => <SettingsPanel />),
    Match.when({ kind: NodeKind.CONDITION }, () => <ConditionPanel nodeId={nodeId} />),
    Match.when({ kind: NodeKind.FOR_EACH }, () => <ForEachPanel nodeId={nodeId} />),
    Match.when({ kind: NodeKind.FOR }, (_) => <ForPanel nodeId={nodeId} />),
    Match.when({ kind: NodeKind.JS }, (_) => <JavaScriptPanel nodeId={nodeId} />),
    Match.when({ kind: NodeKind.HTTP }, (_) => <HttpPanel nodeId={nodeId} />),
    Match.orElse(() => null),
  );

  if (!view) return null;

  return (
    <ReferenceContext value={{ flowNodeId: nodeId, workspaceId }}>
      <PanelResizeHandle direction='vertical' />
      <Panel className={tw`!overflow-auto`} defaultSize={40} id='node' order={2}>
        {view}
      </Panel>
    </ReferenceContext>
  );
};
