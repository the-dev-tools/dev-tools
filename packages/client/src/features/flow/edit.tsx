import { create } from '@bufbuild/protobuf';
import { eq, useLiveQuery } from '@tanstack/react-db';
import { useMatchRoute } from '@tanstack/react-router';
import * as XF from '@xyflow/react';
import { Duration, Match, pipe } from 'effect';
import { Ulid } from 'id128';
import { PropsWithChildren, ReactNode, use, useRef, useState } from 'react';
import { useDrop } from 'react-aria';
import { Button as AriaButton, Dialog, MenuTrigger, useDragAndDrop } from 'react-aria-components';
import { createPortal } from 'react-dom';
import { FiClock, FiMinus, FiMoreHorizontal, FiPlus, FiStopCircle, FiX } from 'react-icons/fi';
import { twJoin } from 'tailwind-merge';
import { FileKind } from '@the-dev-tools/spec/buf/api/file_system/v1/file_system_pb';
import {
  FlowSchema,
  FlowService,
  FlowVariable,
  HandleKind,
  NodeKind,
  NodeSchema,
} from '@the-dev-tools/spec/buf/api/flow/v1/flow_pb';
import { FileCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/file_system';
import {
  EdgeCollectionSchema,
  FlowCollectionSchema,
  FlowVariableCollectionSchema,
  NodeCollectionSchema,
  NodeHttpCollectionSchema,
} from '@the-dev-tools/spec/tanstack-db/v1/api/flow';
import { Button, ButtonAsLink } from '@the-dev-tools/ui/button';
import { DataTable, useReactTable } from '@the-dev-tools/ui/data-table';
import { PlayCircleIcon } from '@the-dev-tools/ui/icons';
import { Menu, MenuItem, useContextMenuState } from '@the-dev-tools/ui/menu';
import { Modal, useProgrammaticModal } from '@the-dev-tools/ui/modal';
import { DropIndicatorHorizontal } from '@the-dev-tools/ui/reorder';
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
import { pick, queryCollection } from '~/utils/tanstack-db';
import { AddNodeSidebar } from './add-node';
import { FlowContext } from './context';
import { ConnectionLine, edgeTypes, useEdgeState } from './edge';
import { useNodesState } from './node';
import { ConditionNode, ConditionSettings } from './nodes/condition';
import { ForNode, ForSettings } from './nodes/for';
import { ForEachNode, ForEachSettings } from './nodes/for-each';
import { HttpNode, HttpSettings } from './nodes/http';
import { JavaScriptNode, JavaScriptSettings } from './nodes/javascript';
import { ManualStartNode } from './nodes/manual-start';
import { useViewport, VIEWPORT_MAX_ZOOM, VIEWPORT_MIN_ZOOM } from './viewport';

export const nodeTypes: XF.NodeTypes = {
  [NodeKind.CONDITION]: ConditionNode,
  [NodeKind.FOR]: ForNode,
  [NodeKind.FOR_EACH]: ForEachNode,
  [NodeKind.HTTP]: HttpNode,
  [NodeKind.JS]: JavaScriptNode,
  [NodeKind.MANUAL_START]: ManualStartNode,
  [NodeKind.UNSPECIFIED]: () => null,
};

export const FlowEditPage = () => {
  const { flowId } = flowLayoutRouteApi.useLoaderData();

  const [sidebar, setSidebar] = useState<ReactNode>(null);

  return (
    <FlowContext.Provider value={{ flowId, setSidebar }}>
      <XF.ReactFlowProvider>
        <div className={tw`flex h-full flex-col`}>
          <TopBarWithControls />
          <Flow key={Ulid.construct(flowId).toCanonical()}>
            <ActionBar />

            {sidebar && (
              <XF.Panel className={tw`inset-y-0 w-80 border-l border-slate-200 bg-white`} position='top-right'>
                {sidebar}
              </XF.Panel>
            )}
          </Flow>
        </div>
      </XF.ReactFlowProvider>
    </FlowContext.Provider>
  );
};

export const Flow = ({ children }: PropsWithChildren) => {
  const fileCollection = useApiCollection(FileCollectionSchema);
  const flowCollection = useApiCollection(FlowCollectionSchema);
  const edgeCollection = useApiCollection(EdgeCollectionSchema);
  const nodeCollection = useApiCollection(NodeCollectionSchema);
  const nodeHttpCollection = useApiCollection(NodeHttpCollectionSchema);

  const nodeEditDialog = useNodeEditDialog();

  const { getNodes, screenToFlowPosition } = XF.useReactFlow();

  const { flowId, isReadOnly = false, setSidebar } = use(FlowContext);

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

  const { nodes, onNodesChange } = useNodesState();
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

  const onConnectEnd: XF.OnConnectEnd = (event, { fromHandle, fromNode, isValid }) => {
    if (!(event instanceof MouseEvent)) return;
    if (isValid) return;
    if (fromNode === null) return;

    const sourceId = Ulid.fromCanonical(fromNode.id).bytes;
    const position = screenToFlowPosition({ x: event.clientX, y: event.clientY });
    const handleKind = !fromHandle?.id ? HandleKind.UNSPECIFIED : parseInt(fromHandle.id);

    setSidebar?.(<AddNodeSidebar handleKind={handleKind} position={position} sourceId={sourceId} />);
  };

  const { dropProps } = useDrop({
    onDrop: async ({ items, x, y }) => {
      const [item] = items;
      if (item?.kind !== 'text' || !item.types.has('key') || items.length !== 1) return;

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
          name: `http_${getNodes().length}`,
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
          name: `http_${getNodes().length}`,
          nodeId,
          position,
        });
      }
    },
    ref,
  });

  const statusBarEndSlot = document.getElementById('statusBarEndSlot');

  return (
    <>
      {statusBarEndSlot &&
        createPortal(
          <div className={tw`flex gap-4 text-xs leading-none text-slate-800`}>
            <NodeSelectionIndicator />
            {duration && <div>Time: {pipe(duration, Duration.millis, Duration.format)}</div>}
          </div>,
          statusBarEndSlot,
        )}

      {nodeEditDialog.render}

      <XF.ReactFlow
        {...(dropProps as object)}
        colorMode='light'
        connectionLineComponent={ConnectionLine}
        deleteKeyCode={['Backspace', 'Delete']}
        edges={edges}
        edgeTypes={edgeTypes}
        maxZoom={VIEWPORT_MAX_ZOOM}
        minZoom={VIEWPORT_MIN_ZOOM}
        nodes={nodes}
        nodesConnectable={!isReadOnly}
        nodesDraggable
        nodeTypes={nodeTypes}
        onConnect={onConnect}
        onConnectEnd={onConnectEnd}
        onEdgesChange={onEdgesChange}
        onNodeDoubleClick={(_, node) => {
          const nodeId = Ulid.fromCanonical(node.id);
          void nodeEditDialog.open(nodeId.bytes);
        }}
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
    </>
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
  const { flowId, setSidebar } = use(FlowContext);
  const { transport } = rootRouteApi.useRouteContext();

  const flowCollection = useApiCollection(FlowCollectionSchema);

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
      <Button className={tw`px-1.5 py-1`} onPress={() => void setSidebar?.(<AddNodeSidebar />)} variant='ghost dark'>
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

const FlowSettings = () => {
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

        <Button className={tw`p-1`} slot='close' variant='ghost'>
          <FiX className={tw`size-5 text-slate-500`} />
        </Button>
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

const useNodeEditDialog = () => {
  const { workspaceId } = workspaceRouteApi.useLoaderData();

  const nodeCollection = useApiCollection(NodeCollectionSchema);

  const modal = useProgrammaticModal();

  const open = async (nodeId: Uint8Array) => {
    const [{ kind } = create(NodeSchema)] = await queryCollection((_) =>
      _.from({ item: nodeCollection })
        .where((_) => eq(_.item.nodeId, nodeId))
        .select((_) => pick(_.item, 'kind'))
        .findOne(),
    );

    const view = pipe(
      Match.value({ kind }),
      Match.when({ kind: NodeKind.MANUAL_START }, () => <FlowSettings />),
      Match.when({ kind: NodeKind.CONDITION }, () => <ConditionSettings nodeId={nodeId} />),
      Match.when({ kind: NodeKind.FOR_EACH }, () => <ForEachSettings nodeId={nodeId} />),
      Match.when({ kind: NodeKind.FOR }, (_) => <ForSettings nodeId={nodeId} />),
      Match.when({ kind: NodeKind.JS }, (_) => <JavaScriptSettings nodeId={nodeId} />),
      Match.when({ kind: NodeKind.HTTP }, (_) => <HttpSettings nodeId={nodeId} />),
      Match.orElse(() => null),
    );

    if (!view) return;

    modal.onOpenChange(true, <ReferenceContext value={{ flowNodeId: nodeId, workspaceId }}>{view}</ReferenceContext>);
  };

  const render: ReactNode = modal.children && (
    <Modal {...modal} className={tw`max-h-[85vh] max-w-[90vw]`}>
      <Dialog aria-label='Node settings' className={tw`flex h-full flex-col overflow-auto outline-hidden`}>
        {modal.children}
      </Dialog>
    </Modal>
  );

  return { open, render };
};
