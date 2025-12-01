import { eq, useLiveQuery } from '@tanstack/react-db';
import { Handle as HandleCore, HandleProps, useNodeConnections } from '@xyflow/react';
import { Array, Option, pipe } from 'effect';
import { Ulid } from 'id128';
import { tv } from 'tailwind-variants';
import { FlowItemState } from '@the-dev-tools/spec/buf/api/flow/v1/flow_pb';
import { EdgeCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/flow';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useApiCollection } from '~/api';
import { pick } from '~/utils/tanstack-db';

const handleInnerStyles = tv({
  base: tw`pointer-events-none`,
  variants: {
    state: {
      [FlowItemState.CANCELED]: tw`text-slate-400`,
      [FlowItemState.FAILURE]: tw`text-red-600`,
      [FlowItemState.RUNNING]: tw`text-violet-600`,
      [FlowItemState.SUCCESS]: tw`text-green-600`,
      [FlowItemState.UNSPECIFIED]: tw`text-slate-800`,
    } satisfies Record<FlowItemState, string>,
  },
});

export const Handle = (props: HandleProps) => {
  const { id, type } = props;

  const edgeCollection = useApiCollection(EdgeCollectionSchema);

  const edgeId = pipe(
    useNodeConnections({ ...(id && { handleId: id }), handleType: type }),
    Array.head,
    Option.map((_) => Ulid.fromCanonical(_.edgeId).bytes),
    Option.getOrUndefined,
  );

  const state =
    useLiveQuery(
      (_) =>
        _.from({ item: edgeCollection })
          .where((_) => eq(_.item.edgeId, edgeId))
          .select((_) => pick(_.item, 'state'))
          .findOne(),
      [edgeCollection, edgeId],
    ).data?.state ?? FlowItemState.UNSPECIFIED;

  return (
    <HandleCore
      className={tw`-z-10 size-5 overflow-visible rounded-full border-none bg-transparent shadow-xs`}
      {...props}
    >
      <svg className={handleInnerStyles({ state })} viewBox='-10 -10 20 20'>
        <circle className={tw`fill-slate-300`} r={10} />
        <circle className={tw`fill-slate-200`} r={9} />
        <circle className={tw`fill-current`} r={4} />
        {edgeId && <path className={tw`stroke-current stroke-1`} d='M 0 -10 L 0 10' />}
      </svg>
    </HandleCore>
  );
};
