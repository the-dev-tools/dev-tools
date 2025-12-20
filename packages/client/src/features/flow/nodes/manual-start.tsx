import * as XF from '@xyflow/react';
import { Ulid } from 'id128';
import { FiZap } from 'react-icons/fi';
import { PlayIcon } from '@the-dev-tools/ui/icons';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { Handle } from '../handle';
import { NodeBodyNew, NodeStateIndicator, NodeTitle } from '../node';

export const ManualStartNode = ({ id, selected }: XF.NodeProps) => {
  const nodeId = Ulid.fromCanonical(id).bytes;

  return (
    <div className={tw`pointer-events-none flex flex-col items-center`}>
      <div className={tw`pointer-events-auto relative`}>
        <NodeBodyNew className={tw`text-green-500`} icon={<PlayIcon />} nodeId={nodeId} selected={selected} />

        <div className={tw`absolute top-1/2 -translate-x-full -translate-y-1/2 p-1`}>
          <NodeStateIndicator nodeId={nodeId}>
            <FiZap className={tw`size-5 text-violet-600`} />
          </NodeStateIndicator>
        </div>

        <Handle nodeId={nodeId} position={XF.Position.Right} type='source' />
      </div>

      <NodeTitle className={tw`mt-1`}>Manual Start</NodeTitle>
    </div>
  );
};
