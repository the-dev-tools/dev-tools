import { create } from '@bufbuild/protobuf';
import { eq, useLiveQuery } from '@tanstack/react-db';
import * as XF from '@xyflow/react';
import { Ulid } from 'id128';
import { use } from 'react';
import { FiClock } from 'react-icons/fi';
import { NodeWaitSchema } from '@the-dev-tools/spec/buf/api/flow/v1/flow_pb';
import { NodeWaitCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/flow';
import { NumberField } from '@the-dev-tools/ui/number-field';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useApiCollection } from '~/shared/api';
import { pick } from '~/shared/lib';
import { FlowContext } from '../context';
import { Handle } from '../handle';
import { NodeSettingsBody, NodeSettingsProps, NodeTitle, SimpleNode } from '../node';

const defaultNodeWait = create(NodeWaitSchema);

export const WaitNode = ({ id, selected }: XF.NodeProps) => {
  const nodeId = Ulid.fromCanonical(id).bytes;

  return (
    <SimpleNode
      className={tw`w-28 text-amber-500`}
      handles={
        <>
          <Handle nodeId={nodeId} position={XF.Position.Left} type='target' />
          <Handle nodeId={nodeId} position={XF.Position.Right} type='source' />
        </>
      }
      icon={<FiClock />}
      nodeId={nodeId}
      selected={selected}
    >
      <NodeTitle className={tw`text-left`}>Wait</NodeTitle>
    </SimpleNode>
  );
};

export const WaitSettings = ({ nodeId }: NodeSettingsProps) => {
  const collection = useApiCollection(NodeWaitCollectionSchema);

  const data =
    useLiveQuery(
      (_) =>
        _.from({ item: collection })
          .where((_) => eq(_.item.nodeId, nodeId))
          .select((_) => pick(_.item, 'durationMs'))
          .findOne(),
      [collection, nodeId],
    ).data ?? defaultNodeWait;

  const { isReadOnly = false } = use(FlowContext);

  return (
    <NodeSettingsBody nodeId={nodeId} title='Wait'>
      <div className={tw`grid grid-cols-[auto_1fr] gap-x-8 gap-y-5`}>
        <NumberField
          className={tw`contents`}
          groupClassName={tw`w-full justify-self-start`}
          isReadOnly={isReadOnly}
          label='Duration (ms)'
          onChange={(_) => collection.utils.updatePaced({ durationMs: BigInt(_), nodeId })}
          value={Number(data.durationMs)}
        />
      </div>
    </NodeSettingsBody>
  );
};
