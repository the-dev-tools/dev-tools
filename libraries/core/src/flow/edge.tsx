import { create, enumFromJson, enumToJson, equals, isEnumJson, Message, MessageInitShape } from '@bufbuild/protobuf';
import { Transport } from '@connectrpc/connect';
import { callUnaryMethod, createConnectQueryKey } from '@connectrpc/connect-query';
import { queryOptions, useQueryClient } from '@tanstack/react-query';
import {
  applyEdgeChanges,
  ConnectionLineComponentProps,
  Edge as EdgeCore,
  EdgeProps as EdgePropsCore,
  getSmoothStepPath,
  OnEdgesChange,
} from '@xyflow/react';
import { Array, HashMap, Option, pipe, Struct } from 'effect';
import { Ulid } from 'id128';
import { use, useCallback, useRef } from 'react';
import { tv } from 'tailwind-variants';
import { useDebouncedCallback } from 'use-debounce';

import { useConnectMutation } from '@the-dev-tools/api/connect-query';
import {
  Edge as EdgeDTO,
  EdgeSchema as EdgeDTOSchema,
  EdgeListRequestSchema,
  Handle as HandleKind,
  HandleSchema as HandleKindSchema,
} from '@the-dev-tools/spec/flow/edge/v1/edge_pb';
import {
  edgeCreate,
  edgeDelete,
  edgeList,
  edgeUpdate,
} from '@the-dev-tools/spec/flow/edge/v1/edge-EdgeService_connectquery';
import { NodeState } from '@the-dev-tools/spec/flow/node/v1/node_pb';
import { tw } from '@the-dev-tools/ui/tailwind-literal';

import { FlowContext, flowRoute } from './internal';

export { type EdgeDTO, EdgeDTOSchema };

export interface EdgeData extends Record<string, unknown> {
  state: NodeState;
}
export interface Edge extends EdgeCore<EdgeData> {}
export interface EdgeProps extends EdgePropsCore<Edge> {}

export const Edge = {
  fromDTO: (edge: Message & Omit<EdgeDTO, keyof Message>): Edge => ({
    data: { state: NodeState.UNSPECIFIED },
    id: Ulid.construct(edge.edgeId).toCanonical(),
    source: Ulid.construct(edge.sourceId).toCanonical(),
    sourceHandle: edge.sourceHandle === HandleKind.UNSPECIFIED ? null : enumToJson(HandleKindSchema, edge.sourceHandle),
    target: Ulid.construct(edge.targetId).toCanonical(),
  }),

  toDTO: (_: Partial<Edge>): Omit<EdgeDTO, keyof Message> =>
    pipe(
      create(EdgeDTOSchema, {
        edgeId: pipe(
          Option.fromNullable(_.id),
          Option.map((_) => Ulid.fromCanonical(_).bytes),
          Option.getOrUndefined,
        )!,
        sourceHandle: isEnumJson(HandleKindSchema, _.sourceHandle)
          ? enumFromJson(HandleKindSchema, _.sourceHandle)
          : HandleKind.UNSPECIFIED,
        sourceId: pipe(
          Option.fromNullable(_.source),
          Option.map((_) => Ulid.fromCanonical(_).bytes),
          Option.getOrUndefined,
        )!,
        targetId: pipe(
          Option.fromNullable(_.target),
          Option.map((_) => Ulid.fromCanonical(_).bytes),
          Option.getOrUndefined,
        )!,
      }),
      Struct.omit('$typeName', '$unknown'),
    ),
};

export const useMakeEdge = () => {
  const { flowId } = use(FlowContext);
  const edgeCreateMutation = useConnectMutation(edgeCreate);
  return useCallback(
    async (data: Omit<MessageInitShape<typeof EdgeDTOSchema>, keyof Message>) => {
      const { edgeId } = await edgeCreateMutation.mutateAsync({ flowId, ...data });
      return create(EdgeDTOSchema, { edgeId, ...data });
    },
    [edgeCreateMutation, flowId],
  );
};

export const edgesQueryOptions = ({
  transport,
  ...input
}: MessageInitShape<typeof EdgeListRequestSchema> & { transport: Transport }) =>
  queryOptions({
    queryFn: async () => pipe(await callUnaryMethod(transport, edgeList, input), (_) => _.items.map(Edge.fromDTO)),
    queryKey: pipe(
      createConnectQueryKey({ cardinality: 'finite', input, schema: edgeList, transport }),
      Array.append('react-flow'),
    ),
  });

const DefaultEdge = ({ data, sourcePosition, sourceX, sourceY, targetPosition, targetX, targetY }: EdgeProps) => (
  <ConnectionLine
    connected
    fromPosition={sourcePosition}
    fromX={sourceX}
    fromY={sourceY}
    state={data?.state}
    toPosition={targetPosition}
    toX={targetX}
    toY={targetY}
  />
);

export const edgeTypes = {
  default: DefaultEdge,
};

const connectionLineStyles = tv({
  base: tw`fill-none stroke-1 transition-colors`,
  variants: {
    state: {
      [NodeState.FAILURE]: tw`stroke-red-600`,
      [NodeState.RUNNING]: tw`stroke-violet-600`,
      [NodeState.SUCCESS]: tw`stroke-green-600`,
      [NodeState.UNSPECIFIED]: tw`stroke-slate-800`,
    } satisfies Record<NodeState, string>,
  },
});

interface ConnectionLineProps
  extends Pick<ConnectionLineComponentProps, 'fromPosition' | 'fromX' | 'fromY' | 'toPosition' | 'toX' | 'toY'> {
  connected?: boolean;
  state?: NodeState | undefined;
}

export const ConnectionLine = ({
  connected = false,
  fromPosition,
  fromX,
  fromY,
  state = NodeState.UNSPECIFIED,
  toPosition,
  toX,
  toY,
}: ConnectionLineProps) => {
  const [edgePath] = getSmoothStepPath({
    borderRadius: 8,
    offset: 8,
    sourcePosition: fromPosition,
    sourceX: fromX,
    sourceY: fromY,
    targetPosition: toPosition,
    targetX: toX,
    targetY: toY,
  });

  return <path className={connectionLineStyles({ state })} d={edgePath} strokeDasharray={connected ? undefined : 4} />;
};

export const useOnEdgesChange = () => {
  const { transport } = flowRoute.useRouteContext();
  const { flowId, isReadOnly = false } = use(FlowContext);

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

  const edgesQueryKey = edgesQueryOptions({ flowId, transport }).queryKey;
  return useCallback<OnEdgesChange<Edge>>(
    async (changes) => {
      const newEdges = queryClient.setQueryData<Edge[]>(edgesQueryKey, (edges) => {
        if (edges === undefined) return undefined;
        oldEdges.current ??= edges;
        return applyEdgeChanges(changes, edges);
      });

      if (newEdges && !isReadOnly) await saveEdges(newEdges);
    },
    [edgesQueryKey, isReadOnly, queryClient, saveEdges],
  );
};
