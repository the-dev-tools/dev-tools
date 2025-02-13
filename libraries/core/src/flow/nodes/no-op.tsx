import { Position } from '@xyflow/react';
import { Match, pipe } from 'effect';

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
      <div className={tw`flex items-center gap-2 rounded-md bg-slate-800 px-4 text-white shadow-sm`}>
        {kind === NodeNoOpKind.START && (
          <>
            <PlayIcon className={tw`-ml-2 size-4`} />
            <div className={tw`w-px self-stretch bg-slate-700`} />
          </>
        )}

        <span className={tw`flex-1 py-1 text-xs font-medium leading-5`}>
          {pipe(
            Match.value(kind),
            Match.when(NodeNoOpKind.START, () => 'Manual start'),
            Match.when(NodeNoOpKind.THEN, () => 'Then'),
            Match.when(NodeNoOpKind.ELSE, () => 'Else'),
            Match.when(NodeNoOpKind.LOOP, () => 'Loop'),
            Match.orElseAbsurd,
          )}
        </span>
      </div>

      {kind !== NodeNoOpKind.START && <Handle type='target' position={Position.Top} isConnectable={false} />}
      <Handle type='source' position={Position.Bottom} />
    </>
  );
};
