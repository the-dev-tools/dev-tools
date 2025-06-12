import { create, enumFromJson, enumToJson, equals, isEnumJson, Message, MessageInitShape } from '@bufbuild/protobuf';
import { useRouteContext } from '@tanstack/react-router';
import {
  ConnectionLineComponentProps,
  Edge as EdgeCore,
  EdgeLabelRenderer,
  EdgeProps as EdgePropsCore,
  getEdgeCenter,
  getSmoothStepPath,
  useEdgesState,
  useReactFlow,
} from '@xyflow/react';
import { Array, HashMap, Option, pipe, Struct } from 'effect';
import { Ulid } from 'id128';
import { use, useCallback } from 'react';
import { FiX } from 'react-icons/fi';
import { tv } from 'tailwind-variants';
import { useDebouncedCallback } from 'use-debounce';
import {
  EdgeListItem,
  EdgeListItemSchema,
  Handle as HandleKind,
  HandleSchema as HandleKindSchema,
} from '@the-dev-tools/spec/flow/edge/v1/edge_pb';
import { NodeState } from '@the-dev-tools/spec/flow/node/v1/node_pb';
import {
  EdgeCreateEndpoint,
  EdgeDeleteEndpoint,
  EdgeListEndpoint,
  EdgeUpdateEndpoint,
} from '@the-dev-tools/spec/meta/flow/edge/v1/edge.endpoints.ts';
import { Button } from '@the-dev-tools/ui/button';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useQuery } from '~data-client';
import { FlowContext } from './internal';

export interface EdgeData extends Record<string, unknown> {
  state: NodeState;
}
export interface Edge extends EdgeCore<EdgeData> {}
export interface EdgeProps extends EdgePropsCore<Edge> {}

export const Edge = {
  fromDTO: (edge: Message & Omit<EdgeListItem, keyof Message>): Edge => ({
    data: { state: NodeState.UNSPECIFIED },
    id: Ulid.construct(edge.edgeId).toCanonical(),
    source: Ulid.construct(edge.sourceId).toCanonical(),
    sourceHandle: edge.sourceHandle === HandleKind.UNSPECIFIED ? null : enumToJson(HandleKindSchema, edge.sourceHandle),
    target: Ulid.construct(edge.targetId).toCanonical(),
  }),

  toDTO: (_: Partial<Edge>): Omit<EdgeListItem, keyof Message> =>
    pipe(
      create(EdgeListItemSchema, {
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
  const { dataClient } = useRouteContext({ from: '__root__' });

  const { flowId } = use(FlowContext);

  return useCallback(
    async (data: Omit<MessageInitShape<typeof EdgeListItemSchema>, keyof Message>) => {
      const { edgeId } = await dataClient.fetch(EdgeCreateEndpoint, { flowId, ...data });
      return create(EdgeListItemSchema, { edgeId, ...data });
    },
    [dataClient, flowId],
  );
};

const DefaultEdge = ({ data, id, sourcePosition, sourceX, sourceY, targetPosition, targetX, targetY }: EdgeProps) => {
  const { deleteElements } = useReactFlow();

  const [labelX, labelY] = getEdgeCenter({ sourceX, sourceY, targetX, targetY });

  return (
    <>
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

      <EdgeLabelRenderer>
        <div
          className={tw`nodrag nopan pointer-events-auto absolute`}
          style={{ transform: `translate(-50%, -50%) translate(${labelX}px,${labelY}px)` }}
        >
          <Button className={tw`rounded-full p-1`} onPress={() => void deleteElements({ edges: [{ id }] })}>
            <FiX className={tw`size-3 text-red-700`} />
          </Button>
        </div>
      </EdgeLabelRenderer>
    </>
  );
};

export const edgeTypes = {
  default: DefaultEdge,
};

const connectionLineStyles = tv({
  base: tw`fill-none stroke-1 transition-colors`,
  variants: {
    state: {
      [NodeState.CANCELED]: tw`stroke-slate-400`,
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

export const useEdgeStateSynced = () => {
  const { dataClient } = useRouteContext({ from: '__root__' });

  const { flowId, isReadOnly = false } = use(FlowContext);

  const { items: edgesServer } = useQuery(EdgeListEndpoint, { flowId });

  const [edgesClient, setEdgesClient, onEdgesChange] = useEdgesState(edgesServer.map(Edge.fromDTO));

  const sync = useDebouncedCallback(async () => {
    const edgeServerMap = pipe(
      edgesServer.map((_) => {
        const id = Ulid.construct(_.edgeId).toCanonical();
        const value = create(EdgeListItemSchema, Struct.omit(_, '$typeName'));
        return [id, value] as const;
      }),
      HashMap.fromIterable,
    );

    const edgeClientMap = pipe(
      edgesClient.map((_) => {
        const value = create(EdgeListItemSchema, Edge.toDTO(_));
        return [_.id, value] as const;
      }),
      HashMap.fromIterable,
    );

    const changes: Record<string, [string, ReturnType<typeof Edge.toDTO>][]> = pipe(
      HashMap.union(edgeServerMap, edgeClientMap),
      HashMap.entries,
      Array.groupBy(([id]) => {
        const edgeServer = HashMap.get(edgeServerMap, id);
        const edgeClient = HashMap.get(edgeClientMap, id);

        if (Option.isNone(edgeServer)) return 'create';
        if (Option.isNone(edgeClient)) return 'delete';

        return equals(EdgeListItemSchema, edgeServer.value, edgeClient.value) ? 'ignore' : 'update';
      }),
    );

    await pipe(
      changes['create'] ?? [],
      Array.filterMap(([_id, edge]) =>
        pipe(
          Option.liftPredicate(edge, (_) => !_.edgeId.length),
          Option.map((edge) => dataClient.fetch(EdgeCreateEndpoint, edge)),
        ),
      ),
      (_) => Promise.allSettled(_),
    );

    await pipe(
      changes['delete'] ?? [],
      Array.map(([_id, edge]) => dataClient.fetch(EdgeDeleteEndpoint, edge)),
      (_) => Promise.allSettled(_),
    );

    await pipe(
      changes['update'] ?? [],
      Array.map(([_id, edge]) => dataClient.fetch(EdgeUpdateEndpoint, edge)),
      (_) => Promise.allSettled(_),
    );
  }, 500);

  const onEdgesChangeSync: typeof onEdgesChange = (changes) => {
    onEdgesChange(changes);
    if (isReadOnly) return;
    void sync();
  };

  return [edgesClient, setEdgesClient, onEdgesChangeSync] as const;
};
