import { eq, useLiveQuery } from '@tanstack/react-db';
import * as XF from '@xyflow/react';
import { Array, HashSet, pipe } from 'effect';
import { Ulid } from 'id128';
import { useContext, useState } from 'react';
import { FiX } from 'react-icons/fi';
import { tv } from 'tailwind-variants';
import { EdgeKind, FlowItemState, HandleKind } from '@the-dev-tools/spec/buf/api/flow/v1/flow_pb';
import { EdgeCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/flow';
import { Button } from '@the-dev-tools/ui/button';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useApiCollection } from '~/api';
import { pick } from '~/utils/tanstack-db';
import { FlowContext } from './context';

export const useEdgeState = () => {
  const { flowId } = useContext(FlowContext);

  const collection = useApiCollection(EdgeCollectionSchema);

  const [selection, setSelection] = useState(HashSet.empty<string>());

  const items = pipe(
    useLiveQuery(
      (_) =>
        _.from({ item: collection })
          .where((_) => eq(_.item.flowId, flowId))
          .select((_) => pick(_.item, 'edgeId', 'sourceId', 'sourceHandle', 'targetId', 'kind')),
      [collection, flowId],
    ).data,
    Array.map((_): XF.Edge => {
      const id = Ulid.construct(_.edgeId).toCanonical();
      return {
        id,
        selected: HashSet.has(selection, id),
        source: Ulid.construct(_.sourceId).toCanonical(),
        sourceHandle: _.sourceHandle === HandleKind.UNSPECIFIED ? null : _.sourceHandle.toString(),
        target: Ulid.construct(_.targetId).toCanonical(),
        type: _.kind.toString(),
      };
    }),
  );

  const onChange: XF.OnEdgesChange = (_) => {
    const changes = Array.groupBy(_, (_) => _.type) as { [T in XF.EdgeChange as T['type']]?: T[] };

    setSelection(
      HashSet.mutate(
        (selection) =>
          void changes.select?.forEach((_) => {
            if (_.selected) HashSet.add(selection, _.id);
            else HashSet.remove(selection, _.id);
          }),
      ),
    );

    if (changes.add?.length) console.log('add', changes.add);
    //  pipe(changes.add, Array.map(_ => {}))

    if (changes.remove?.length)
      pipe(
        changes.remove,
        Array.map((_) => collection.utils.getKeyObject({ edgeId: Ulid.fromCanonical(_.id).bytes })),
        (_) => collection.utils.delete(_),
      );
  };

  return { edges: items, onEdgesChange: onChange };
};

const DefaultEdge = (props: XF.EdgeProps) => {
  const { id, sourceX, sourceY, targetX, targetY } = props;
  const { deleteElements } = XF.useReactFlow();

  const [labelX, labelY] = XF.getEdgeCenter({ sourceX, sourceY, targetX, targetY });

  return (
    <>
      <NoOpEdge {...props} />

      <XF.EdgeLabelRenderer>
        <div
          // eslint-disable-next-line better-tailwindcss/no-unregistered-classes
          className={tw`nodrag nopan pointer-events-auto absolute`}
          style={{ transform: `translate(-50%, -50%) translate(${labelX}px,${labelY}px)` }}
        >
          <Button className={tw`rounded-full p-1`} onPress={() => void deleteElements({ edges: [{ id }] })}>
            <FiX className={tw`size-3 text-red-700`} />
          </Button>
        </div>
      </XF.EdgeLabelRenderer>
    </>
  );
};

const NoOpEdge = ({ id, sourcePosition, sourceX, sourceY, targetPosition, targetX, targetY }: XF.EdgeProps) => {
  const edgeCollection = useApiCollection(EdgeCollectionSchema);

  const state =
    useLiveQuery(
      (_) =>
        _.from({ item: edgeCollection })
          .where((_) => eq(_.item.edgeId, Ulid.fromCanonical(id).bytes))
          .select((_) => pick(_.item, 'state'))
          .findOne(),
      [edgeCollection, id],
    ).data?.state ?? FlowItemState.UNSPECIFIED;

  return (
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
  );
};

export const edgeTypes: XF.EdgeTypes = {
  [EdgeKind.NO_OP]: NoOpEdge,
  [EdgeKind.UNSPECIFIED]: DefaultEdge,
};

const connectionLineStyles = tv({
  base: tw`fill-none stroke-1 transition-colors`,
  variants: {
    state: {
      [FlowItemState.CANCELED]: tw`stroke-slate-400`,
      [FlowItemState.FAILURE]: tw`stroke-red-600`,
      [FlowItemState.RUNNING]: tw`stroke-violet-600`,
      [FlowItemState.SUCCESS]: tw`stroke-green-600`,
      [FlowItemState.UNSPECIFIED]: tw`stroke-slate-800`,
    } satisfies Record<FlowItemState, string>,
  },
});

interface ConnectionLineProps
  extends Pick<XF.ConnectionLineComponentProps, 'fromPosition' | 'fromX' | 'fromY' | 'toPosition' | 'toX' | 'toY'> {
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
  const [edgePath] = XF.getSmoothStepPath({
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
