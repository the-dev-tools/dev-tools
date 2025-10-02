import { Position } from '@xyflow/react';
import { Match, pipe } from 'effect';
import { twMerge } from 'tailwind-merge';

import { NodeNoOpKind } from '@the-dev-tools/spec/flow/node/v1/node_pb';
import { PlayIcon } from '@the-dev-tools/ui/icons';
import { tw } from '@the-dev-tools/ui/tailwind-literal';

import { Handle } from '../internal';
import { NodeProps } from '../node';
import { CreateNode } from './create';

export const NoOpNode = (props: NodeProps) => {
  const kind = props.data.noOp;

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

      {kind !== NodeNoOpKind.START && <Handle isConnectable={false} position={Position.Top} type='target' />}
      <Handle position={Position.Bottom} type='source' />
    </>
  );
};
