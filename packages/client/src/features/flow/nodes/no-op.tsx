import { eq, useLiveQuery } from '@tanstack/react-db';
import * as XF from '@xyflow/react';
import { Match, pipe } from 'effect';
import { Ulid } from 'id128';
import { twMerge } from 'tailwind-merge';
import { NodeNoOpKind } from '@the-dev-tools/spec/api/flow/v1/flow_pb';
import { NodeNoOpCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/flow';
import { PlayIcon } from '@the-dev-tools/ui/icons';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useApiCollection } from '~/api';
import { pick } from '~/utils/tanstack-db';
import { Handle } from '../handle';
import { CreateNode } from './create';

export const NoOpNode = (props: XF.NodeProps) => {
  const collection = useApiCollection(NodeNoOpCollectionSchema);

  const { kind } = useLiveQuery(
    (_) =>
      _.from({ item: collection })
        .where((_) => eq(_.item.nodeId, Ulid.fromCanonical(props.id).bytes))
        .select((_) => pick(_.item, 'kind'))
        .findOne(),
    [collection, props.id],
  ).data!;

  if (kind === NodeNoOpKind.CREATE) return <CreateNode {...props} />;

  return (
    <>
      <div
        className={twMerge(
          tw`flex items-center gap-2 rounded-md bg-slate-800 px-4 text-white shadow-xs transition-colors`,
          props.selected && tw`bg-slate-600`,
        )}
      >
        {kind === NodeNoOpKind.START && (
          <>
            <PlayIcon className={tw`-ml-2 size-4`} />
            <div className={tw`w-px self-stretch bg-slate-700`} />
          </>
        )}

        <span className={tw`flex-1 py-1 text-xs leading-5 font-medium`}>
          {pipe(
            Match.value(kind),
            Match.when(NodeNoOpKind.START, () => 'Start'),
            Match.when(NodeNoOpKind.THEN, () => 'Then'),
            Match.when(NodeNoOpKind.ELSE, () => 'Else'),
            Match.when(NodeNoOpKind.LOOP, () => 'Loop'),
            Match.orElseAbsurd,
          )}
        </span>
      </div>

      {kind !== NodeNoOpKind.START && <Handle isConnectable={false} position={XF.Position.Top} type='target' />}
      <Handle position={XF.Position.Bottom} type='source' />
    </>
  );
};
