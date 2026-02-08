import * as XF from '@xyflow/react';
import { Ulid } from 'id128';
import { FiZap } from 'react-icons/fi';
import { PlayIcon } from '@the-dev-tools/ui/icons';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { Handle } from '../handle';
import { SimpleNode } from '../node';

export const ManualStartNode = ({ id, selected }: XF.NodeProps) => {
  const nodeId = Ulid.fromCanonical(id).bytes;

  return (
    <SimpleNode
      className={tw`text-green-500`}
      handles={
        <>
          <div className={tw`absolute top-1/2 -translate-x-full -translate-y-1/2 p-1`}>
            <FiZap className={tw`size-5 text-accent-fg`} />
          </div>

          <Handle nodeId={nodeId} position={XF.Position.Right} type='source' />
        </>
      }
      icon={<PlayIcon />}
      nodeId={nodeId}
      selected={selected}
      title='Manual Start'
    />
  );
};
