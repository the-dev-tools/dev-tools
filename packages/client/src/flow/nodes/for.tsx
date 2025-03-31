import { Position } from '@xyflow/react';
import { use, useEffect } from 'react';
import { useForm } from 'react-hook-form';
import { FiX } from 'react-icons/fi';
import { useDebouncedCallback } from 'use-debounce';

import { ErrorHandling } from '@the-dev-tools/spec/flow/node/v1/node_pb';
import { nodeUpdate } from '@the-dev-tools/spec/flow/node/v1/node-NodeService_connectquery';
import { ButtonAsLink } from '@the-dev-tools/ui/button';
import { CheckListAltIcon, ForIcon } from '@the-dev-tools/ui/icons';
import { ListBoxItem } from '@the-dev-tools/ui/list-box';
import { NumberFieldRHF } from '@the-dev-tools/ui/number-field';
import { SelectRHF } from '@the-dev-tools/ui/select';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useConnectMutation } from '~/api/connect-query';

import { ConditionField } from '../../condition';
import { FlowContext, Handle, HandleKindJson } from '../internal';
import { FlowSearch } from '../layout';
import { NodeBase, NodePanelProps, NodeProps } from '../node';

export const ForNode = (props: NodeProps) => {
  const { id } = props;

  return (
    <>
      <NodeBase {...props} Icon={ForIcon}>
        <div className={tw`shadow-xs rounded-md border border-slate-200 bg-white`}>
          <ButtonAsLink
            className={tw`shadow-xs flex w-full justify-start gap-1.5 rounded-md border border-slate-200 px-2 py-3 text-xs font-medium leading-4 tracking-tight text-slate-800`}
            href={{ search: (_: Partial<FlowSearch>) => ({ ..._, node: id }), to: '.' }}
          >
            <CheckListAltIcon className={tw`size-5 text-slate-500`} />
            <span>Edit Loop</span>
          </ButtonAsLink>
        </div>
      </NodeBase>

      <Handle position={Position.Top} type='target' />
      <Handle
        id={'HANDLE_LOOP' satisfies HandleKindJson}
        isConnectable={false}
        position={Position.Bottom}
        type='source'
      />
      <Handle
        id={'HANDLE_THEN' satisfies HandleKindJson}
        isConnectable={false}
        position={Position.Bottom}
        type='source'
      />
    </>
  );
};

export const ForPanel = ({ node: { for: data, nodeId } }: NodePanelProps) => {
  const { control, handleSubmit, watch } = useForm({ values: data! });
  const { isReadOnly = false } = use(FlowContext);

  const nodeUpdateMutation = useConnectMutation(nodeUpdate);

  const update = useDebouncedCallback(async () => {
    await handleSubmit(async (data) => {
      await nodeUpdateMutation.mutateAsync({ for: data, nodeId });
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
          <div className={tw`text-md leading-5 text-slate-400`}>For Loop</div>
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

      <div className={tw`m-5 grid grid-cols-[auto_1fr] gap-x-8 gap-y-5`}>
        <NumberFieldRHF
          className={tw`contents`}
          control={control}
          groupClassName={tw`min-w-[30%] justify-self-start`}
          isReadOnly={isReadOnly}
          label='Iterations'
          name='iterations'
        />

        <ConditionField
          className={tw`contents`}
          control={control}
          isReadOnly={isReadOnly}
          label='Break If'
          path='condition'
        />

        <SelectRHF
          className={tw`contents`}
          control={control}
          disabled={isReadOnly}
          label='On Error'
          name='errorHandling'
          triggerClassName={tw`min-w-[30%] justify-between justify-self-start`}
        >
          <ListBoxItem id={ErrorHandling.UNSPECIFIED}>Throw</ListBoxItem>
          <ListBoxItem id={ErrorHandling.IGNORE}>Ignore</ListBoxItem>
          <ListBoxItem id={ErrorHandling.BREAK}>Break</ListBoxItem>
        </SelectRHF>
      </div>
    </>
  );
};
