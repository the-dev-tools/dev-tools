import { create, enumFromJson, enumToJson, equals, isEnumJson, Message, MessageInitShape } from '@bufbuild/protobuf';
import { createClient, Transport } from '@connectrpc/connect';
import { callUnaryMethod, createConnectQueryKey, createQueryOptions } from '@connectrpc/connect-query';
import { queryOptions, useQueryClient, useSuspenseQueries } from '@tanstack/react-query';
import { createFileRoute, getRouteApi, redirect, ToOptions } from '@tanstack/react-router';
import {
  applyEdgeChanges,
  applyNodeChanges,
  Background,
  BackgroundVariant,
  ConnectionLineComponentProps,
  EdgeProps,
  getConnectedEdges,
  getSmoothStepPath,
  Handle as HandleCore,
  HandleProps,
  Node as NodeCore,
  NodeProps as NodePropsCore,
  NodeTypes as NodeTypesCore,
  OnEdgesChange,
  OnNodesChange,
  Position,
  ReactFlow,
  ReactFlowProps,
  ReactFlowProvider,
  Panel as RFPanel,
  useReactFlow,
  useViewport,
  type Edge,
} from '@xyflow/react';
import { Array, HashMap, Match, Option, pipe, Schema, Struct } from 'effect';
import { Ulid } from 'id128';
import { ComponentProps, ReactNode, Suspense, useCallback, useEffect, useMemo, useRef } from 'react';
import { Header, ListBoxSection, MenuTrigger } from 'react-aria-components';
import { Controller, useForm } from 'react-hook-form';
import { IconType } from 'react-icons';
import { FiExternalLink, FiMinus, FiMoreHorizontal, FiPlus, FiTerminal, FiX } from 'react-icons/fi';
import { Panel } from 'react-resizable-panels';
import { useDebouncedCallback } from 'use-debounce';

import { useConnectMutation, useConnectSuspenseQuery } from '@the-dev-tools/api/connect-query';
import { endpointGet } from '@the-dev-tools/spec/collection/item/endpoint/v1/endpoint-EndpointService_connectquery';
import {
  exampleCreate,
  exampleGet,
} from '@the-dev-tools/spec/collection/item/example/v1/example-ExampleService_connectquery';
import { collectionGet } from '@the-dev-tools/spec/collection/v1/collection-CollectionService_connectquery';
import {
  Edge as EdgeDTO,
  EdgeSchema as EdgeDTOSchema,
  EdgeListRequestSchema,
  Handle as HandleKind,
  HandleJson as HandleKindJson,
  HandleSchema as HandleKindSchema,
} from '@the-dev-tools/spec/flow/edge/v1/edge_pb';
import {
  edgeCreate,
  edgeDelete,
  edgeList,
  edgeUpdate,
} from '@the-dev-tools/spec/flow/edge/v1/edge-EdgeService_connectquery';
import {
  ErrorHandling,
  Node as NodeDTO,
  NodeSchema as NodeDTOSchema,
  NodeGetResponse,
  NodeKind,
  NodeKindJson,
  NodeKindSchema,
  NodeListRequestSchema,
  NodeNoOpKind,
  NodeRequest,
  NodeRequestSchema,
  PositionSchema,
} from '@the-dev-tools/spec/flow/node/v1/node_pb';
import {
  nodeCreate,
  nodeDelete,
  nodeGet,
  nodeList,
  nodeUpdate,
} from '@the-dev-tools/spec/flow/node/v1/node-NodeService_connectquery';
import { FlowGetResponse, FlowService } from '@the-dev-tools/spec/flow/v1/flow_pb';
import { flowDelete, flowGet, flowUpdate } from '@the-dev-tools/spec/flow/v1/flow-FlowService_connectquery';
import { Button, ButtonAsLink } from '@the-dev-tools/ui/button';
import { FieldLabel } from '@the-dev-tools/ui/field';
import {
  CheckListAltIcon,
  CollectIcon,
  DataSourceIcon,
  DelayIcon,
  ForIcon,
  IfIcon,
  PlayCircleIcon,
  PlayIcon,
  SendRequestIcon,
  Spinner,
} from '@the-dev-tools/ui/icons';
import { ListBox, ListBoxItem, ListBoxItemProps } from '@the-dev-tools/ui/list-box';
import { Menu, MenuItem, useContextMenuState } from '@the-dev-tools/ui/menu';
import { MethodBadge } from '@the-dev-tools/ui/method-badge';
import { NumberFieldRHF } from '@the-dev-tools/ui/number-field';
import { PanelResizeHandle } from '@the-dev-tools/ui/resizable-panel';
import { SelectRHF } from '@the-dev-tools/ui/select';
import { Separator } from '@the-dev-tools/ui/separator';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextField, useEditableTextState } from '@the-dev-tools/ui/text-field';

import { CollectionListTree } from './collection';
import { ConditionField } from './condition';
import { EndpointRequestView, EndpointRouteSearch, ResponsePanel, useEndpointUrl } from './endpoint';
import { ReferenceContext, ReferenceField } from './reference';

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

const workspaceRoute = getRouteApi('/_authorized/workspace/$workspaceIdCan');

const edgesQueryOptions = ({
  transport,
  ...input
}: MessageInitShape<typeof EdgeListRequestSchema> & { transport: Transport }) =>
  queryOptions({
    queryKey: pipe(
      createConnectQueryKey({ schema: edgeList, cardinality: 'finite', transport, input }),
      Array.append('react-flow'),
    ),
    queryFn: async () => pipe(await callUnaryMethod(transport, edgeList, input), (_) => _.items.map(Edge.fromDTO)),
  });

const nodesQueryOptions = ({
  transport,
  ...input
}: MessageInitShape<typeof NodeListRequestSchema> & { transport: Transport }) =>
  queryOptions({
    queryKey: pipe(
      createConnectQueryKey({ schema: nodeList, cardinality: 'finite', transport, input }),
      Array.append('react-flow'),
    ),
    queryFn: async () => pipe(await callUnaryMethod(transport, nodeList, input), (_) => _.items.map(Node.fromDTO)),
  });

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

const Edge = {
  fromDTO: (edge: Omit<EdgeDTO, keyof Message> & Message): Edge => ({
    id: Ulid.construct(edge.edgeId).toCanonical(),
    source: Ulid.construct(edge.sourceId).toCanonical(),
    sourceHandle: edge.sourceHandle === HandleKind.UNSPECIFIED ? null : enumToJson(HandleKindSchema, edge.sourceHandle),
    target: Ulid.construct(edge.targetId).toCanonical(),
  }),

  toDTO: (_: Edge): Omit<EdgeDTO, keyof Message> => ({
    edgeId: Ulid.fromCanonical(_.id).bytes,
    sourceId: Ulid.fromCanonical(_.source).bytes,
    sourceHandle: isEnumJson(HandleKindSchema, _.sourceHandle)
      ? enumFromJson(HandleKindSchema, _.sourceHandle)
      : HandleKind.UNSPECIFIED,
    targetId: Ulid.fromCanonical(_.target).bytes,
  }),
};

interface NodeData extends Omit<NodeDTO, keyof Message | 'nodeId' | 'kind' | 'position'> {}
interface Node extends NodeCore<NodeData> {}

const Node = {
  fromDTO: ({ nodeId, kind, position, ...data }: Omit<NodeDTO, keyof Message> & Message): Node => ({
    id: Ulid.construct(nodeId).toCanonical(),
    position: Struct.pick(position!, 'x', 'y'),
    origin: [0.5, 0],
    type: enumToJson(NodeKindSchema, kind),
    data: Struct.omit(data, '$typeName', '$unknown'),
  }),

  toDTO: (_: Node): Omit<NodeDTO, keyof Message> => ({
    ..._.data,
    nodeId: Ulid.fromCanonical(_.id).bytes,
    kind: isEnumJson(NodeKindSchema, _.type) ? enumFromJson(NodeKindSchema, _.type) : NodeKind.UNSPECIFIED,
    position: create(PositionSchema, _.position),
  }),
};

interface NodeProps extends NodePropsCore<Node> {}

const NoOpNode = (props: NodeProps) => {
  const kind = props.data.noOp;

  if (kind === NodeNoOpKind.CREATE) return <CreateNode {...props} />;

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

      {kind !== NodeNoOpKind.START && <Handle type='target' position={Position.Top} isConnectable={false} />}
      <Handle type='source' position={Position.Bottom} />
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
    async (data: Omit<MessageInitShape<typeof NodeDTOSchema>, keyof Message>) => {
      const { nodeId } = await nodeCreateMutation.mutateAsync({ flowId, ...data });
      return create(NodeDTOSchema, { nodeId, ...data });
    },
    [flowId, nodeCreateMutation],
  );
};

const useMakeEdge = () => {
  const { flowId } = Route.useLoaderData();
  const edgeCreateMutation = useConnectMutation(edgeCreate);
  return useCallback(
    async (data: Omit<MessageInitShape<typeof EdgeDTOSchema>, keyof Message>) => {
      const { edgeId } = await edgeCreateMutation.mutateAsync({ flowId, ...data });
      return create(EdgeDTOSchema, { edgeId, ...data });
    },
    [edgeCreateMutation, flowId],
  );
};

const CreateNode = ({ id }: NodeProps) => {
  const { getNode, getEdges, addNodes, addEdges, deleteElements } = useReactFlow();

  const edge = useMemo(() => getEdges().find((_) => _.target === id)!, [getEdges, id]);
  const sourceId = useMemo(() => Ulid.fromCanonical(edge.source).bytes, [edge.source]);

  const add = useCallback(
    async (nodes: NodeDTO[], edges: EdgeDTO[]) => {
      await deleteElements({ nodes: [{ id }], edges: [edge] });

      pipe(nodes.map(Node.fromDTO), addNodes);
      pipe(edges.map(Edge.fromDTO), addEdges);
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
                  sourceHandle: HandleKind.THEN,
                  targetId: nodeThen.nodeId,
                }),
                makeEdge({
                  sourceId: node.nodeId,
                  sourceHandle: HandleKind.ELSE,
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
                  sourceHandle: HandleKind.LOOP,
                  targetId: nodeLoop.nodeId,
                }),
                makeEdge({
                  sourceId: node.nodeId,
                  sourceHandle: HandleKind.THEN,
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
                  sourceHandle: HandleKind.LOOP,
                  targetId: nodeLoop.nodeId,
                }),
                makeEdge({
                  sourceId: node.nodeId,
                  sourceHandle: HandleKind.THEN,
                  targetId: nodeThen.nodeId,
                }),
              ]);

              await add([node, nodeLoop, nodeThen], edges);
            }}
          />
        </ListBoxSection>
      </ListBox>

      <Handle type='target' position={Position.Top} isConnectable={false} />
    </>
  );
};

interface NodeBaseProps {
  id: string;
  Icon: IconType;
  title: string;
  children: ReactNode;
}

// TODO: add node name
const NodeBase = ({ id, Icon, title, children }: NodeBaseProps) => {
  const { getEdges, getNode, deleteElements } = useReactFlow();

  const ref = useRef<HTMLDivElement>(null);
  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  return (
    <div ref={ref} className={tw`w-80 rounded-lg border border-slate-400 bg-slate-200 p-1 shadow-sm`}>
      <div
        className={tw`flex items-center gap-3 px-1 pb-1.5 pt-0.5`}
        onContextMenu={(event) => {
          const offset = ref.current?.getBoundingClientRect();
          if (!offset) return;
          onContextMenu(event, offset);
        }}
      >
        <Icon className={tw`size-5 text-slate-500`} />

        <div className={tw`h-4 w-px bg-slate-300`} />

        <span className={tw`flex-1 text-xs font-medium leading-5 tracking-tight`}>{title}</span>

        <MenuTrigger {...menuTriggerProps}>
          <Button variant='ghost' className={tw`p-0.5`}>
            <FiMoreHorizontal className={tw`size-4 text-slate-500`} />
          </Button>

          <Menu {...menuProps}>
            <MenuItem
              href={{
                to: '.',
                search: { selectedNodeIdCan: id } satisfies ToOptions['search'],
              }}
            >
              Edit
            </MenuItem>

            <MenuItem>Rename</MenuItem>

            <MenuItem
              variant='danger'
              onAction={async () => {
                const node = getNode(id);

                const { createEdges = [], edges = [] } = pipe(
                  getConnectedEdges([node!], getEdges()),
                  Array.groupBy((_) => (_.id.startsWith('create-') ? 'createEdges' : 'edges')),
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

const RequestNode = ({ id, data }: NodeProps) => {
  const { updateNodeData } = useReactFlow();

  const exampleCreateMutation = useConnectMutation(exampleCreate);

  return (
    <>
      <NodeBase id={id} Icon={SendRequestIcon} title='Send Request'>
        <div className={tw`rounded-md border border-slate-200 bg-white shadow-sm`}>
          {data.request?.exampleId.length !== 0 ? (
            <RequestNodeSelected request={data.request!} />
          ) : (
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
                updateNodeData(id, { ...data, request });
              }}
            />
          )}
        </div>
      </NodeBase>

      <Handle type='target' position={Position.Top} />
      <Handle type='source' position={Position.Bottom} />
    </>
  );
};

interface RequestNodeSelectedProps {
  request: NodeRequest;
}

const RequestNodeSelected = ({ request: { collectionId, endpointId, exampleId } }: RequestNodeSelectedProps) => {
  const { transport } = Route.useRouteContext();

  const [
    {
      data: { name: collectionName },
    },
    {
      data: { method },
    },
    {
      data: { name },
    },
  ] = useSuspenseQueries({
    queries: [
      createQueryOptions(collectionGet, { collectionId }, { transport }),
      createQueryOptions(endpointGet, { endpointId }, { transport }),
      createQueryOptions(exampleGet, { exampleId }, { transport }),
    ],
  });

  return (
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
};

const ConditionNode = ({ id, data }: NodeProps) => {
  const { condition } = data.condition!;

  return (
    <>
      <NodeBase id={id} Icon={IfIcon} title='If'>
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
                search: { selectedNodeIdCan: id } satisfies ToOptions['search'],
              }}
            >
              <FiPlus className={tw`size-4`} />
              <span>Setup Condition</span>
            </ButtonAsLink>
          )}
        </div>
      </NodeBase>

      <Handle type='target' position={Position.Top} />
      <Handle
        type='source'
        position={Position.Bottom}
        id={'HANDLE_THEN' satisfies HandleKindJson}
        isConnectable={false}
      />
      <Handle
        type='source'
        position={Position.Bottom}
        id={'HANDLE_ELSE' satisfies HandleKindJson}
        isConnectable={false}
      />
    </>
  );
};

const ForNode = ({ id }: NodeProps) => (
  <>
    <NodeBase id={id} Icon={ForIcon} title='For Loop'>
      <div className={tw`rounded-md border border-slate-200 bg-white shadow-sm`}>
        <ButtonAsLink
          className={tw`flex w-full justify-start gap-1.5 rounded-md border border-slate-200 px-2 py-3 text-xs font-medium leading-4 tracking-tight text-slate-800 shadow-sm`}
          href={{
            to: '.',
            search: { selectedNodeIdCan: id } satisfies ToOptions['search'],
          }}
        >
          <CheckListAltIcon className={tw`size-5 text-slate-500`} />
          <span>Edit Loop</span>
        </ButtonAsLink>
      </div>
    </NodeBase>

    <Handle type='target' position={Position.Top} />
    <Handle
      type='source'
      position={Position.Bottom}
      id={'HANDLE_LOOP' satisfies HandleKindJson}
      isConnectable={false}
    />
    <Handle
      type='source'
      position={Position.Bottom}
      id={'HANDLE_THEN' satisfies HandleKindJson}
      isConnectable={false}
    />
  </>
);

const ForEachNode = ({ id }: NodeProps) => (
  <>
    <NodeBase id={id} Icon={ForIcon} title='For Each Loop'>
      <div className={tw`rounded-md border border-slate-200 bg-white shadow-sm`}>
        <ButtonAsLink
          className={tw`flex w-full justify-start gap-1.5 rounded-md border border-slate-200 px-2 py-3 text-xs font-medium leading-4 tracking-tight text-slate-800 shadow-sm`}
          href={{
            to: '.',
            search: { selectedNodeIdCan: id } satisfies ToOptions['search'],
          }}
        >
          <CheckListAltIcon className={tw`size-5 text-slate-500`} />
          <span>Edit Loop</span>
        </ButtonAsLink>
      </div>
    </NodeBase>

    <Handle type='target' position={Position.Top} />
    <Handle
      type='source'
      position={Position.Bottom}
      id={'HANDLE_LOOP' satisfies HandleKindJson}
      isConnectable={false}
    />
    <Handle
      type='source'
      position={Position.Bottom}
      id={'HANDLE_THEN' satisfies HandleKindJson}
      isConnectable={false}
    />
  </>
);

const nodeTypes: Record<NodeKindJson, NodeTypesCore[string]> = {
  NODE_KIND_UNSPECIFIED: () => null,
  NODE_KIND_NO_OP: NoOpNode,
  NODE_KIND_REQUEST: RequestNode,
  NODE_KIND_CONDITION: ConditionNode,
  NODE_KIND_FOR: ForNode,
  NODE_KIND_FOR_EACH: ForEachNode,
};

const DefaultEdge = ({ sourceX, sourceY, sourcePosition, targetX, targetY, targetPosition }: EdgeProps) => (
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
  default: DefaultEdge,
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

const Handle = (props: HandleProps) => (
  <HandleCore
    className={tw`-z-10 flex size-5 items-center justify-center rounded-full border border-slate-300 bg-slate-200 shadow-sm`}
    {...props}
  >
    <div className={tw`pointer-events-none size-2 rounded-full bg-slate-800`} />
  </HandleCore>
);

const minZoom = 0.5;
const maxZoom = 2;

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

      <Button variant='primary' onPress={() => void flowRun({ flowId })}>
        <PlayCircleIcon className={tw`size-4`} />
        Run
      </Button>
    </RFPanel>
  );
};

interface FlowViewProps {
  flow: FlowGetResponse;
  edges: Edge[];
  nodes: Node[];
}

const FlowView = ({ flow, edges, nodes }: FlowViewProps) => {
  const { transport } = Route.useRouteContext();

  const { addNodes, addEdges, screenToFlowPosition } = useReactFlow();

  const queryClient = useQueryClient();

  const edgeCreateMutation = useConnectMutation(edgeCreate);
  const edgeDeleteMutation = useConnectMutation(edgeDelete);
  const edgeUpdateMutation = useConnectMutation(edgeUpdate);

  const oldEdges = useRef<Edge[]>(undefined);
  const saveEdges = useDebouncedCallback(async (newEdges: Edge[]) => {
    const oldEdgeMap = pipe(
      oldEdges.current ?? [],
      Array.map((_) => [_.id, Edge.toDTO(_)] as const),
      HashMap.fromIterable,
    );

    const newEdgeMap = pipe(
      newEdges.map((_) => [_.id, Edge.toDTO(_)] as const),
      HashMap.fromIterable,
    );

    const edges: Record<string, [string, ReturnType<typeof Edge.toDTO>][]> = pipe(
      HashMap.union(oldEdgeMap, newEdgeMap),
      HashMap.entries,
      Array.groupBy(([id]) => {
        const oldEdge = HashMap.get(oldEdgeMap, id);
        const newEdge = HashMap.get(newEdgeMap, id);

        if (Option.isNone(oldEdge)) return 'create';
        if (Option.isNone(newEdge)) return 'delete';

        return equals(EdgeDTOSchema, create(EdgeDTOSchema, oldEdge.value), create(EdgeDTOSchema, newEdge.value))
          ? 'ignore'
          : 'update';
      }),
    );

    await pipe(
      edges['create'] ?? [],
      Array.filterMap(([_id, edge]) =>
        pipe(
          Option.liftPredicate(edge, (_) => !_.edgeId.length),
          Option.map(edgeCreateMutation.mutateAsync),
        ),
      ),
      (_) => Promise.allSettled(_),
    );

    await pipe(
      edges['delete'] ?? [],
      Array.map(([_id, edge]) => edgeDeleteMutation.mutateAsync(edge)),
      (_) => Promise.allSettled(_),
    );

    await pipe(
      edges['update'] ?? [],
      Array.map(([_id, edge]) => edgeUpdateMutation.mutateAsync(edge)),
      (_) => Promise.allSettled(_),
    );

    oldEdges.current = undefined;
  }, 500);

  const edgesQueryKey = edgesQueryOptions({ transport, flowId: flow.flowId }).queryKey;
  const onEdgesChange = useCallback<OnEdgesChange>(
    async (changes) => {
      const newEdges = queryClient.setQueryData<Edge[]>(edgesQueryKey, (edges) => {
        if (edges === undefined) return undefined;
        if (oldEdges.current === undefined) oldEdges.current = edges;
        return applyEdgeChanges(changes, edges);
      });

      if (newEdges) await saveEdges(newEdges);
    },
    [edgesQueryKey, queryClient, saveEdges],
  );

  const nodeCreateMutation = useConnectMutation(nodeCreate);
  const nodeDeleteMutation = useConnectMutation(nodeDelete);
  const nodeUpdateMutation = useConnectMutation(nodeUpdate);

  const oldNodes = useRef<Node[]>(undefined);
  const saveNodes = useDebouncedCallback(async (newNodes: Node[]) => {
    const oldNodeMap = pipe(
      oldNodes.current ?? [],
      Array.map((_) => [_.id, Node.toDTO(_)] as const),
      HashMap.fromIterable,
    );

    const newNodeMap = pipe(
      newNodes.map((_) => [_.id, Node.toDTO(_)] as const),
      HashMap.fromIterable,
    );

    const nodes: Record<string, [string, ReturnType<typeof Node.toDTO>][]> = pipe(
      HashMap.union(oldNodeMap, newNodeMap),
      HashMap.entries,
      Array.groupBy(([id]) => {
        const oldNode = HashMap.get(oldNodeMap, id);
        const newNode = HashMap.get(newNodeMap, id);

        if (Option.isNone(oldNode)) return 'create';
        if (Option.isNone(newNode)) return 'delete';

        return equals(NodeDTOSchema, create(NodeDTOSchema, oldNode.value), create(NodeDTOSchema, newNode.value))
          ? 'ignore'
          : 'update';
      }),
    );

    await pipe(
      nodes['create'] ?? [],
      Array.filterMap(([_id, node]) =>
        pipe(
          Option.liftPredicate(node, (_) => !_.nodeId.length),
          Option.map(nodeCreateMutation.mutateAsync),
        ),
      ),
      (_) => Promise.allSettled(_),
    );

    await pipe(
      nodes['delete'] ?? [],
      Array.map(([_id, node]) => nodeDeleteMutation.mutateAsync(node)),
      (_) => Promise.allSettled(_),
    );

    await pipe(
      nodes['update'] ?? [],
      Array.map(([_id, node]) => nodeUpdateMutation.mutateAsync(node)),
      (_) => Promise.allSettled(_),
    );

    oldNodes.current = undefined;
  }, 500);

  const nodesQueryKey = nodesQueryOptions({ transport, flowId: flow.flowId }).queryKey;
  const onNodesChange = useCallback<OnNodesChange<Node>>(
    async (changes) => {
      const newNodes = queryClient.setQueryData<Node[]>(nodesQueryKey, (nodes) => {
        if (nodes === undefined) return undefined;
        if (oldNodes.current === undefined) oldNodes.current = nodes;
        return applyNodeChanges(changes, nodes);
      });

      if (newNodes) await saveNodes(newNodes);
    },
    [nodesQueryKey, queryClient, saveNodes],
  );

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

interface NodePanelProps {
  node: NodeGetResponse;
}

const RequestPanel = ({ node: { nodeId, request } }: NodePanelProps) => {
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

const ConditionPanel = ({ node: { nodeId, condition } }: NodePanelProps) => {
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

const ForPanel = ({ node: { nodeId, for: data } }: NodePanelProps) => {
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

const ForEachPanel = ({ node: { nodeId, forEach } }: NodePanelProps) => {
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
