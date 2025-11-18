import { eq, useLiveQuery } from '@tanstack/react-db';
import * as XF from '@xyflow/react';
import { use, useEffect } from 'react';
import { useForm } from 'react-hook-form';
import { FiX } from 'react-icons/fi';
import { useDebouncedCallback } from 'use-debounce';
import { HandleKind } from '@the-dev-tools/spec/api/flow/v1/flow_pb';
import { NodeConditionCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/flow';
import { ButtonAsLink } from '@the-dev-tools/ui/button';
import { CheckListAltIcon, IfIcon } from '@the-dev-tools/ui/icons';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useApiCollection } from '~/api-new';
import { ReferenceFieldRHF } from '~/reference';
import { pick } from '~/utils/tanstack-db';
import { FlowContext } from '../context';
import { Handle } from '../handle';
import { NodeBody, NodeContainer, NodeExecutionPanel, NodePanelProps } from '../node';

export const ConditionNode = (props: XF.NodeProps) => (
  <NodeContainer
    {...props}
    handles={
      <>
        <Handle position={XF.Position.Top} type='target' />
        <Handle id={HandleKind.THEN.toString()} isConnectable={false} position={XF.Position.Bottom} type='source' />
        <Handle id={HandleKind.ELSE.toString()} isConnectable={false} position={XF.Position.Bottom} type='source' />
      </>
    }
  >
    <NodeBody {...props} Icon={IfIcon}>
      <div className={tw`rounded-md border border-slate-200 bg-white shadow-xs`}>
        <div
          className={tw`
            flex justify-start gap-2 rounded-md border border-slate-200 p-3 text-xs leading-5 font-medium tracking-tight
            text-slate-800 shadow-xs
          `}
        >
          <CheckListAltIcon className={tw`size-5 text-slate-500`} />
          <span>Edit Condition</span>
        </div>
      </div>
    </NodeBody>
  </NodeContainer>
);

export const ConditionPanel = ({ nodeId }: NodePanelProps) => {
  const collection = useApiCollection(NodeConditionCollectionSchema);

  const { condition } = useLiveQuery(
    (_) =>
      _.from({ item: collection })
        .where((_) => eq(_.item.nodeId, nodeId))
        .select((_) => pick(_.item, 'condition'))
        .findOne(),
    [collection, nodeId],
  ).data!;

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
