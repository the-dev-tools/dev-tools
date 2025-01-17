import { create, Message, MessageInitShape } from '@bufbuild/protobuf';
import { createClient } from '@connectrpc/connect';
import { createQueryOptions } from '@connectrpc/connect-query';
import { useSuspenseQueries } from '@tanstack/react-query';
import { createFileRoute, getRouteApi, redirect, ToOptions } from '@tanstack/react-router';
import {
  Background,
  BackgroundVariant,
  ConnectionLineComponentProps,
  getConnectedEdges,
  getSmoothStepPath,
  Position,
  ReactFlow,
  ReactFlowProps,
  ReactFlowProvider,
  Handle as RFBaseHandle,
  NodeProps as RFBaseNodeProps,
  Edge as RFEdge,
  EdgeProps as RFEdgeProps,
  HandleProps as RFHandleProps,
  Node as RFNode,
  NodeTypes as RFNodeTypes,
  Panel as RFPanel,
  useEdgesState,
  useNodesState,
  useReactFlow,
  useViewport,
} from '@xyflow/react';
import { Array, Match, pipe, Schema, Struct } from 'effect';
import { Ulid } from 'id128';
import { ComponentProps, ReactNode, useCallback, useEffect, useMemo } from 'react';
import { Header, ListBoxSection, MenuTrigger } from 'react-aria-components';
import { Controller, useForm } from 'react-hook-form';
import { IconType } from 'react-icons';
import { FiExternalLink, FiMinus, FiMoreHorizontal, FiPlus, FiTerminal, FiX } from 'react-icons/fi';
import { Panel } from 'react-resizable-panels';
import { useDebouncedCallback } from 'use-debounce';

import { useConnectMutation, useConnectQuery } from '@the-dev-tools/api/connect-query';
import { endpointGet } from '@the-dev-tools/spec/collection/item/endpoint/v1/endpoint-EndpointService_connectquery';
import {
  exampleCreate,
  exampleGet,
} from '@the-dev-tools/spec/collection/item/example/v1/example-ExampleService_connectquery';
import { collectionGet } from '@the-dev-tools/spec/collection/v1/collection-CollectionService_connectquery';
import { Edge, EdgeListItem, EdgeSchema, Handle } from '@the-dev-tools/spec/flow/edge/v1/edge_pb';
import { edgeCreate, edgeDelete, edgeList } from '@the-dev-tools/spec/flow/edge/v1/edge-EdgeService_connectquery';
import {
  ErrorHandling,
  Node,
  NodeGetResponse,
  NodeKind,
  NodeListItem,
  NodeNoOpKind,
  NodeRequestSchema,
  NodeSchema,
} from '@the-dev-tools/spec/flow/node/v1/node_pb';
import {
  nodeCreate,
  nodeDelete,
  nodeGet,
  nodeList,
  nodeUpdate,
} from '@the-dev-tools/spec/flow/node/v1/node-NodeService_connectquery';
import { FlowGetResponse, FlowService } from '@the-dev-tools/spec/flow/v1/flow_pb';
import { flowGet } from '@the-dev-tools/spec/flow/v1/flow-FlowService_connectquery';
import { Button, ButtonAsLink } from '@the-dev-tools/ui/button';
import { FieldLabel } from '@the-dev-tools/ui/field';
import {
  ChatAddIcon,
  CheckListAltIcon,
  CollectIcon,
  DataSourceIcon,
  DelayIcon,
  ForIcon,
  IfIcon,
  PlayCircleIcon,
  PlayIcon,
  SendRequestIcon,
  TextBoxIcon,
} from '@the-dev-tools/ui/icons';
import { ListBox, ListBoxItem, ListBoxItemProps } from '@the-dev-tools/ui/list-box';
import { Menu, MenuItem } from '@the-dev-tools/ui/menu';
import { MethodBadge } from '@the-dev-tools/ui/method-badge';
import { NumberFieldRHF } from '@the-dev-tools/ui/number-field';
import { PanelResizeHandle } from '@the-dev-tools/ui/resizable-panel';
import { SelectRHF } from '@the-dev-tools/ui/select';
import { Separator } from '@the-dev-tools/ui/separator';
import { tw } from '@the-dev-tools/ui/tailwind-literal';

import { CollectionListTree } from './collection';
import { ConditionField } from './condition';
import { EndpointRequestView, EndpointRouteSearch, ResponsePanel, useEndpointUrl } from './endpoint';
import { ReferenceContext, ReferenceField } from './reference';

class Search extends EndpointRouteSearch.extend<Search>('FlowRouteSearch')({
  selectedNodeIdCan: pipe(Schema.String, Schema.optional),
}) {}

export const Route = createFileRoute('/_authorized/workspace/$workspaceIdCan/flow/$flowIdCan')({
  component: RouteComponent,
  pendingComponent: () => 'Loading flow...',
  validateSearch: (_) => Schema.decodeSync(Search)(_),
  loader: async ({ params: { flowIdCan }, context: { transport, queryClient } }) => {
    const flowId = Ulid.fromCanonical(flowIdCan).bytes;

    try {
      await Promise.all([
        queryClient.ensureQueryData(createQueryOptions(flowGet, { flowId }, { transport })),
        queryClient.ensureQueryData(createQueryOptions(nodeList, { flowId }, { transport })),
        queryClient.ensureQueryData(createQueryOptions(edgeList, { flowId }, { transport })),
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

const workspaceRoute = getRouteApi('/_authorized/workspace/$workspaceIdCan');

function RouteComponent() {
  const { flowId } = Route.useLoaderData();
  const { selectedNodeIdCan } = Route.useSearch();

  const { workspaceId } = workspaceRoute.useLoaderData();

  const nodeId = selectedNodeIdCan ? Ulid.fromCanonical(selectedNodeIdCan).bytes : undefined!;

  const flowQuery = useConnectQuery(flowGet, { flowId });
  const edgeListQuery = useConnectQuery(edgeList, { flowId });
  const nodeListQuery = useConnectQuery(nodeList, { flowId });
  const selectedNodeQuery = useConnectQuery(nodeGet, { nodeId }, { enabled: selectedNodeIdCan !== undefined });

  if (!flowQuery.data || !edgeListQuery.data || !nodeListQuery.data) return null;

  return (
    <ReactFlowProvider>
      <Panel id='request' order={1} className='flex h-full flex-col'>
        <FlowView flow={flowQuery.data} edges={edgeListQuery.data.items} nodes={nodeListQuery.data.items} />
      </Panel>
      {selectedNodeQuery.data && (
        <ReferenceContext value={{ nodeId, workspaceId }}>
          <PanelResizeHandle direction='vertical' />
          <Panel id='response' order={2} defaultSize={40} className={tw`!overflow-auto`}>
            <EditPanel node={selectedNodeQuery.data} />
          </Panel>
        </ReferenceContext>
      )}
    </ReactFlowProvider>
  );
}

const mapEdgeToClient = (edge: Omit<EdgeListItem, keyof Message>) =>
  ({
    id: Ulid.construct(edge.edgeId).toCanonical(),
    source: Ulid.construct(edge.sourceId).toCanonical(),
    sourceHandle: edge.sourceHandle === Handle.UNSPECIFIED ? null : edge.sourceHandle.toString(),
    target: Ulid.construct(edge.targetId).toCanonical(),
  }) satisfies RFEdge;

const mapNodeToClient = (node: Omit<NodeListItem, keyof Message>) =>
  ({
    id: Ulid.construct(node.nodeId).toCanonical(),
    position: Struct.pick(node.position!, 'x', 'y'),
    origin: [0.5, 0],
    type: node.kind.toString(),
    data: node,
  }) satisfies Partial<RFNode>;

interface RFNodeProps extends RFBaseNodeProps<RFNode<Node>> {}

const NoOpNodeView = (props: RFNodeProps) => {
  const kind = props.data.noOp;

  if (kind === NodeNoOpKind.CREATE) return <CreateNodeView {...props} />;

  return (
    <>
      <div className={tw`flex items-center gap-2 rounded-md bg-slate-800 px-4 text-white shadow-sm`}>
        {kind === NodeNoOpKind.START && (
          <>
            <PlayIcon className={tw`-ml-2 size-4`} />
            <div className={tw`w-px self-stretch bg-slate-700`} />
          </>
        )}

        <span className={tw`flex-1 py-1 text-xs font-medium leading-5`}>
          {pipe(
            Match.value(kind),
            Match.when(NodeNoOpKind.START, () => 'Manual start'),
            Match.when(NodeNoOpKind.THEN, () => 'Then'),
            Match.when(NodeNoOpKind.ELSE, () => 'Else'),
            Match.when(NodeNoOpKind.LOOP, () => 'Loop'),
            Match.orElseAbsurd,
          )}
        </span>
      </div>

      {kind !== NodeNoOpKind.START && <RFHandle type='target' position={Position.Top} isConnectable={false} />}
      <RFHandle type='source' position={Position.Bottom} />
    </>
  );
};

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

const useMakeNode = () => {
  const { flowId } = Route.useLoaderData();
  const nodeCreateMutation = useConnectMutation(nodeCreate);
  return useCallback(
    async (data: Omit<MessageInitShape<typeof NodeSchema>, keyof Message>) => {
      const { nodeId } = await nodeCreateMutation.mutateAsync({ flowId, ...data });
      return create(NodeSchema, { nodeId, ...data });
    },
    [flowId, nodeCreateMutation],
  );
};

const useMakeEdge = () => {
  const { flowId } = Route.useLoaderData();
  const edgeCreateMutation = useConnectMutation(edgeCreate);
  return useCallback(
    async (data: Omit<MessageInitShape<typeof EdgeSchema>, keyof Message>) => {
      const { edgeId } = await edgeCreateMutation.mutateAsync({ flowId, ...data });
      return create(EdgeSchema, { edgeId, ...data });
    },
    [edgeCreateMutation, flowId],
  );
};

const CreateNodeView = ({ id }: RFNodeProps) => {
  const { getNode, getEdges, addNodes, addEdges, deleteElements } = useReactFlow();

  const edge = useMemo(() => getEdges().find((_) => _.target === id)!, [getEdges, id]);
  const sourceId = useMemo(() => Ulid.fromCanonical(edge.source).bytes, [edge.source]);

  const add = useCallback(
    async (nodes: Node[], edges: Edge[]) => {
      await deleteElements({ nodes: [{ id }], edges: [edge] });

      addNodes(nodes.map(mapNodeToClient));
      addEdges(edges.map(mapEdgeToClient));
    },
    [addEdges, addNodes, deleteElements, edge, id],
  );

  const makeNode = useMakeNode();
  const makeEdge = useMakeEdge();

  const position = useMemo(() => getNode(id)!.position, [getNode, id]);

  const { x, y } = position;
  const offset = 200;

  return (
    <>
      <ListBox
        aria-label='Create node type'
        onAction={() => void {}}
        className={tw`w-80 divide-y divide-slate-200 pt-0`}
      >
        <ListBoxSection>
          <CreateNodeHeader>Task</CreateNodeHeader>

          <CreateNodeItem
            id='request'
            Icon={SendRequestIcon}
            title='Send Request'
            description='Send request from your collection'
            onAction={async () => {
              const node = await makeNode({ kind: NodeKind.REQUEST, request: {}, position });
              const edge = await makeEdge({ sourceId, targetId: node.nodeId });
              await add([node], [edge]);
            }}
          />

          <CreateNodeItem
            id='data'
            Icon={DataSourceIcon}
            title='Data Source'
            description='Import data from .xlsx file'
          />

          <CreateNodeItem id='delay' Icon={DelayIcon} title='Delay' description='Wait specific time' />

          <CreateNodeItem id='javascript' Icon={FiTerminal} title='JavaScript' description='Custom Javascript block' />
        </ListBoxSection>

        <ListBoxSection>
          <CreateNodeHeader>Logic</CreateNodeHeader>

          <CreateNodeItem
            id='condition'
            Icon={IfIcon}
            title='If'
            description='Add true/false'
            onAction={async () => {
              const [node, nodeThen, nodeElse] = await Promise.all([
                makeNode({ kind: NodeKind.CONDITION, condition: {}, position }),
                makeNode({
                  kind: NodeKind.NO_OP,
                  noOp: NodeNoOpKind.THEN,
                  position: { x: x - offset, y: y + offset },
                }),
                makeNode({
                  kind: NodeKind.NO_OP,
                  noOp: NodeNoOpKind.ELSE,
                  position: { x: x + offset, y: y + offset },
                }),
              ]);

              const edges = await Promise.all([
                makeEdge({ sourceId, targetId: node.nodeId }),
                makeEdge({
                  sourceId: node.nodeId,
                  sourceHandle: Handle.THEN,
                  targetId: nodeThen.nodeId,
                }),
                makeEdge({
                  sourceId: node.nodeId,
                  sourceHandle: Handle.ELSE,
                  targetId: nodeElse.nodeId,
                }),
              ]);

              await add([node, nodeThen, nodeElse], edges);
            }}
          />
        </ListBoxSection>

        <ListBoxSection>
          <CreateNodeHeader>Looping</CreateNodeHeader>

          <CreateNodeItem id='collect' Icon={CollectIcon} title='Collect' description='Collect all result' />

          <CreateNodeItem
            id='for'
            Icon={ForIcon}
            title='For Loop'
            description='Loop'
            onAction={async () => {
              const [node, nodeLoop, nodeThen] = await Promise.all([
                makeNode({ kind: NodeKind.FOR, for: {}, position }),
                makeNode({
                  kind: NodeKind.NO_OP,
                  noOp: NodeNoOpKind.LOOP,
                  position: { x: x - offset, y: y + offset },
                }),
                makeNode({
                  kind: NodeKind.NO_OP,
                  noOp: NodeNoOpKind.THEN,
                  position: { x: x + offset, y: y + offset },
                }),
              ]);

              const edges = await Promise.all([
                makeEdge({ sourceId, targetId: node.nodeId }),
                makeEdge({
                  sourceId: node.nodeId,
                  sourceHandle: Handle.LOOP,
                  targetId: nodeLoop.nodeId,
                }),
                makeEdge({
                  sourceId: node.nodeId,
                  sourceHandle: Handle.THEN,
                  targetId: nodeThen.nodeId,
                }),
              ]);

              await add([node, nodeLoop, nodeThen], edges);
            }}
          />

          <CreateNodeItem
            id='foreach'
            Icon={ForIcon}
            title='For Each Loop'
            description='Loop'
            onAction={async () => {
              const [node, nodeLoop, nodeThen] = await Promise.all([
                makeNode({ kind: NodeKind.FOR_EACH, forEach: {}, position }),
                makeNode({
                  kind: NodeKind.NO_OP,
                  noOp: NodeNoOpKind.LOOP,
                  position: { x: x - offset, y: y + offset },
                }),
                makeNode({
                  kind: NodeKind.NO_OP,
                  noOp: NodeNoOpKind.THEN,
                  position: { x: x + offset, y: y + offset },
                }),
              ]);

              const edges = await Promise.all([
                makeEdge({ sourceId, targetId: node.nodeId }),
                makeEdge({
                  sourceId: node.nodeId,
                  sourceHandle: Handle.LOOP,
                  targetId: nodeLoop.nodeId,
                }),
                makeEdge({
                  sourceId: node.nodeId,
                  sourceHandle: Handle.THEN,
                  targetId: nodeThen.nodeId,
                }),
              ]);

              await add([node, nodeLoop, nodeThen], edges);
            }}
          />
        </ListBoxSection>
      </ListBox>

      <RFHandle type='target' position={Position.Top} isConnectable={false} />
    </>
  );
};

interface BaseNodeViewProps {
  id: string;
  nodeId: Uint8Array;
  Icon: IconType;
  title: string;
  children: ReactNode;
}

const BaseNodeView = ({ id, nodeId, Icon, title, children }: BaseNodeViewProps) => {
  const nodeIdCan = Ulid.construct(nodeId).toCanonical();

  const { getEdges, getNode, deleteElements } = useReactFlow();

  const nodeDeleteMutation = useConnectMutation(nodeDelete);
  const edgeDeleteMutation = useConnectMutation(edgeDelete);

  return (
    <div className={tw`w-80 rounded-lg border border-slate-400 bg-slate-200 p-1 shadow-sm`}>
      <div className={tw`flex items-center gap-3 px-1 pb-1.5 pt-0.5`}>
        <Icon className={tw`size-5 text-slate-500`} />

        <div className={tw`h-4 w-px bg-slate-300`} />

        <span className={tw`flex-1 text-xs font-medium leading-5 tracking-tight`}>{title}</span>

        <MenuTrigger>
          <Button variant='ghost' className={tw`p-0.5`}>
            <FiMoreHorizontal className={tw`size-4 text-slate-500`} />
          </Button>

          <Menu>
            <MenuItem
              href={{
                to: '.',
                search: { selectedNodeIdCan: nodeIdCan } satisfies ToOptions['search'],
              }}
            >
              Edit
            </MenuItem>
            <MenuItem>Rename</MenuItem>
            <MenuItem>Duplicate</MenuItem>
            <MenuItem
              variant='danger'
              onAction={async () => {
                const node = getNode(id);

                const { createEdges = [], edges = [] } = pipe(
                  getConnectedEdges([node!], getEdges()),
                  Array.groupBy((_) => (_.id.startsWith('create-') ? 'createEdges' : 'edges')),
                );

                await nodeDeleteMutation.mutateAsync({ nodeId });

                await Promise.allSettled(
                  edges.map(({ id }) =>
                    edgeDeleteMutation.mutateAsync({
                      edgeId: Ulid.fromCanonicalTrusted(id).bytes,
                    }),
                  ),
                );

                await deleteElements({
                  nodes: [{ id }, ...createEdges.map((_) => ({ id: _.target }))],
                  edges: [...createEdges, ...edges],
                });
              }}
            >
              Delete
            </MenuItem>
          </Menu>
        </MenuTrigger>
      </div>

      {children}
    </div>
  );
};

const RequestNodeView = ({ id, data }: RFNodeProps) => {
  const { nodeId } = data;
  const { collectionId, endpointId, exampleId } = data.request!;

  const { updateNodeData } = useReactFlow();

  const collectionGetQuery = useConnectQuery(collectionGet, { collectionId }, { enabled: collectionId.length > 0 });
  const endpointGetQuery = useConnectQuery(endpointGet, { endpointId }, { enabled: endpointId.length > 0 });
  const exampleGetQuery = useConnectQuery(exampleGet, { exampleId }, { enabled: exampleId.length > 0 });

  const exampleCreateMutation = useConnectMutation(exampleCreate);
  const nodeUpdateMutation = useConnectMutation(nodeUpdate);

  let content;
  if (collectionGetQuery.isSuccess && endpointGetQuery.isSuccess && exampleGetQuery.isSuccess) {
    const { name: collectionName } = collectionGetQuery.data;
    const { method } = endpointGetQuery.data;
    const { name } = exampleGetQuery.data;

    content = (
      <div className={tw`space-y-1.5 p-2`}>
        <div className={tw`text-xs leading-4 tracking-tight text-slate-400`}>{collectionName}</div>
        <div className={tw`flex items-center gap-1.5`}>
          <MethodBadge method={method} />
          <div className={tw`flex-1 text-xs font-medium leading-5 tracking-tight text-slate-800`}>{name}</div>
          <ButtonAsLink
            variant='ghost'
            className={tw`p-0.5`}
            href={{
              from: Route.fullPath,
              to: '/workspace/$workspaceIdCan/endpoint/$endpointIdCan/example/$exampleIdCan',
              params: {
                endpointIdCan: Ulid.construct(endpointId).toCanonical(),
                exampleIdCan: Ulid.construct(exampleId).toCanonical(),
              },
            }}
          >
            <FiExternalLink className={tw`size-4 text-slate-500`} />
          </ButtonAsLink>
        </div>
      </div>
    );
  } else {
    content = (
      <CollectionListTree
        onAction={async ({ collectionId, endpointId, exampleId }) => {
          if (collectionId === undefined || endpointId === undefined || exampleId === undefined) return;
          const { exampleId: deltaExampleId } = await exampleCreateMutation.mutateAsync({ endpointId });
          const request = create(NodeRequestSchema, {
            ...data.request!,
            collectionId,
            endpointId,
            exampleId,
            deltaExampleId,
          });
          await nodeUpdateMutation.mutateAsync({ nodeId, request });
          updateNodeData(id, { ...data, request });
        }}
      />
    );
  }

  return (
    <>
      <BaseNodeView id={id} nodeId={nodeId} Icon={SendRequestIcon} title='Send Request'>
        <div className={tw`rounded-md border border-slate-200 bg-white shadow-sm`}>{content}</div>
      </BaseNodeView>

      <RFHandle type='target' position={Position.Top} />
      <RFHandle type='source' position={Position.Bottom} />
    </>
  );
};

const ConditionNodeView = ({ id, data }: RFNodeProps) => {
  const { nodeId } = data;
  const { condition } = data.condition!;

  const nodeIdCan = Ulid.construct(nodeId).toCanonical();

  return (
    <>
      <BaseNodeView id={id} nodeId={nodeId} Icon={IfIcon} title='If'>
        <div className={tw`rounded-md border border-slate-200 bg-white shadow-sm`}>
          {condition ? (
            <div
              className={tw`flex justify-start gap-2 rounded-md border border-slate-200 p-3 text-xs font-medium leading-5 tracking-tight text-slate-800 shadow-sm`}
            >
              <CheckListAltIcon className={tw`size-5 text-slate-500`} />
              <span>Edit Condition</span>
            </div>
          ) : (
            <ButtonAsLink
              className={tw`flex w-full justify-start gap-1.5 rounded-md border border-slate-200 px-2 py-3 text-xs font-medium leading-4 tracking-tight text-violet-600 shadow-sm`}
              href={{
                to: '.',
                search: { selectedNodeIdCan: nodeIdCan } satisfies ToOptions['search'],
              }}
            >
              <FiPlus className={tw`size-4`} />
              <span>Setup Condition</span>
            </ButtonAsLink>
          )}
        </div>
      </BaseNodeView>

      <RFHandle type='target' position={Position.Top} />
      <RFHandle type='source' position={Position.Bottom} id={Handle.THEN.toString()} isConnectable={false} />
      <RFHandle type='source' position={Position.Bottom} id={Handle.ELSE.toString()} isConnectable={false} />
    </>
  );
};

const ForNodeView = ({ id, data: { nodeId } }: RFNodeProps) => {
  const nodeIdCan = Ulid.construct(nodeId).toCanonical();

  return (
    <>
      <BaseNodeView id={id} nodeId={nodeId} Icon={ForIcon} title='For Loop'>
        <div className={tw`rounded-md border border-slate-200 bg-white shadow-sm`}>
          <ButtonAsLink
            className={tw`flex w-full justify-start gap-1.5 rounded-md border border-slate-200 px-2 py-3 text-xs font-medium leading-4 tracking-tight text-slate-800 shadow-sm`}
            href={{
              to: '.',
              search: { selectedNodeIdCan: nodeIdCan } satisfies ToOptions['search'],
            }}
          >
            <CheckListAltIcon className={tw`size-5 text-slate-500`} />
            <span>Edit Loop</span>
          </ButtonAsLink>
        </div>
      </BaseNodeView>

      <RFHandle type='target' position={Position.Top} />
      <RFHandle type='source' position={Position.Bottom} id={Handle.LOOP.toString()} isConnectable={false} />
      <RFHandle type='source' position={Position.Bottom} id={Handle.THEN.toString()} isConnectable={false} />
    </>
  );
};

const ForEachNodeView = ({ id, data: { nodeId } }: RFNodeProps) => {
  const nodeIdCan = Ulid.construct(nodeId).toCanonical();

  return (
    <>
      <BaseNodeView id={id} nodeId={nodeId} Icon={ForIcon} title='For Each Loop'>
        <div className={tw`rounded-md border border-slate-200 bg-white shadow-sm`}>
          <ButtonAsLink
            className={tw`flex w-full justify-start gap-1.5 rounded-md border border-slate-200 px-2 py-3 text-xs font-medium leading-4 tracking-tight text-slate-800 shadow-sm`}
            href={{
              to: '.',
              search: { selectedNodeIdCan: nodeIdCan } satisfies ToOptions['search'],
            }}
          >
            <CheckListAltIcon className={tw`size-5 text-slate-500`} />
            <span>Edit Loop</span>
          </ButtonAsLink>
        </div>
      </BaseNodeView>

      <RFHandle type='target' position={Position.Top} />
      <RFHandle type='source' position={Position.Bottom} id={Handle.LOOP.toString()} isConnectable={false} />
      <RFHandle type='source' position={Position.Bottom} id={Handle.THEN.toString()} isConnectable={false} />
    </>
  );
};

const nodeTypes: RFNodeTypes = {
  [NodeKind.NO_OP.toString()]: NoOpNodeView,
  [NodeKind.REQUEST.toString()]: RequestNodeView,
  [NodeKind.CONDITION.toString()]: ConditionNodeView,
  [NodeKind.FOR.toString()]: ForNodeView,
  [NodeKind.FOR_EACH.toString()]: ForEachNodeView,
};

const EdgeView = ({ sourceX, sourceY, sourcePosition, targetX, targetY, targetPosition }: RFEdgeProps) => (
  <ConnectionLine
    fromX={sourceX}
    fromY={sourceY}
    fromPosition={sourcePosition}
    toX={targetX}
    toY={targetY}
    toPosition={targetPosition}
    connected
  />
);

const edgeTypes = {
  default: EdgeView,
};

interface ConnectionLineProps
  extends Pick<ConnectionLineComponentProps, 'fromX' | 'fromY' | 'fromPosition' | 'toX' | 'toY' | 'toPosition'> {
  connected?: boolean;
}

const ConnectionLine = ({
  fromX,
  fromY,
  fromPosition,
  toX,
  toY,
  toPosition,
  connected = false,
}: ConnectionLineProps) => {
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

  return (
    <path
      className={tw`fill-none stroke-slate-800 stroke-1`}
      d={edgePath}
      strokeDasharray={connected ? undefined : 4}
    />
  );
};

const RFHandle = (props: RFHandleProps) => (
  <RFBaseHandle
    className={tw`-z-10 flex size-5 items-center justify-center rounded-full border border-slate-300 bg-slate-200 shadow-sm`}
    {...props}
  >
    <div className={tw`pointer-events-none size-2 rounded-full bg-slate-800`} />
  </RFBaseHandle>
);

const minZoom = 0.5;
const maxZoom = 2;

interface TopBarProps {
  flow: FlowGetResponse;
}

const TopBar = ({ flow }: TopBarProps) => {
  const { zoomIn, zoomOut } = useReactFlow();
  const { zoom } = useViewport();

  return (
    <RFPanel className={tw`m-0 flex w-full items-center gap-2 border-b border-slate-200 bg-white px-3 py-3.5`}>
      <div className={tw`text-md font-medium leading-5 tracking-tight text-slate-800`}>{flow.name}</div>

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

      <MenuTrigger>
        <Button variant='ghost' className={tw`bg-slate-200 p-0.5`}>
          <FiMoreHorizontal className={tw`size-4 text-slate-500`} />
        </Button>

        <Menu>
          <MenuItem>Rename</MenuItem>
          <Separator />
          <MenuItem variant='danger'>Delete</MenuItem>
        </Menu>
      </MenuTrigger>
    </RFPanel>
  );
};

const ActionBar = () => {
  const { flowId } = Route.useLoaderData();
  const { transport } = Route.useRouteContext();
  const { flowRun } = useMemo(() => createClient(FlowService, transport), [transport]);

  return (
    <RFPanel className={tw`mb-4 flex items-center gap-2 rounded-lg bg-slate-900 p-1 shadow`} position='bottom-center'>
      <Button variant='ghost dark' className={tw`p-1`}>
        <TextBoxIcon className={tw`size-5 text-slate-300`} />
      </Button>

      <Button variant='ghost dark' className={tw`p-1`}>
        <ChatAddIcon className={tw`size-5 text-slate-300`} />
      </Button>

      <div className={tw`mx-2 h-5 w-px bg-white/20`} />

      <Button variant='ghost dark' className={tw`px-1.5 py-1`}>
        <FiPlus className={tw`size-5 text-slate-300`} />
        Add Node
      </Button>

      <Button variant='primary' onPress={() => void flowRun({ flowId })}>
        <PlayCircleIcon className={tw`size-4`} />
        Run
      </Button>
    </RFPanel>
  );
};

interface FlowViewProps {
  flow: FlowGetResponse;
  edges: EdgeListItem[];
  nodes: NodeListItem[];
}

const FlowView = ({ flow, edges: serverEdges, nodes: serverNodes }: FlowViewProps) => {
  const { addNodes, addEdges, screenToFlowPosition } = useReactFlow();

  const [edges, _setEdges, onEdgesChange] = pipe(serverEdges, Array.map(mapEdgeToClient), useEdgesState);
  const [nodes, _setNodes, onNodesChange] = pipe(serverNodes, Array.map(mapNodeToClient), useNodesState);

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

      addNodes(mapNodeToClient(node));
      addEdges(mapEdgeToClient(edge));
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

interface EditPanelProps {
  node: NodeGetResponse;
}

const EditPanel = ({ node }: EditPanelProps) =>
  pipe(
    Match.value(node.kind),
    Match.when(NodeKind.REQUEST, () => <EditRequestNodeView node={node} />),
    Match.when(NodeKind.CONDITION, () => <EditConditionNodeView node={node} />),
    Match.when(NodeKind.FOR, () => <EditForNodeView node={node} />),
    Match.when(NodeKind.FOR_EACH, () => <EditForEachNodeView node={node} />),
    Match.orElseAbsurd,
  );

const EditRequestNodeView = ({ node: { nodeId, request } }: EditPanelProps) => {
  const { collectionId, endpointId, exampleId, deltaExampleId } = request!;

  const { requestTab, responseTab } = Route.useSearch();
  const { transport } = Route.useRouteContext();

  const { workspaceId } = workspaceRoute.useLoaderData();

  const [{ data: collection }, { data: endpoint }, { data: example }] = useSuspenseQueries({
    queries: [
      createQueryOptions(collectionGet, { collectionId }, { transport }),
      createQueryOptions(endpointGet, { endpointId }, { transport }),
      createQueryOptions(exampleGet, { exampleId }, { transport }),
    ],
  });

  const url = useEndpointUrl({ endpointId, exampleId });

  const { lastResponseId } = example;

  return (
    <>
      <div className={tw`sticky top-0 z-10 flex items-center border-b border-slate-200 bg-white px-5 py-2`}>
        <div>
          <div className={tw`text-md leading-5 text-slate-400`}>{collection.name}</div>
          <div className={tw`text-sm font-medium leading-5 text-slate-800`}>{example.name}</div>
        </div>

        <div className={tw`flex-1`} />

        <ButtonAsLink
          variant='ghost'
          className={tw`px-2`}
          href={{
            from: Route.fullPath,
            to: '/workspace/$workspaceIdCan/endpoint/$endpointIdCan/example/$exampleIdCan',
            params: {
              endpointIdCan: Ulid.construct(endpointId).toCanonical(),
              exampleIdCan: Ulid.construct(exampleId).toCanonical(),
            },
          }}
        >
          <FiExternalLink className={tw`size-4 text-slate-500`} />
          Open API
        </ButtonAsLink>

        <div className={tw`ml-2 mr-3 h-5 w-px bg-slate-300`} />

        <ButtonAsLink variant='ghost' className={tw`p-1`} href={{ from: Route.fullPath }}>
          <FiX className={tw`size-5 text-slate-500`} />
        </ButtonAsLink>
      </div>

      <div className='m-5 mb-4 flex flex-1 items-center gap-3 rounded-lg border border-slate-300 px-3 py-2 shadow-sm'>
        <MethodBadge method={endpoint.method} size='lg' />
        <div className={tw`h-7 w-px bg-slate-200`} />
        <div className={tw`truncate font-medium leading-5 tracking-tight text-slate-800`}>{url}</div>
      </div>

      <div className={tw`mx-5 overflow-auto rounded-lg border border-slate-200`}>
        <div
          className={tw`border-b border-slate-200 bg-slate-50 px-3 py-2 text-md font-medium leading-5 tracking-tight text-slate-800`}
        >
          Request
        </div>

        <ReferenceContext value={{ nodeId, exampleId, workspaceId }}>
          <EndpointRequestView
            className={tw`p-5 pt-3`}
            endpointId={endpointId}
            exampleId={exampleId}
            deltaExampleId={deltaExampleId}
            requestTab={requestTab}
          />
        </ReferenceContext>
      </div>

      {lastResponseId && (
        <div className={tw`mx-5 my-4 overflow-auto rounded-lg border border-slate-200`}>
          <div
            className={tw`border-b border-slate-200 bg-slate-50 px-3 py-2 text-md font-medium leading-5 tracking-tight text-slate-800`}
          >
            Response
          </div>

          <ResponsePanel className={tw`p-5 pt-3`} responseId={lastResponseId} responseTab={responseTab} />
        </div>
      )}
    </>
  );
};

const EditConditionNodeView = ({ node: { nodeId, condition } }: EditPanelProps) => {
  const { control, handleSubmit, watch } = useForm({ values: condition! });

  const nodeUpdateMutation = useConnectMutation(nodeUpdate);

  const update = useDebouncedCallback(async () => {
    await handleSubmit(async (condition) => {
      await nodeUpdateMutation.mutateAsync({ nodeId, condition });
    })();
  }, 200);

  useEffect(() => {
    const subscription = watch(() => void update());
    return () => void subscription.unsubscribe();
  }, [update, watch]);

  return (
    <>
      <div className={tw`sticky top-0 z-10 flex items-center border-b border-slate-200 bg-white px-5 py-2`}>
        <div>
          <div className={tw`text-md leading-5 text-slate-400`}>If Condition</div>
          <div className={tw`text-sm font-medium leading-5 text-slate-800`}>Node Name</div>
        </div>

        <div className={tw`flex-1`} />

        <ButtonAsLink variant='ghost' className={tw`p-1`} href={{ from: Route.fullPath }}>
          <FiX className={tw`size-5 text-slate-500`} />
        </ButtonAsLink>
      </div>

      <div className={tw`m-5`}>
        <ConditionField control={control} path='condition' />
      </div>
    </>
  );
};

const EditForNodeView = ({ node: { nodeId, for: data } }: EditPanelProps) => {
  const { control, handleSubmit, watch } = useForm({ values: data! });

  const nodeUpdateMutation = useConnectMutation(nodeUpdate);

  const update = useDebouncedCallback(async () => {
    await handleSubmit(async (data) => {
      await nodeUpdateMutation.mutateAsync({ nodeId, for: data });
    })();
  }, 200);

  useEffect(() => {
    const subscription = watch(() => void update());
    return () => void subscription.unsubscribe();
  }, [update, watch]);

  return (
    <>
      <div className={tw`sticky top-0 z-10 flex items-center border-b border-slate-200 bg-white px-5 py-2`}>
        <div>
          <div className={tw`text-md leading-5 text-slate-400`}>For Loop</div>
          <div className={tw`text-sm font-medium leading-5 text-slate-800`}>Node Name</div>
        </div>

        <div className={tw`flex-1`} />

        <ButtonAsLink variant='ghost' className={tw`p-1`} href={{ from: Route.fullPath }}>
          <FiX className={tw`size-5 text-slate-500`} />
        </ButtonAsLink>
      </div>

      <div className={tw`m-5 grid grid-cols-[auto_1fr] gap-x-8 gap-y-5`}>
        <NumberFieldRHF
          control={control}
          name='iterations'
          label='Iterations'
          className={tw`contents`}
          groupClassName={tw`min-w-[30%] justify-self-start`}
        />

        <ConditionField control={control} path='condition' label='Break If' className={tw`contents`} />

        <SelectRHF
          control={control}
          name='errorHandling'
          label='On Error'
          className={tw`contents`}
          triggerClassName={tw`min-w-[30%] justify-between justify-self-start`}
        >
          <ListBoxItem id={ErrorHandling.UNSPECIFIED}>Throw</ListBoxItem>
          <ListBoxItem id={ErrorHandling.IGNORE}>Ignore</ListBoxItem>
          <ListBoxItem id={ErrorHandling.BREAK}>Break</ListBoxItem>
        </SelectRHF>
      </div>
    </>
  );
};

const EditForEachNodeView = ({ node: { nodeId, forEach } }: EditPanelProps) => {
  const { control, handleSubmit, watch } = useForm({ values: forEach! });

  const nodeUpdateMutation = useConnectMutation(nodeUpdate);

  const update = useDebouncedCallback(async () => {
    await handleSubmit(async (forEach) => {
      await nodeUpdateMutation.mutateAsync({ nodeId, forEach });
    })();
  }, 200);

  useEffect(() => {
    const subscription = watch(() => void update());
    return () => void subscription.unsubscribe();
  }, [update, watch]);

  return (
    <>
      <div className={tw`sticky top-0 z-10 flex items-center border-b border-slate-200 bg-white px-5 py-2`}>
        <div>
          <div className={tw`text-md leading-5 text-slate-400`}>For Each Loop</div>
          <div className={tw`text-sm font-medium leading-5 text-slate-800`}>Node Name</div>
        </div>

        <div className={tw`flex-1`} />

        <ButtonAsLink variant='ghost' className={tw`p-1`} href={{ from: Route.fullPath }}>
          <FiX className={tw`size-5 text-slate-500`} />
        </ButtonAsLink>
      </div>

      <div className={tw`m-5 grid grid-cols-[auto_1fr] gap-x-8 gap-y-5`}>
        <FieldLabel>Array to Loop</FieldLabel>
        <Controller
          control={control}
          name='path'
          defaultValue={[]}
          render={({ field }) => (
            <ReferenceField
              path={field.value}
              onSelect={field.onChange}
              buttonClassName={tw`min-w-[30%] justify-self-start`}
            />
          )}
        />

        <ConditionField control={control} path='condition' label='Break If' className={tw`contents`} />

        <SelectRHF
          control={control}
          name='errorHandling'
          label='On Error'
          className={tw`contents`}
          triggerClassName={tw`min-w-[30%] justify-between justify-self-start`}
        >
          <ListBoxItem id={ErrorHandling.UNSPECIFIED}>Throw</ListBoxItem>
          <ListBoxItem id={ErrorHandling.IGNORE}>Ignore</ListBoxItem>
          <ListBoxItem id={ErrorHandling.BREAK}>Break</ListBoxItem>
        </SelectRHF>
      </div>
    </>
  );
};
