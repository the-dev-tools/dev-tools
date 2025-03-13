import { Position } from '@xyflow/react';
import { use, useEffect } from 'react';
import { useForm } from 'react-hook-form';
import { FiPlus, FiX } from 'react-icons/fi';
import { useDebouncedCallback } from 'use-debounce';

import { useConnectMutation } from '@the-dev-tools/api/connect-query';
import { nodeUpdate } from '@the-dev-tools/spec/flow/node/v1/node-NodeService_connectquery';
import { ButtonAsLink } from '@the-dev-tools/ui/button';
import { CheckListAltIcon, IfIcon } from '@the-dev-tools/ui/icons';
import { tw } from '@the-dev-tools/ui/tailwind-literal';

import { ConditionField } from '../../condition';
import { FlowContext, Handle, HandleKindJson } from '../internal';
import { FlowSearch } from '../layout';
import { NodeBase, NodePanelProps, NodeProps } from '../node';

export const ConditionNode = (props: NodeProps) => {
  const { id, data } = props;
  const { condition } = data.condition!;

  return (
    <>
      <NodeBase {...props} Icon={IfIcon}>
        <div className={tw`rounded-md border border-slate-200 bg-white shadow-sm`}>
          {condition ? (
            <div
              className={tw`flex justify-start gap-2 rounded-md border border-slate-200 p-3 text-xs font-medium leading-5 tracking-tight text-slate-800 shadow-sm`}
            >
              <CheckListAltIcon className={tw`size-5 text-slate-500`} />
              <span>Edit Condition</span>
            </div>
          ) : (
            <ButtonAsLink
              className={tw`flex w-full justify-start gap-1.5 rounded-md border border-slate-200 px-2 py-3 text-xs font-medium leading-4 tracking-tight text-violet-600 shadow-sm`}
              href={{ to: '.', search: (_: Partial<FlowSearch>) => ({ ..._, node: id }) }}
            >
              <FiPlus className={tw`size-4`} />
              <span>Setup Condition</span>
            </ButtonAsLink>
          )}
        </div>
      </NodeBase>

      <Handle type='target' position={Position.Top} />
      <Handle
        type='source'
        position={Position.Bottom}
        id={'HANDLE_THEN' satisfies HandleKindJson}
        isConnectable={false}
      />
      <Handle
        type='source'
        position={Position.Bottom}
        id={'HANDLE_ELSE' satisfies HandleKindJson}
        isConnectable={false}
      />
    </>
  );
};

export const ConditionPanel = ({ node: { nodeId, condition } }: NodePanelProps) => {
  const { control, handleSubmit, watch } = useForm({ values: condition! });
  const { isReadOnly = false } = use(FlowContext);

  const nodeUpdateMutation = useConnectMutation(nodeUpdate);

  const update = useDebouncedCallback(async () => {
    await handleSubmit(async (condition) => {
      await nodeUpdateMutation.mutateAsync({ nodeId, condition });
    })();
  }, 200);

  useEffect(() => {
    const subscription = watch(() => void update());
    return () => void subscription.unsubscribe();
  }, [update, watch]);

  return (
    <>
      <div className={tw`sticky top-0 z-10 flex items-center border-b border-slate-200 bg-white px-5 py-2`}>
        <div>
          <div className={tw`text-md leading-5 text-slate-400`}>If Condition</div>
          <div className={tw`text-sm font-medium leading-5 text-slate-800`}>Node Name</div>
        </div>

        <div className={tw`flex-1`} />

        <ButtonAsLink
          variant='ghost'
          className={tw`p-1`}
          href={{ to: '.', search: (_: Partial<FlowSearch>) => ({ ..._, node: undefined }) }}
        >
          <FiX className={tw`size-5 text-slate-500`} />
        </ButtonAsLink>
      </div>

      <div className={tw`m-5`}>
        <ConditionField control={control} path='condition' isReadOnly={isReadOnly} />
      </div>
    </>
  );
};
