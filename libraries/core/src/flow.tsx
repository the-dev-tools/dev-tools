import { create, enumToJson, MessageInitShape } from '@bufbuild/protobuf';
import {
  createQueryOptions,
  useMutation as useConnectMutation,
  useQuery as useConnectQuery,
} from '@connectrpc/connect-query';
import { useSuspenseQueries } from '@tanstack/react-query';
import { createFileRoute, redirect, ToOptions } from '@tanstack/react-router';
import {
  addEdge,
  Background,
  BackgroundVariant,
  Handle as BaseHandle,
  ConnectionLineComponentProps,
  Edge,
  EdgeProps,
  Panel as FlowPanel,
  getConnectedEdges,
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
  useViewport,
} from '@xyflow/react';
import { Array, Match, Option, pipe, Schema, String } from 'effect';
import { Ulid } from 'id128';
import { ComponentProps, useCallback, useMemo } from 'react';
import { Header, ListBoxSection, MenuTrigger } from 'react-aria-components';
import { IconType } from 'react-icons';
import { FiExternalLink, FiMinus, FiMoreHorizontal, FiPlus, FiTerminal, FiX } from 'react-icons/fi';
import { Panel, PanelGroup } from 'react-resizable-panels';

import { endpointGet } from '@the-dev-tools/spec/collection/item/endpoint/v1/endpoint-EndpointService_connectquery';
import { exampleGet } from '@the-dev-tools/spec/collection/item/example/v1/example-ExampleService_connectquery';
import { collectionGet } from '@the-dev-tools/spec/collection/v1/collection-CollectionService_connectquery';
import { EdgeListItem } from '@the-dev-tools/spec/flow/edge/v1/edge_pb';
import { edgeCreate, edgeDelete, edgeList } from '@the-dev-tools/spec/flow/edge/v1/edge-EdgeService_connectquery';
import {
  NodeGetResponse,
  NodeKind,
  NodeKindJson,
  NodeKindSchema,
  NodeListItem,
  NodeRequest,
  NodeSchema,
  NodeStart,
} from '@the-dev-tools/spec/flow/node/v1/node_pb';
import {
  nodeCreate,
  nodeDelete,
  nodeGet,
  nodeList,
  nodeUpdate,
} from '@the-dev-tools/spec/flow/node/v1/node-NodeService_connectquery';
import { FlowGetResponse } from '@the-dev-tools/spec/flow/v1/flow_pb';
import { flowGet } from '@the-dev-tools/spec/flow/v1/flow-FlowService_connectquery';
import { Button, ButtonAsLink } from '@the-dev-tools/ui/button';
import {
  ChatAddIcon,
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
import { PanelResizeHandle } from '@the-dev-tools/ui/resizable-panel';
import { Separator } from '@the-dev-tools/ui/separator';
import { tw } from '@the-dev-tools/ui/tailwind-literal';

import { CollectionListTree } from './collection';
import { EndpointRequestView, EndpointRouteSearch, ResponsePanel, useEndpointUrl } from './endpoint';
import { QueryDeltaTable } from './query';

class Search extends EndpointRouteSearch.extend<Search>('FlowRouteSearch')({
  selectedNodeIdCan: pipe(Schema.String, Schema.optional),
}) {}

export const Route = createFileRoute('/_authorized/workspace/$workspaceIdCan/flow/$flowIdCan')({
  component: RouteComponent,
  pendingComponent: () => 'Loading flow...',
  validateSearch: (_) => Schema.decodeSync(Search)(_),
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
        from: route.fullPath as NonNullable<ToOptions['from']>,
        to: '../..' as NonNullable<ToOptions['to']>,
        throw: true,
      });
    }

    return { flowId };
  },
});

function RouteComponent() {
  const { flowId } = Route.useLoaderData();
  const { selectedNodeIdCan } = Route.useSearch();

  const flowQuery = useConnectQuery(flowGet, { flowId });
  const edgeListQuery = useConnectQuery(edgeList, { flowId });
  const nodeListQuery = useConnectQuery(nodeList, { flowId });
  const selectedNodeQuery = useConnectQuery(
    nodeGet,
    { nodeId: selectedNodeIdCan ? Ulid.fromCanonical(selectedNodeIdCan).bytes : undefined! },
    { enabled: selectedNodeIdCan !== undefined },
  );

  if (!flowQuery.data || !edgeListQuery.data || !nodeListQuery.data) return null;

  return (
    <ReactFlowProvider>
      <PanelGroup direction='vertical'>
        <Panel id='request' order={1} className='flex h-full flex-col'>
          <FlowView flow={flowQuery.data} edges={edgeListQuery.data.items} nodes={nodeListQuery.data.items} />
        </Panel>
        {selectedNodeQuery.data && (
          <>
            <PanelResizeHandle direction='vertical' />
            <Panel id='response' order={2} defaultSize={40} className={tw`!overflow-auto`}>
              <EditPanel node={selectedNodeQuery.data} />
            </Panel>
          </>
        )}
      </PanelGroup>
    </ReactFlowProvider>
  );
}

const mapEdgeToClient = (edge: EdgeListItem) =>
  ({
    id: Ulid.construct(edge.edgeId).toCanonical(),
    source: Ulid.construct(edge.sourceId).toCanonical(),
    target: Ulid.construct(edge.targetId).toCanonical(),
  }) satisfies Edge;

const nodeKindToString = (kind: NodeKind) =>
  pipe(
    enumToJson(NodeKindSchema, kind),
    String.substring('NODE_KIND_'.length),
    (_) => _ as NodeKindJson extends `NODE_KIND_${infer Kind}` ? Kind : never,
    String.toLowerCase,
  );

const mapNodeToClient = (node: Omit<NodeListItem, '$typeName'>) => {
  const kind = nodeKindToString(node.kind);
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

const CreateNodeView = ({ id, positionAbsoluteX, positionAbsoluteY }: NodeProps<CreateNode>) => {
  const { getEdge, addNodes, addEdges, deleteElements } = useReactFlow();

  const sourceIdCan = getEdge(id)!.source;
  const sourceId = Ulid.fromCanonical(sourceIdCan).bytes;

  const { flowId } = Route.useLoaderData();

  const nodeCreateMutation = useConnectMutation(nodeCreate);
  const edgeCreateMutation = useConnectMutation(edgeCreate);

  const position = useMemo(
    () => ({ x: positionAbsoluteX, y: positionAbsoluteY }),
    [positionAbsoluteX, positionAbsoluteY],
  );

  const makeNode = useCallback(
    async (kind: NodeKind, initData?: Omit<MessageInitShape<typeof NodeSchema>, '$typeName'>) => {
      const type = nodeKindToString(kind);
      if (type === 'unspecified') return;

      const data = { ...initData, [type]: { ...initData?.[type], position } };

      const { nodeId } = await nodeCreateMutation.mutateAsync({ flowId, kind, ...data });
      const { edgeId } = await edgeCreateMutation.mutateAsync({ flowId, sourceId, targetId: nodeId });

      const nodeIdCan = Ulid.construct(nodeId).toCanonical();

      const node = {
        id: nodeIdCan,
        position,
        type,
        data: { ...create(NodeSchema, data)[type]!, nodeId },
      } satisfies Partial<Node>;

      const edge = {
        id: Ulid.construct(edgeId).toCanonical(),
        source: sourceIdCan,
        target: nodeIdCan,
      } satisfies Edge;

      await deleteElements({ nodes: [{ id }], edges: [{ id }] });

      addNodes(node);
      addEdges(edge);
    },
    [
      addEdges,
      addNodes,
      deleteElements,
      edgeCreateMutation,
      flowId,
      id,
      nodeCreateMutation,
      position,
      sourceId,
      sourceIdCan,
    ],
  );

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
            onAction={() => void makeNode(NodeKind.REQUEST)}
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

          <CreateNodeItem id='condition' Icon={IfIcon} title='If' description='Add true/false' />
        </ListBoxSection>

        <ListBoxSection>
          <CreateNodeHeader>Looping</CreateNodeHeader>

          <CreateNodeItem id='collect' Icon={CollectIcon} title='Collect' description='Collect all result' />

          <CreateNodeItem id='for' Icon={ForIcon} title='For Loop' description='Loop' />

          <CreateNodeItem id='foreach' Icon={ForIcon} title='For Each Loop' description='Loop' />
        </ListBoxSection>
      </ListBox>

      <Handle type='target' position={Position.Top} isConnectable={false} />
    </>
  );
};

interface RequestNode extends Node<NodeRequest, 'request'> {}

const RequestNodeView = ({ id, data }: NodeProps<RequestNode>) => {
  const { nodeId, collectionId, endpointId, exampleId } = data;

  const nodeIdCan = Ulid.construct(nodeId).toCanonical();

  const { updateNodeData, getEdges, getNode, deleteElements } = useReactFlow();

  const collectionGetQuery = useConnectQuery(collectionGet, { collectionId }, { enabled: collectionId.length > 0 });
  const endpointGetQuery = useConnectQuery(endpointGet, { endpointId }, { enabled: endpointId.length > 0 });
  const exampleGetQuery = useConnectQuery(exampleGet, { exampleId }, { enabled: exampleId.length > 0 });

  const nodeUpdateMutation = useConnectMutation(nodeUpdate);
  const nodeDeleteMutation = useConnectMutation(nodeDelete);
  const edgeDeleteMutation = useConnectMutation(edgeDelete);

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
          const newData = { ...data, collectionId, endpointId, exampleId };
          await nodeUpdateMutation.mutateAsync({ nodeId, request: newData });
          updateNodeData(id, newData);
        }}
      />
    );
  }

  return (
    <>
      <div className={tw`w-80 rounded-lg border border-slate-400 bg-slate-200 p-1 shadow-sm`}>
        <div className={tw`flex items-center gap-3 px-1 pb-1.5 pt-0.5`}>
          <SendRequestIcon className={tw`size-5 text-slate-500`} />

          <div className={tw`h-4 w-px bg-slate-300`} />

          <span className={tw`flex-1 text-xs font-medium leading-5 tracking-tight`}>Send Request</span>

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

        <div className={tw`rounded-md border border-slate-200 bg-white shadow-sm`}>{content}</div>
      </div>

      <Handle type='target' position={Position.Top} />
      <Handle type='source' position={Position.Bottom} />
    </>
  );
};

const nodeTypes: NodeTypes = {
  start: StartNodeView,
  create: CreateNodeView,
  request: RequestNodeView,
};

const EdgeView = ({ sourceX, sourceY, sourcePosition, targetX, targetY, targetPosition }: EdgeProps) => (
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

const Handle = (props: HandleProps) => (
  <BaseHandle
    className={tw`-z-10 flex size-5 items-center justify-center rounded-full border border-slate-300 bg-slate-200 shadow-sm`}
    {...props}
  >
    <div className={tw`pointer-events-none size-2 rounded-full bg-slate-800`} />
  </BaseHandle>
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
    <FlowPanel className={tw`m-0 flex w-full items-center gap-2 border-b border-slate-200 bg-white px-3 py-3.5`}>
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
    </FlowPanel>
  );
};

const ActionBar = () => (
  <FlowPanel className={tw`mb-4 flex items-center gap-2 rounded-lg bg-slate-900 p-1 shadow`} position='bottom-center'>
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

    <Button variant='primary' className={tw``}>
      <PlayCircleIcon className={tw`size-4`} />
      Run
    </Button>
  </FlowPanel>
);

interface FlowViewProps {
  flow: FlowGetResponse;
  edges: EdgeListItem[];
  nodes: NodeListItem[];
}

const FlowView = ({ flow, edges: serverEdges, nodes: serverNodes }: FlowViewProps) => {
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
    Match.value(node),
    Match.when({ kind: NodeKind.REQUEST }, (_) => <EditRequestNodeView data={_.request!} />),
    Match.orElse(() => null),
  );

interface EditRequestNodeViewProps {
  data: NodeRequest;
}

const EditRequestNodeView = ({
  data: { collectionId, endpointId, exampleId, deltaExampleId },
}: EditRequestNodeViewProps) => {
  const { requestTab, responseTab } = Route.useSearch();
  const { transport } = Route.useRouteContext();

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

        <div className={tw`p-5 py-3`}>
          <QueryDeltaTable exampleId={exampleId} deltaExampleId={deltaExampleId} />
        </div>

        {/* TODO: implement delta views */}
        <EndpointRequestView
          className={tw`hidden p-5 pt-3`}
          endpointId={endpointId}
          exampleId={exampleId}
          requestTab={requestTab}
        />
      </div>

      {lastResponseId.length > 0 && (
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
