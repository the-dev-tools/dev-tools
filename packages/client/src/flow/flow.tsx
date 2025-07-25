import { enumFromJson, isEnumJson } from '@bufbuild/protobuf';
import { createClient } from '@connectrpc/connect';
import { createFileRoute, useBlocker, useMatchRoute, useNavigate, useRouteContext } from '@tanstack/react-router';
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
import { Array, Boolean, HashMap, Match, MutableHashMap, Option, pipe, Predicate, Record, Schema } from 'effect';
import { Ulid } from 'id128';
import { PropsWithChildren, Suspense, use, useCallback, useMemo, useRef, useState } from 'react';
import { useDrop } from 'react-aria';
import { Dialog, MenuTrigger, useDragAndDrop } from 'react-aria-components';
import { FiClock, FiMinus, FiMoreHorizontal, FiPlus, FiStopCircle, FiX } from 'react-icons/fi';
import { Panel, PanelGroup } from 'react-resizable-panels';
import { Example } from '@the-dev-tools/spec/collection/item/example/v1/example_pb';
import { EdgeKind, EdgeKindJson } from '@the-dev-tools/spec/flow/edge/v1/edge_pb';
import { NodeKind, NodeKindJson, NodeNoOpKind, NodeState } from '@the-dev-tools/spec/flow/node/v1/node_pb';
import { FlowService } from '@the-dev-tools/spec/flow/v1/flow_pb';
import { EndpointCreateEndpoint } from '@the-dev-tools/spec/meta/collection/item/endpoint/v1/endpoint.endpoints.js';
import {
  ExampleCreateEndpoint,
  ExampleVersionsEndpoint,
} from '@the-dev-tools/spec/meta/collection/item/example/v1/example.endpoints.js';
import {
  ExampleEntity,
  ExampleVersionsItemEntity,
} from '@the-dev-tools/spec/meta/collection/item/example/v1/example.entities.js';
import { NodeExecutionListEndpoint } from '@the-dev-tools/spec/meta/flow/node/execution/v1/execution.endpoints.js';
import { NodeGetEndpoint } from '@the-dev-tools/spec/meta/flow/node/v1/node.endpoints.js';
import {
  FlowDeleteEndpoint,
  FlowGetEndpoint,
  FlowUpdateEndpoint,
  FlowVersionsEndpoint,
} from '@the-dev-tools/spec/meta/flow/v1/flow.endpoints.ts';
import {
  FlowVariableCreateEndpoint,
  FlowVariableDeleteEndpoint,
  FlowVariableListEndpoint,
  FlowVariableMoveEndpoint,
  FlowVariableUpdateEndpoint,
} from '@the-dev-tools/spec/meta/flowvariable/v1/flowvariable.endpoints.ts';
import { FlowVariableListItemEntity } from '@the-dev-tools/spec/meta/flowvariable/v1/flowvariable.entities.ts';
import { MovePosition } from '@the-dev-tools/spec/resources/v1/resources_pb';
import { Button, ButtonAsLink } from '@the-dev-tools/ui/button';
import { DataTable, useReactTable } from '@the-dev-tools/ui/data-table';
import { PlayCircleIcon, Spinner } from '@the-dev-tools/ui/icons';
import { Menu, MenuItem, useContextMenuState } from '@the-dev-tools/ui/menu';
import { Modal } from '@the-dev-tools/ui/modal';
import { PanelResizeHandle } from '@the-dev-tools/ui/resizable-panel';
import { Separator } from '@the-dev-tools/ui/separator';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextField, useEditableTextState } from '@the-dev-tools/ui/text-field';
import { EndpointKey, ExampleKey, TreeKey } from '~collection';
import { setQueryChild, useDLE, useEndpointProps, useMutate, useQuery } from '~data-client';
import {
  columnActionsCommon,
  columnCheckboxField,
  columnReferenceField,
  columnTextField,
  useFormTable,
} from '~form-table';
import { ReferenceContext } from '../reference';
import { useFlowCopyPaste } from './copy-paste';
import { ConnectionLine, Edge, edgeTypes, useMakeEdge } from './edge';
import { FlowContext, flowRoute, HandleKind, HandleKindSchema, workspaceRoute } from './internal';
import { useOnFlowDelete } from './layout';
import { Node, useMakeNode } from './node';
import { ConditionNode, ConditionPanel } from './nodes/condition';
import { ForNode, ForPanel } from './nodes/for';
import { ForEachNode, ForEachPanel } from './nodes/for-each';
import { JavaScriptNode, JavaScriptPanel } from './nodes/javascript';
import { NoOpNode } from './nodes/no-op';
import { RequestNode, RequestPanel } from './nodes/request';
import { useFlowStateSynced } from './sync';

const makeRoute = createFileRoute('/_authorized/workspace/$workspaceIdCan/flow/$flowIdCan/');

export const Route = makeRoute({
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
    <Suspense
      fallback={
        <div className={tw`flex h-full items-center justify-center`}>
          <Spinner className={tw`size-16`} />
        </div>
      }
    >
      <FlowContext.Provider value={{ flowId }}>
        <ReactFlowProvider>
          <PanelGroup direction='vertical'>
            <TopBar />
            <Panel className='flex h-full flex-col' id='flow' order={1}>
              <Flow key={Ulid.construct(flowId).toCanonical()}>
                <ActionBar />
              </Flow>
            </Panel>
            <EditPanel />
          </PanelGroup>
        </ReactFlowProvider>
      </FlowContext.Provider>
    </Suspense>
  );
}

const minZoom = 0.1;
const maxZoom = 2;

export const Flow = ({ children }: PropsWithChildren) => {
  const { dataClient } = useRouteContext({ from: '__root__' });

  const { addEdges, addNodes, getEdges, getNode, getNodes, screenToFlowPosition, setNodes } = useReactFlow<
    Node,
    Edge
  >();
  const { isReadOnly = false } = use(FlowContext);

  const ref = useRef<HTMLDivElement>(null);

  const navigate = useNavigate();

  const { edges, nodes, onEdgesChange, onNodesChange } = useFlowStateSynced();

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
        kind: EdgeKind.NO_OP,
        sourceId: Ulid.fromCanonical(fromNode.id).bytes,
        targetId: node.nodeId,
      });

      setNodes((_) => _.map((_) => ({ ..._, selected: false })));

      pipe(Node.fromDTO(node, { selected: true }), addNodes);
      pipe(Edge.fromDTO(edge), addEdges);
    },
    [addEdges, addNodes, makeEdge, makeNode, screenToFlowPosition, setNodes],
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

  useFlowCopyPaste();

  const { dropProps } = useDrop({
    onDrop: async ({ items, x, y }) => {
      const [item] = items;
      if (!item || item.kind !== 'text' || !item.types.has('key') || items.length !== 1) return;

      const key = await pipe(Schema.parseJson(TreeKey), Schema.decodeUnknownSync, async (decode) =>
        pipe(await item.getText('key'), decode),
      );

      if (key._tag !== EndpointKey._tag && key._tag !== ExampleKey._tag) return;

      const { collectionId, endpointId, exampleId } = key;

      const {
        endpoint: { endpointId: deltaEndpointId },
      } = await dataClient.fetch(EndpointCreateEndpoint, {
        collectionId,
        hidden: true,
      });

      const { exampleId: deltaExampleId } = await dataClient.fetch(ExampleCreateEndpoint, {
        endpointId: deltaEndpointId,
        hidden: true,
      });

      const canvas = ref.current?.getBoundingClientRect() ?? { x: 0, y: 0 };

      const node = await makeNode({
        kind: NodeKind.REQUEST,
        name: `request_${getNodes().length}`,
        position: screenToFlowPosition({ x: x + canvas.x, y: y + canvas.y }),
        request: {
          collectionId,
          deltaEndpointId,
          deltaExampleId,
          endpointId,
          exampleId,
        },
      });

      addNodes(Node.fromDTO(node));
    },
    ref,
  });

  return (
    <ReactFlow
      {...(dropProps as object)}
      colorMode='light'
      connectionLineComponent={ConnectionLine}
      defaultEdgeOptions={{ type: 'EDGE_KIND_UNSPECIFIED' satisfies EdgeKindJson }}
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
      ref={ref}
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
  const { dataClient } = useRouteContext({ from: '__root__' });

  const { flowId } = flowRoute.useLoaderData();
  const { flowIdCan, workspaceIdCan } = flowRoute.useParams();

  const { name } = useQuery(FlowGetEndpoint, { flowId });

  const { zoomIn, zoomOut } = useReactFlow();
  const { zoom } = useViewport();

  const matchRoute = useMatchRoute();

  const onFlowDelete = useOnFlowDelete();

  const [flowUpdate, flowUpdateLoading] = useMutate(FlowUpdateEndpoint);

  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    onSuccess: (_) => flowUpdate({ flowId, name: _ }),
    value: name,
  });

  return (
    <div className={tw`flex items-center gap-2 border-b border-slate-200 bg-white px-3 py-2.5`}>
      {isEditing ? (
        <TextField
          aria-label='Flow name'
          inputClassName={tw`-my-1 py-1 text-md leading-none font-medium tracking-tight text-slate-800`}
          isDisabled={flowUpdateLoading}
          {...textFieldProps}
        />
      ) : (
        <div className={tw`text-md leading-5 font-medium tracking-tight text-slate-800`} onContextMenu={onContextMenu}>
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

      <div className={tw`w-10 text-center text-sm leading-5 font-medium tracking-tight text-gray-900`}>
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
        from='/'
        params={{ flowIdCan, workspaceIdCan }}
        to={
          matchRoute({ to: '/workspace/$workspaceIdCan/flow/$flowIdCan/history' })
            ? '/workspace/$workspaceIdCan/flow/$flowIdCan'
            : '/workspace/$workspaceIdCan/flow/$flowIdCan/history'
        }
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

          <MenuItem
            onAction={async () => {
              await onFlowDelete(flowId);
              await dataClient.fetch(FlowDeleteEndpoint, { flowId });
            }}
            variant='danger'
          >
            Delete
          </MenuItem>
        </Menu>
      </MenuTrigger>
    </div>
  );
};

const ActionBar = () => {
  const { flowId } = use(FlowContext);
  const { dataClient, transport } = useRouteContext({ from: '__root__' });
  const { flowRun } = useMemo(() => createClient(FlowService, transport), [transport]);
  const flow = useReactFlow<Node, Edge>();
  const storeApi = useStoreApi<Node, Edge>();
  const endpointProps = useEndpointProps();

  const [controller, setController] = useState<AbortController>();

  const makeNode = useMakeNode();

  const { proceed, reset, status } = useBlocker({
    disabled: !controller,
    shouldBlockFn: (_) => _.current.pathname !== _.next.pathname,
    withResolver: true,
  });

  const onRun = async () => {
    const controller = new AbortController();
    setController(controller);

    try {
      flow.getNodes().forEach((_) => void flow.updateNodeData(_.id, { ..._.data, state: NodeState.UNSPECIFIED }));
      flow.getEdges().forEach((_) => void flow.updateEdgeData(_.id, { ..._.data, state: NodeState.UNSPECIFIED }));

      // Wait for auto-save
      // TODO: would be better to implement some sort of a locking mechanism
      await new Promise((r) => setTimeout(r, 500));

      const sourceEdges = pipe(
        flow.getEdges(),
        Array.groupBy((_) => _.source),
        Record.toEntries,
        HashMap.fromIterable,
      );

      for await (const { example, node, version } of flowRun({ flowId }, { signal: controller.signal })) {
        if (version) {
          void setQueryChild(
            dataClient.controller,
            FlowVersionsEndpoint.schema.items,
            'unshift',
            { controller: () => dataClient.controller, input: { flowId }, transport },
            version,
          );
        }

        if (example) {
          const { exampleId, responseId, versionId } = example;

          const snapshot = dataClient.controller.snapshot(dataClient.controller.getState());

          const oldExampleData: Example | undefined = snapshot.get(ExampleEntity, { exampleId });

          if (oldExampleData)
            void dataClient.controller.set(
              ExampleEntity,
              { exampleId },
              { ...oldExampleData, lastResponseId: responseId },
            );

          void setQueryChild(
            dataClient.controller,
            ExampleVersionsEndpoint.schema.items,
            'unshift',
            { controller: () => dataClient.controller, input: { exampleId }, transport },
            { exampleId: versionId, lastResponseId: responseId } satisfies Partial<ExampleVersionsItemEntity>,
          );
        }

        if (node) {
          const { info, nodeId, state } = node;
          const nodeIdCan = Ulid.construct(nodeId).toCanonical();

          void dataClient.controller.expireAll({
            testKey: (_) => _ === NodeExecutionListEndpoint.key({ ...endpointProps, input: { nodeId } }),
          });

          flow.updateNodeData(nodeIdCan, (_) => ({ ..._, info: info!, state }));

          pipe(
            HashMap.get(sourceEdges, nodeIdCan),
            Array.fromOption,
            Array.flatten,
            Array.forEach((_) => void flow.updateEdgeData(_.id, (_) => ({ ..._, state }))),
          );
        }
      }
    } finally {
      setController(undefined);
    }
  };

  const onStop = () => {
    controller?.abort();

    flow.getNodes().forEach((_) => {
      if (_.data.state !== NodeState.RUNNING) return;
      flow.updateNodeData(_.id, { ..._.data, state: NodeState.CANCELED });
    });
    flow.getEdges().forEach((_) => {
      if (_.data?.state !== NodeState.RUNNING) return;
      flow.updateEdgeData(_.id, { ..._.data, state: NodeState.CANCELED });
    });
  };

  return (
    <>
      <Modal isOpen={status === 'blocked'} modalClassName={tw`h-auto w-sm`}>
        <Dialog className={tw`grid grid-cols-2 gap-4 p-6`}>
          <div className={tw`col-span-full`}>
            Leaving the flow will stop the execution, are you sure you want to proceed?
          </div>

          <Button onPress={() => reset?.()} variant='secondary'>
            Cancel
          </Button>

          <Button
            onPress={() => {
              onStop();
              proceed?.();
            }}
            variant='primary'
          >
            Continue
          </Button>
        </Dialog>
      </Modal>

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

        {controller ? (
          <Button onPress={onStop} variant='primary'>
            <FiStopCircle className={tw`size-4`} />
            Stop
          </Button>
        ) : (
          <Button onPress={onRun} variant='primary'>
            <PlayCircleIcon className={tw`size-4`} />
            Run
          </Button>
        )}
      </RFPanel>
    </>
  );
};

const SettingsPanel = () => {
  const { dataClient } = useRouteContext({ from: '__root__' });

  const { flowId } = flowRoute.useLoaderData();

  const { items } = useQuery(FlowVariableListEndpoint, { flowId });

  const table = useReactTable({
    columns: [
      columnCheckboxField<FlowVariableListItemEntity>('enabled', { meta: { divider: false } }),
      columnReferenceField<FlowVariableListItemEntity>('name', { meta: { isRowHeader: true } }),
      columnReferenceField<FlowVariableListItemEntity>('value', { allowFiles: true }),
      columnTextField<FlowVariableListItemEntity>('description', { meta: { divider: false } }),
      columnActionsCommon<FlowVariableListItemEntity>({
        onDelete: (_) => dataClient.fetch(FlowVariableDeleteEndpoint, { variableId: _.variableId }),
      }),
    ],
    data: items,
    getRowId: (_) => Ulid.construct(_.variableId).toCanonical(),
  });

  const formTable = useFormTable({
    createLabel: 'New variable',
    items,
    onCreate: () =>
      dataClient.fetch(FlowVariableCreateEndpoint, {
        enabled: true,
        flowId,
        name: `FLOW_VARIABLE_${items.length}`,
      }),
    onUpdate: ({ $typeName: _, ...item }) => dataClient.fetch(FlowVariableUpdateEndpoint, item),
    primaryColumn: 'name',
  });

  const { dragAndDropHooks } = useDragAndDrop({
    getItems: (keys) => [...keys].map((key) => ({ key: key.toString() })),
    onReorder: ({ keys, target: { dropPosition, key } }) =>
      Option.gen(function* () {
        const targetIdCan = yield* Option.liftPredicate(key, Predicate.isString);

        const sourceIdCan = yield* pipe(
          yield* Option.liftPredicate(keys, (_) => _.size === 1),
          Array.fromIterable,
          Array.head,
          Option.filter(Predicate.isString),
        );

        const position = yield* pipe(
          Match.value(dropPosition),
          Match.when('after', () => MovePosition.AFTER),
          Match.when('before', () => MovePosition.BEFORE),
          Match.option,
        );

        void dataClient.fetch(FlowVariableMoveEndpoint, {
          flowId,
          position,
          targetVariableId: Ulid.fromCanonical(targetIdCan).bytes,
          variableId: Ulid.fromCanonical(sourceIdCan).bytes,
        });
      }),
    renderDropIndicator: () => <tr className={tw`relative z-10 col-span-full h-0 w-full ring ring-violet-700`} />,
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
          table={table}
          tableAria-label='Flow variables'
          tableDragAndDropHooks={dragAndDropHooks}
        />
      </div>
    </>
  );
};

export const EditPanel = () => {
  const { workspaceId } = workspaceRoute.useLoaderData();
  const { nodeId } = flowRoute.useLoaderData();

  const { data } = useDLE(NodeGetEndpoint, Option.isSome(nodeId) ? { nodeId: nodeId.value } : null);

  if (Option.isNone(nodeId) || !data) return null;

  const view = pipe(
    Match.value(data),
    Match.when({ kind: NodeKind.NO_OP, noOp: NodeNoOpKind.START }, () => <SettingsPanel />),
    Match.when({ kind: NodeKind.CONDITION }, () => <ConditionPanel node={data} />),
    Match.when({ kind: NodeKind.FOR_EACH }, () => <ForEachPanel node={data} />),
    Match.when({ kind: NodeKind.FOR }, () => <ForPanel node={data} />),
    Match.when({ kind: NodeKind.JS }, () => <JavaScriptPanel node={data} />),
    Match.when({ kind: NodeKind.REQUEST }, () => <RequestPanel node={data} />),
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
          key={Ulid.construct(nodeId.value).toCanonical()}
        >
          {view}
        </Suspense>
      </Panel>
    </ReferenceContext>
  );
};
