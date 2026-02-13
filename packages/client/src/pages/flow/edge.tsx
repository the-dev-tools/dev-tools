import { create } from '@bufbuild/protobuf';
import {
  createCollection,
  createLiveQueryCollection,
  eq,
  localOnlyCollectionOptions,
  useLiveQuery,
} from '@tanstack/react-db';
import * as XF from '@xyflow/react';
import { Array, pipe, Schema } from 'effect';
import { Ulid } from 'id128';
import { useContext } from 'react';
import { FiX } from 'react-icons/fi';
import { tv } from 'tailwind-variants';
import { EdgeSchema, FlowItemState, HandleKind } from '@the-dev-tools/spec/buf/api/flow/v1/flow_pb';
import { EdgeCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/flow';
import { Button } from '@the-dev-tools/ui/button';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useApiCollection } from '~/shared/api';
import { pick } from '~/shared/lib';
import { FlowContext } from './context';
import { HandleHalo } from './handle';

class EdgeClient extends Schema.Class<EdgeClient>('EdgeClient')({
  edgeId: Schema.Uint8ArrayFromSelf,
  selected: pipe(Schema.Boolean, Schema.optionalWith({ default: () => false })),
}) {}

export const edgeClientCollection = createCollection(
  localOnlyCollectionOptions({
    getKey: (_) => Ulid.construct(_.edgeId).toCanonical(),
    schema: Schema.standardSchemaV1(EdgeClient),
  }),
);

export const useEdgeState = () => {
  const { flowId } = useContext(FlowContext);

  const edgeServerCollection = useApiCollection(EdgeCollectionSchema);

  const items = useLiveQuery(
    (_) => {
      const server = _.from({ server: edgeServerCollection })
        .where((_) => eq(_.server.flowId, flowId))
        .fn.select((_) => ({ ..._.server, edgeId: Ulid.construct(_.server.edgeId).toCanonical() }));

      // This is suboptimal, but without creating a live query the data does not resolve sometimes for some reason
      const client = createLiveQueryCollection((_) =>
        _.from({ client: edgeClientCollection }).fn.select((_) => ({
          // eslint-disable-next-line @typescript-eslint/no-misused-spread
          ..._.client,
          edgeId: Ulid.construct(_.client.edgeId).toCanonical(),
        })),
      );

      return _.from({ server })
        .join({ client }, (_) => eq(_.server.edgeId, _.client.edgeId))
        .fn.select(
          (_): XF.Edge => ({
            id: _.server.edgeId,
            selected: _.client?.selected ?? false,
            source: Ulid.construct(_.server.sourceId).toCanonical(),
            sourceHandle: _.server.sourceHandle === HandleKind.UNSPECIFIED ? null : _.server.sourceHandle.toString(),
            target: Ulid.construct(_.server.targetId).toCanonical(),
          }),
        );
    },
    [edgeServerCollection, flowId],
  ).data;

  const onChange: XF.OnEdgesChange = (_) => {
    const changes = Array.groupBy(_, (_) => _.type) as { [T in XF.EdgeChange as T['type']]?: T[] };

    changes.select?.forEach(({ id, selected }) => {
      if (!edgeClientCollection.has(id)) edgeClientCollection.insert({ edgeId: Ulid.fromCanonical(id).bytes });
      edgeClientCollection.update(id, (_) => (_.selected = selected));
    });

    if (changes.remove?.length) {
      pipe(
        changes.remove.map((_) => edgeServerCollection.utils.getKeyObject({ edgeId: Ulid.fromCanonical(_.id).bytes })),
        edgeServerCollection.utils.delete,
      );

      pipe(
        changes.remove.map((_) => _.id),
        edgeClientCollection.delete,
      );
    }
  };

  return { edges: items, onEdgesChange: onChange };
};

const DefaultEdge = ({ id, sourcePosition, sourceX, sourceY, targetPosition, targetX, targetY }: XF.EdgeProps) => {
  const { deleteElements } = XF.useReactFlow();

  const { labelX, labelY } = getConnectionPath({ sourcePosition, sourceX, sourceY, targetPosition, targetX, targetY });

  const edgeCollection = useApiCollection(EdgeCollectionSchema);

  const { state } =
    useLiveQuery(
      (_) =>
        _.from({ item: edgeCollection })
          .where((_) => eq(_.item.edgeId, Ulid.fromCanonical(id).bytes))
          .select((_) => pick(_.item, 'state'))
          .findOne(),
      [edgeCollection, id],
    ).data ?? create(EdgeSchema);

  return (
    <>
      <ConnectionLine
        connected
        fromPosition={sourcePosition}
        fromX={sourceX}
        fromY={sourceY}
        state={state}
        toPosition={targetPosition}
        toX={targetX}
        toY={targetY}
      />

      <XF.EdgeLabelRenderer>
        <div className={tw`absolute -z-10 size-0`} style={{ transform: `translate(${sourceX}px,${sourceY}px)` }}>
          <HandleHalo />
        </div>

        <div className={tw`absolute -z-10 size-0`} style={{ transform: `translate(${targetX}px,${targetY}px)` }}>
          <HandleHalo />
        </div>

        <div
          // eslint-disable-next-line better-tailwindcss/no-unregistered-classes
          className={tw`nodrag nopan pointer-events-auto absolute`}
          style={{ transform: `translate(-50%, -50%) translate(${labelX}px,${labelY}px)` }}
        >
          <Button
            className={tw`rounded-full border-on-neutral p-1`}
            onPress={() => void deleteElements({ edges: [{ id }] })}
          >
            <FiX className={tw`size-3 text-danger-high`} />
          </Button>
        </div>
      </XF.EdgeLabelRenderer>
    </>
  );
};

export const edgeTypes: XF.EdgeTypes = {
  default: DefaultEdge,
};

const connectionLineStyles = tv({
  base: tw`fill-none stroke-1 transition-colors`,
  variants: {
    state: {
      [FlowItemState.CANCELED]: tw`stroke-neutral-higher`,
      [FlowItemState.FAILURE]: tw`stroke-danger`,
      [FlowItemState.RUNNING]: tw`stroke-accent`,
      [FlowItemState.SUCCESS]: tw`stroke-success`,
      [FlowItemState.UNSPECIFIED]: tw`stroke-on-neutral`,
    } satisfies Record<FlowItemState, string>,
  },
});

interface ConnectionLineProps extends Pick<
  XF.ConnectionLineComponentProps,
  'fromPosition' | 'fromX' | 'fromY' | 'toPosition' | 'toX' | 'toY'
> {
  connected?: boolean;
  state?: FlowItemState;
}

export const ConnectionLine = ({
  connected = false,
  fromPosition,
  fromX,
  fromY,
  state = FlowItemState.UNSPECIFIED,
  toPosition,
  toX,
  toY,
}: ConnectionLineProps) => {
  const { path } = getConnectionPath({
    sourcePosition: fromPosition,
    sourceX: fromX,
    sourceY: fromY,
    targetPosition: toPosition,
    targetX: toX,
    targetY: toY,
  });

  return <path className={connectionLineStyles({ state })} d={path} strokeDasharray={connected ? undefined : 4} />;
};

const getConnectionPath = (params: XF.GetBezierPathParams) => {
  const [path, labelX, labelY, offsetX, offsetY] = XF.getBezierPath({ curvature: 1, ...params });
  return { labelX, labelY, offsetX, offsetY, path };
};
