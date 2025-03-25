import { create, enumFromJson, enumToJson, equals, isEnumJson, Message, MessageInitShape } from '@bufbuild/protobuf';
import { Transport } from '@connectrpc/connect';
import { callUnaryMethod, createConnectQueryKey } from '@connectrpc/connect-query';
import { queryOptions, useQueryClient } from '@tanstack/react-query';
import {
  applyNodeChanges,
  getConnectedEdges,
  Node as NodeCore,
  NodeProps as NodePropsCore,
  OnNodesChange,
  useReactFlow,
} from '@xyflow/react';
import { Array, HashMap, Match, Option, pipe, Struct } from 'effect';
import { Ulid } from 'id128';
import { ReactNode, Suspense, use, useCallback, useRef } from 'react';
import { MenuTrigger } from 'react-aria-components';
import { IconType } from 'react-icons';
import { FiMoreHorizontal } from 'react-icons/fi';
import { TbAlertTriangle, TbRefresh } from 'react-icons/tb';
import { tv } from 'tailwind-variants';
import { useDebouncedCallback } from 'use-debounce';

import { useConnectMutation } from '@the-dev-tools/api/connect-query';
import {
  Node as NodeDTO,
  NodeSchema as NodeDTOSchema,
  NodeGetResponse,
  NodeKind,
  NodeKindSchema,
  NodeListRequestSchema,
  NodeState,
  PositionSchema,
} from '@the-dev-tools/spec/flow/node/v1/node_pb';
import {
  nodeCreate,
  nodeDelete,
  nodeList,
  nodeUpdate,
} from '@the-dev-tools/spec/flow/node/v1/node-NodeService_connectquery';
import { Button } from '@the-dev-tools/ui/button';
import { CheckIcon, Spinner } from '@the-dev-tools/ui/icons';
import { Menu, MenuItem, useContextMenuState } from '@the-dev-tools/ui/menu';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextField, useEditableTextState } from '@the-dev-tools/ui/text-field';
import { useEscapePortal } from '@the-dev-tools/ui/utils';

import { FlowContext, flowRoute } from './internal';
import { FlowSearch } from './layout';

export { type NodeDTO, NodeDTOSchema };

export interface NodeData extends Omit<NodeDTO, 'kind' | 'nodeId' | 'position' | keyof Message> {}
export interface Node extends NodeCore<NodeData> {}
export interface NodeProps extends NodePropsCore<Node> {}

export interface NodePanelProps {
  node: NodeGetResponse;
}

export const Node = {
  fromDTO: ({ kind, nodeId, position, ...data }: Message & Omit<NodeDTO, keyof Message>): Node => ({
    data: Struct.omit(data, '$typeName', '$unknown'),
    id: Ulid.construct(nodeId).toCanonical(),
    origin: [0.5, 0],
    position: Struct.pick(position!, 'x', 'y'),
    type: enumToJson(NodeKindSchema, kind),
  }),

  toDTO: (_: Node): Omit<NodeDTO, 'state' | keyof Message> => ({
    ...Struct.omit(_.data, 'state'),
    kind: isEnumJson(NodeKindSchema, _.type) ? enumFromJson(NodeKindSchema, _.type) : NodeKind.UNSPECIFIED,
    nodeId: Ulid.fromCanonical(_.id).bytes,
    position: create(PositionSchema, _.position),
  }),
};

const nodeBaseStyles = tv({
  base: tw`nopan shadow-xs relative w-80 rounded-lg bg-slate-200 p-1 outline-1 transition-colors`,
  variants: {
    isSelected: { true: tw`bg-slate-300` },
    state: {
      [NodeState.FAILURE]: tw`outline-red-600`,
      [NodeState.RUNNING]: tw`outline-violet-600`,
      [NodeState.SUCCESS]: tw`outline-green-600`,
      [NodeState.UNSPECIFIED]: tw`outline-slate-300`,
    } satisfies Record<NodeState, string>,
  },
});

interface NodeBaseProps extends NodeProps {
  children: ReactNode;
  Icon: IconType;
}

// TODO: add node name
export const NodeBase = ({ children, data: { name, state }, Icon, id, selected }: NodeBaseProps) => {
  const { deleteElements, getEdges, getNode, getZoom } = useReactFlow();
  const { isReadOnly = false } = use(FlowContext);

  const nodeUpdateMutation = useConnectMutation(nodeUpdate);

  const ref = useRef<HTMLDivElement>(null);
  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  const escape = useEscapePortal(ref);

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    onSuccess: (_) => nodeUpdateMutation.mutateAsync({ name: _, nodeId: Ulid.fromCanonical(id).bytes }),
    value: name,
  });

  return (
    <div className={nodeBaseStyles({ isSelected: selected, state })} ref={ref}>
      <div
        className={tw`flex items-center gap-3 px-1 pb-1.5 pt-0.5`}
        onContextMenu={(event) => {
          const offset = ref.current?.getBoundingClientRect();
          if (!offset) return;
          onContextMenu(event, offset, getZoom());
        }}
      >
        <Icon className={tw`size-5 text-slate-500`} />

        <div className={tw`h-4 w-px bg-slate-300`} />

        <div className={tw`flex-1 truncate text-xs font-medium leading-5 tracking-tight`} ref={escape.ref}>
          {name}
        </div>

        {isEditing &&
          escape.render(
            <TextField
              aria-label='New node name'
              className={tw`w-full`}
              inputClassName={tw`-my-1 py-1`}
              isDisabled={nodeUpdateMutation.isPending}
              {...textFieldProps}
            />,
            getZoom(),
          )}

        {pipe(
          Match.value(state),
          Match.when(NodeState.RUNNING, () => (
            <TbRefresh className={tw`size-5 animate-spin text-violet-600`} style={{ animationDirection: 'reverse' }} />
          )),
          Match.when(NodeState.SUCCESS, () => <CheckIcon className={tw`size-5 text-green-600`} />),
          Match.when(NodeState.FAILURE, () => <TbAlertTriangle className={tw`size-5 text-red-600`} />),
          Match.orElse(() => null),
        )}

        {!isReadOnly && (
          <MenuTrigger {...menuTriggerProps}>
            <Button className={tw`p-0.5`} variant='ghost'>
              <FiMoreHorizontal className={tw`size-4 text-slate-500`} />
            </Button>

            <Menu {...menuProps}>
              <MenuItem href={{ search: (_: Partial<FlowSearch>) => ({ ..._, node: id }), to: '.' }}>Edit</MenuItem>

              <MenuItem onAction={() => void edit()}>Rename</MenuItem>

              <MenuItem
                onAction={async () => {
                  const node = getNode(id);

                  const { createEdges = [], edges = [] } = pipe(
                    getConnectedEdges([node!], getEdges()),
                    Array.groupBy((_) => (_.id.startsWith('create-') ? 'createEdges' : 'edges')),
                  );

                  await deleteElements({
                    edges: [...createEdges, ...edges],
                    nodes: [{ id }, ...createEdges.map((_) => ({ id: _.target }))],
                  });
                }}
                variant='danger'
              >
                Delete
              </MenuItem>
            </Menu>
          </MenuTrigger>
        )}
      </div>

      <Suspense
        fallback={
          <div className={tw`flex h-full items-center justify-center`}>
            <Spinner className={tw`size-8`} />
          </div>
        }
      >
        {children}
      </Suspense>
    </div>
  );
};

export const useMakeNode = () => {
  const { flowId } = use(FlowContext);
  const nodeCreateMutation = useConnectMutation(nodeCreate);
  return useCallback(
    async (data: Omit<MessageInitShape<typeof NodeDTOSchema>, keyof Message>) => {
      const { nodeId } = await nodeCreateMutation.mutateAsync({ flowId, ...data });
      return create(NodeDTOSchema, { nodeId, ...data });
    },
    [flowId, nodeCreateMutation],
  );
};

export const nodesQueryOptions = ({
  transport,
  ...input
}: MessageInitShape<typeof NodeListRequestSchema> & { transport: Transport }) =>
  queryOptions({
    queryFn: async () => pipe(await callUnaryMethod(transport, nodeList, input), (_) => _.items.map(Node.fromDTO)),
    queryKey: pipe(
      createConnectQueryKey({ cardinality: 'finite', input, schema: nodeList, transport }),
      Array.append('react-flow'),
    ),
  });

export const useOnNodesChange = () => {
  const { transport } = flowRoute.useRouteContext();
  const { flowId, isReadOnly = false } = use(FlowContext);

  const queryClient = useQueryClient();

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

  const nodesQueryKey = nodesQueryOptions({ flowId, transport }).queryKey;

  return useCallback<OnNodesChange<Node>>(
    async (changes) => {
      const newNodes = queryClient.setQueryData<Node[]>(nodesQueryKey, (nodes) => {
        if (nodes === undefined) return undefined;
        if (oldNodes.current === undefined) oldNodes.current = nodes;
        return applyNodeChanges(changes, nodes);
      });

      if (newNodes && !isReadOnly) await saveNodes(newNodes);
    },
    [isReadOnly, nodesQueryKey, queryClient, saveNodes],
  );
};
