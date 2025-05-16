import { useTransport } from '@connectrpc/connect-query';
import { useController } from '@data-client/react';
import { Position } from '@xyflow/react';
import { use, useEffect } from 'react';
import { useForm } from 'react-hook-form';
import { FiPlus, FiX } from 'react-icons/fi';
import { useDebouncedCallback } from 'use-debounce';

import { NodeUpdateEndpoint } from '@the-dev-tools/spec/meta/flow/node/v1/node.endpoints.ts';
import { ButtonAsLink } from '@the-dev-tools/ui/button';
import { CheckListAltIcon, IfIcon } from '@the-dev-tools/ui/icons';
import { tw } from '@the-dev-tools/ui/tailwind-literal';

import { ConditionField } from '../../condition';
import { FlowContext, Handle, HandleKindJson } from '../internal';
import { FlowSearch } from '../layout';
import { NodeBase, NodePanelProps, NodeProps } from '../node';

export const ConditionNode = (props: NodeProps) => {
  const { data, id } = props;
  const { condition } = data.condition!;

  return (
    <>
      <NodeBase {...props} Icon={IfIcon}>
        <div className={tw`shadow-xs rounded-md border border-slate-200 bg-white`}>
          {condition ? (
            <div
              className={tw`shadow-xs flex justify-start gap-2 rounded-md border border-slate-200 p-3 text-xs font-medium leading-5 tracking-tight text-slate-800`}
            >
              <CheckListAltIcon className={tw`size-5 text-slate-500`} />
              <span>Edit Condition</span>
            </div>
          ) : (
            <ButtonAsLink
              className={tw`shadow-xs flex w-full justify-start gap-1.5 rounded-md border border-slate-200 px-2 py-3 text-xs font-medium leading-4 tracking-tight text-violet-600`}
              href={{ search: (_: Partial<FlowSearch>) => ({ ..._, node: id }), to: '.' }}
            >
              <FiPlus className={tw`size-4`} />
              <span>Setup Condition</span>
            </ButtonAsLink>
          )}
        </div>
      </NodeBase>

      <Handle position={Position.Top} type='target' />
      <Handle
        id={'HANDLE_THEN' satisfies HandleKindJson}
        isConnectable={false}
        position={Position.Bottom}
        type='source'
      />
      <Handle
        id={'HANDLE_ELSE' satisfies HandleKindJson}
        isConnectable={false}
        position={Position.Bottom}
        type='source'
      />
    </>
  );
};

export const ConditionPanel = ({ node: { condition, nodeId } }: NodePanelProps) => {
  const transport = useTransport();
  const controller = useController();

  const { control, handleSubmit, watch } = useForm({ values: condition! });
  const { isReadOnly = false } = use(FlowContext);

  const update = useDebouncedCallback(async () => {
    await handleSubmit(async (condition) => {
      await controller.fetch(NodeUpdateEndpoint, transport, { condition, nodeId });
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
          className={tw`p-1`}
          href={{ search: (_: Partial<FlowSearch>) => ({ ..._, node: undefined }), to: '.' }}
          variant='ghost'
        >
          <FiX className={tw`size-5 text-slate-500`} />
        </ButtonAsLink>
      </div>

      <div className={tw`m-5`}>
        <ConditionField control={control} isReadOnly={isReadOnly} path='condition' />
      </div>
    </>
  );
};
