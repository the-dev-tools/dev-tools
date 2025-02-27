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
import { Array, HashMap, Option, pipe } from 'effect';
import { Ulid } from 'id128';
import { useCallback, useRef } from 'react';
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

import { flowRoute } from './internal';

export { EdgeDTOSchema, type EdgeDTO };

export interface EdgeData extends Record<string, unknown> {
  state: NodeState;
}
export interface Edge extends EdgeCore<EdgeData> {}
export interface EdgeProps extends EdgePropsCore<Edge> {}

// const a: EdgeProps = {data: {}}

export const Edge = {
  fromDTO: (edge: Omit<EdgeDTO, keyof Message> & Message): Edge => ({
    id: Ulid.construct(edge.edgeId).toCanonical(),
    source: Ulid.construct(edge.sourceId).toCanonical(),
    sourceHandle: edge.sourceHandle === HandleKind.UNSPECIFIED ? null : enumToJson(HandleKindSchema, edge.sourceHandle),
    target: Ulid.construct(edge.targetId).toCanonical(),
    data: { state: NodeState.UNSPECIFIED },
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

export const useMakeEdge = () => {
  const { flowId } = flowRoute.useLoaderData();
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
    queryKey: pipe(
      createConnectQueryKey({ schema: edgeList, cardinality: 'finite', transport, input }),
      Array.append('react-flow'),
    ),
    queryFn: async () => pipe(await callUnaryMethod(transport, edgeList, input), (_) => _.items.map(Edge.fromDTO)),
  });

const DefaultEdge = ({ sourceX, sourceY, sourcePosition, targetX, targetY, targetPosition, data }: EdgeProps) => (
  <ConnectionLine
    fromX={sourceX}
    fromY={sourceY}
    fromPosition={sourcePosition}
    toX={targetX}
    toY={targetY}
    toPosition={targetPosition}
    connected
    state={data?.state}
  />
);

export const edgeTypes = {
  default: DefaultEdge,
};

const connectionLineStyles = tv({
  base: tw`fill-none stroke-1 transition-colors`,
  variants: {
    state: {
      [NodeState.UNSPECIFIED]: tw`stroke-slate-800`,
      [NodeState.RUNNING]: tw`stroke-violet-600`,
      [NodeState.SUCCESS]: tw`stroke-green-600`,
      [NodeState.FAILURE]: tw`stroke-red-600`,
    } satisfies Record<NodeState, string>,
  },
});

interface ConnectionLineProps
  extends Pick<ConnectionLineComponentProps, 'fromX' | 'fromY' | 'fromPosition' | 'toX' | 'toY' | 'toPosition'> {
  connected?: boolean;
  state?: NodeState | undefined;
}

export const ConnectionLine = ({
  fromX,
  fromY,
  fromPosition,
  toX,
  toY,
  toPosition,
  connected = false,
  state = NodeState.UNSPECIFIED,
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

  return <path className={connectionLineStyles({ state })} d={edgePath} strokeDasharray={connected ? undefined : 4} />;
};

export const useOnEdgesChange = () => {
  const { transport } = flowRoute.useRouteContext();
  const { flowId } = flowRoute.useLoaderData();

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

  const edgesQueryKey = edgesQueryOptions({ transport, flowId }).queryKey;
  return useCallback<OnEdgesChange<Edge>>(
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
};
