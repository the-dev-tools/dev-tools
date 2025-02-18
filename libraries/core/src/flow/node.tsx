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
import { ReactNode, useCallback, useRef } from 'react';
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
  PositionSchema,
} from '@the-dev-tools/spec/flow/node/v1/node_pb';
import {
  nodeCreate,
  nodeDelete,
  nodeList,
  nodeUpdate,
} from '@the-dev-tools/spec/flow/node/v1/node-NodeService_connectquery';
import { NodeState } from '@the-dev-tools/spec/flow/v1/flow_pb';
import { Button } from '@the-dev-tools/ui/button';
import { CheckIcon } from '@the-dev-tools/ui/icons';
import { Menu, MenuItem, useContextMenuState } from '@the-dev-tools/ui/menu';
import { tw } from '@the-dev-tools/ui/tailwind-literal';

import { flowRoute, useSetSelectedNodes } from './internal';

export { NodeDTOSchema, type NodeDTO };

export interface NodeData
  extends Record<string, unknown>,
    Omit<NodeDTO, keyof Message | 'nodeId' | 'kind' | 'position'> {
  state: NodeState;
}
export interface Node extends NodeCore<NodeData> {}
export interface NodeProps extends NodePropsCore<Node> {}

export interface NodePanelProps {
  node: NodeGetResponse;
}

export const Node = {
  fromDTO: ({ nodeId, kind, position, ...data }: Omit<NodeDTO, keyof Message> & Message): Node => ({
    id: Ulid.construct(nodeId).toCanonical(),
    position: Struct.pick(position!, 'x', 'y'),
    origin: [0.5, 0],
    type: enumToJson(NodeKindSchema, kind),
    data: { ...Struct.omit(data, '$typeName', '$unknown'), state: NodeState.UNSPECIFIED },
  }),

  toDTO: (_: Node): Omit<NodeDTO, keyof Message> => ({
    ..._.data,
    nodeId: Ulid.fromCanonical(_.id).bytes,
    kind: isEnumJson(NodeKindSchema, _.type) ? enumFromJson(NodeKindSchema, _.type) : NodeKind.UNSPECIFIED,
    position: create(PositionSchema, _.position),
  }),
};

const nodeBaseStyles = tv({
  base: tw`w-80 rounded-lg border bg-slate-200 p-1 shadow-sm transition-colors`,
  variants: {
    state: {
      [NodeState.UNSPECIFIED]: tw`border-slate-200`,
      [NodeState.RUNNING]: tw`border-violet-600`,
      [NodeState.SUCCESS]: tw`border-green-600`,
      [NodeState.FAILURE]: tw`border-red-600`,
    } satisfies Record<NodeState, string>,
  },
});

interface NodeBaseProps extends NodeProps {
  Icon: IconType;
  title: string;
  children: ReactNode;
}

// TODO: add node name
export const NodeBase = ({ id, data: { state }, Icon, title, children }: NodeBaseProps) => {
  const { getEdges, getNode, deleteElements } = useReactFlow();

  const setSelectedNodes = useSetSelectedNodes();

  const ref = useRef<HTMLDivElement>(null);
  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  return (
    <div ref={ref} className={nodeBaseStyles({ state })}>
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

        {pipe(
          Match.value(state),
          Match.when(NodeState.RUNNING, () => (
            <TbRefresh className={tw`size-5 animate-spin text-violet-600`} style={{ animationDirection: 'reverse' }} />
          )),
          Match.when(NodeState.SUCCESS, () => <CheckIcon className={tw`size-5 text-green-600`} />),
          Match.when(NodeState.FAILURE, () => <TbAlertTriangle className={tw`size-5 text-red-600`} />),
          Match.orElse(() => null),
        )}

        <MenuTrigger {...menuTriggerProps}>
          <Button variant='ghost' className={tw`p-0.5`}>
            <FiMoreHorizontal className={tw`size-4 text-slate-500`} />
          </Button>

          <Menu {...menuProps}>
            <MenuItem onAction={() => void setSelectedNodes([id])}>Edit</MenuItem>

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

export const useMakeNode = () => {
  const { flowId } = flowRoute.useLoaderData();
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
    queryKey: pipe(
      createConnectQueryKey({ schema: nodeList, cardinality: 'finite', transport, input }),
      Array.append('react-flow'),
    ),
    queryFn: async () => pipe(await callUnaryMethod(transport, nodeList, input), (_) => _.items.map(Node.fromDTO)),
  });

export const useOnNodesChange = () => {
  const { transport } = flowRoute.useRouteContext();
  const { flowId } = flowRoute.useLoaderData();

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

  const nodesQueryKey = nodesQueryOptions({ transport, flowId }).queryKey;

  return useCallback<OnNodesChange<Node>>(
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
};
