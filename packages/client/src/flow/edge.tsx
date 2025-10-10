import { create, enumFromJson, enumToJson, isEnumJson, Message, MessageInitShape } from '@bufbuild/protobuf';
import {
  ConnectionLineComponentProps,
  Edge as EdgeCore,
  EdgeLabelRenderer,
  EdgeProps as EdgePropsCore,
  EdgeTypes as EdgeTypesCore,
  getEdgeCenter,
  getSmoothStepPath,
  useReactFlow,
} from '@xyflow/react';
import { Option, pipe, Struct } from 'effect';
import { Ulid } from 'id128';
import { use, useCallback } from 'react';
import { FiX } from 'react-icons/fi';
import { tv } from 'tailwind-variants';
import { EdgeCreateEndpoint } from '@the-dev-tools/spec/data-client/flow/edge/v1/edge.endpoints.ts';
import {
  EdgeKind,
  EdgeKindJson,
  EdgeKindSchema,
  EdgeListItem,
  EdgeListItemSchema,
  Handle as HandleKind,
  HandleSchema as HandleKindSchema,
} from '@the-dev-tools/spec/flow/edge/v1/edge_pb';
import { NodeState } from '@the-dev-tools/spec/flow/node/v1/node_pb';
import { Button } from '@the-dev-tools/ui/button';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { rootRouteApi } from '~routes';
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
    type: enumToJson(EdgeKindSchema, edge.kind),
  }),

  toDTO: (_: Partial<Edge>): Omit<EdgeListItem, keyof Message> =>
    pipe(
      create(EdgeListItemSchema, {
        edgeId: pipe(
          Option.fromNullable(_.id),
          Option.map((_) => Ulid.fromCanonical(_).bytes),
          Option.getOrUndefined,
        )!,
        kind: isEnumJson(EdgeKindSchema, _.type) ? enumFromJson(EdgeKindSchema, _.type) : EdgeKind.UNSPECIFIED,
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
  const { dataClient } = rootRouteApi.useRouteContext();

  const { flowId } = use(FlowContext);

  return useCallback(
    async (data: Omit<MessageInitShape<typeof EdgeListItemSchema>, keyof Message>) => {
      const { edgeId } = await dataClient.fetch(EdgeCreateEndpoint, { flowId, ...data });
      return create(EdgeListItemSchema, { ...data, edgeId });
    },
    [dataClient, flowId],
  );
};

const DefaultEdge = (props: EdgeProps) => {
  const { id, sourceX, sourceY, targetX, targetY } = props;
  const { deleteElements } = useReactFlow();

  const [labelX, labelY] = getEdgeCenter({ sourceX, sourceY, targetX, targetY });

  return (
    <>
      <NoOpEdge {...props} />

      <EdgeLabelRenderer>
        <div
          // eslint-disable-next-line better-tailwindcss/no-unregistered-classes
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

const NoOpEdge = ({ data, sourcePosition, sourceX, sourceY, targetPosition, targetX, targetY }: EdgeProps) => (
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

export const edgeTypes: Record<EdgeKindJson, EdgeTypesCore[string]> = {
  EDGE_KIND_NO_OP: NoOpEdge,
  EDGE_KIND_UNSPECIFIED: DefaultEdge,
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
