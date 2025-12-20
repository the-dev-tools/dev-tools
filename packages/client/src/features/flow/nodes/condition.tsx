import { create } from '@bufbuild/protobuf';
import { eq, useLiveQuery } from '@tanstack/react-db';
import * as XF from '@xyflow/react';
import { Ulid } from 'id128';
import { use, useEffect } from 'react';
import { useForm } from 'react-hook-form';
import { FiX } from 'react-icons/fi';
import { useDebouncedCallback } from 'use-debounce';
import { HandleKind, NodeConditionSchema } from '@the-dev-tools/spec/buf/api/flow/v1/flow_pb';
import { NodeConditionCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/flow';
import { ButtonAsLink } from '@the-dev-tools/ui/button';
import { IfIcon } from '@the-dev-tools/ui/icons';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useApiCollection } from '~/api';
import { ReferenceFieldRHF } from '~/reference';
import { pick } from '~/utils/tanstack-db';
import { FlowContext } from '../context';
import { Handle } from '../handle';
import { NodeBodyNew, NodeExecutionPanel, NodeName, NodePanelProps, NodeStateIndicator, NodeTitle } from '../node';

export const ConditionNode = ({ id, selected }: XF.NodeProps) => {
  const nodeId = Ulid.fromCanonical(id).bytes;

  return (
    <div className={tw`flex flex-col items-center`}>
      <div className={tw`relative`}>
        <NodeBodyNew className={tw`w-48 text-sky-500`} icon={<IfIcon />} nodeId={nodeId} selected={selected}>
          <div className={tw`flex-1`}>
            <NodeTitle className={tw`text-left`}>If</NodeTitle>
            <NodeName className={tw`ml-0 text-left`} nodeId={nodeId} />
          </div>

          <NodeStateIndicator nodeId={nodeId} />
        </NodeBodyNew>

        <Handle nodeId={nodeId} position={XF.Position.Left} type='target' />
        <Handle kind={HandleKind.THEN} nodeId={nodeId} position={XF.Position.Right} type='source' />
        <Handle kind={HandleKind.ELSE} nodeId={nodeId} position={XF.Position.Bottom} type='source' />
      </div>
    </div>
  );
};

export const ConditionPanel = ({ nodeId }: NodePanelProps) => {
  const collection = useApiCollection(NodeConditionCollectionSchema);

  const { condition } =
    useLiveQuery(
      (_) =>
        _.from({ item: collection })
          .where((_) => eq(_.item.nodeId, nodeId))
          .select((_) => pick(_.item, 'condition'))
          .findOne(),
      [collection, nodeId],
    ).data ?? create(NodeConditionSchema);

  const { control, handleSubmit, watch } = useForm({
    resetOptions: { keepDirtyValues: true },
    values: { condition },
  });

  const { isReadOnly = false } = use(FlowContext);

  const update = useDebouncedCallback(async () => {
    await handleSubmit(({ condition }) => void collection.utils.update({ condition, nodeId }))();
  }, 200);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/incompatible-library
    const subscription = watch((_, { type }) => {
      if (type === 'change') void update();
    });
    return () => void subscription.unsubscribe();
  }, [update, watch]);

  return (
    <>
      <div className={tw`sticky top-0 z-10 flex items-center border-b border-slate-200 bg-white px-5 py-2`}>
        <div>
          <div className={tw`text-md leading-5 text-slate-400`}>If Condition</div>
          <div className={tw`text-sm leading-5 font-medium text-slate-800`}>Node Name</div>
        </div>

        <div className={tw`flex-1`} />

        <ButtonAsLink className={tw`p-1`} search={(_) => ({ ..._, node: undefined })} to='.' variant='ghost'>
          <FiX className={tw`size-5 text-slate-500`} />
        </ButtonAsLink>
      </div>

      <div className={tw`m-5`}>
        <ReferenceFieldRHF control={control} name={'condition'} readOnly={isReadOnly} />
      </div>

      <NodeExecutionPanel nodeId={nodeId} />
    </>
  );
};
