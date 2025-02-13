import { ToOptions } from '@tanstack/react-router';
import { Position } from '@xyflow/react';
import { useEffect } from 'react';
import { Controller, useForm } from 'react-hook-form';
import { FiX } from 'react-icons/fi';
import { useDebouncedCallback } from 'use-debounce';

import { useConnectMutation } from '@the-dev-tools/api/connect-query';
import { ErrorHandling } from '@the-dev-tools/spec/flow/node/v1/node_pb';
import { nodeUpdate } from '@the-dev-tools/spec/flow/node/v1/node-NodeService_connectquery';
import { ButtonAsLink } from '@the-dev-tools/ui/button';
import { FieldLabel } from '@the-dev-tools/ui/field';
import { CheckListAltIcon, ForIcon } from '@the-dev-tools/ui/icons';
import { ListBoxItem } from '@the-dev-tools/ui/list-box';
import { SelectRHF } from '@the-dev-tools/ui/select';
import { tw } from '@the-dev-tools/ui/tailwind-literal';

import { ConditionField } from '../../condition';
import { ReferenceField } from '../../reference';
import { Handle, HandleKindJson } from '../internal';
import { NodeBase, NodePanelProps, NodeProps } from '../node';

export const ForEachNode = ({ id }: NodeProps) => (
  <>
    <NodeBase id={id} Icon={ForIcon} title='For Each Loop'>
      <div className={tw`rounded-md border border-slate-200 bg-white shadow-sm`}>
        <ButtonAsLink
          className={tw`flex w-full justify-start gap-1.5 rounded-md border border-slate-200 px-2 py-3 text-xs font-medium leading-4 tracking-tight text-slate-800 shadow-sm`}
          href={{
            to: '.',
            search: { selectedNodeIdCan: id } satisfies ToOptions['search'],
          }}
        >
          <CheckListAltIcon className={tw`size-5 text-slate-500`} />
          <span>Edit Loop</span>
        </ButtonAsLink>
      </div>
    </NodeBase>

    <Handle type='target' position={Position.Top} />
    <Handle
      type='source'
      position={Position.Bottom}
      id={'HANDLE_LOOP' satisfies HandleKindJson}
      isConnectable={false}
    />
    <Handle
      type='source'
      position={Position.Bottom}
      id={'HANDLE_THEN' satisfies HandleKindJson}
      isConnectable={false}
    />
  </>
);

export const ForEachPanel = ({ node: { nodeId, forEach } }: NodePanelProps) => {
  const { control, handleSubmit, watch } = useForm({ values: forEach! });

  const nodeUpdateMutation = useConnectMutation(nodeUpdate);

  const update = useDebouncedCallback(async () => {
    await handleSubmit(async (forEach) => {
      await nodeUpdateMutation.mutateAsync({ nodeId, forEach });
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
          <div className={tw`text-md leading-5 text-slate-400`}>For Each Loop</div>
          <div className={tw`text-sm font-medium leading-5 text-slate-800`}>Node Name</div>
        </div>

        <div className={tw`flex-1`} />

        <ButtonAsLink variant='ghost' className={tw`p-1`} href={{ to: '.' }}>
          <FiX className={tw`size-5 text-slate-500`} />
        </ButtonAsLink>
      </div>

      <div className={tw`m-5 grid grid-cols-[auto_1fr] gap-x-8 gap-y-5`}>
        <FieldLabel>Array to Loop</FieldLabel>
        <Controller
          control={control}
          name='path'
          defaultValue={[]}
          render={({ field }) => (
            <ReferenceField
              path={field.value}
              onSelect={field.onChange}
              buttonClassName={tw`min-w-[30%] justify-self-start`}
            />
          )}
        />

        <ConditionField control={control} path='condition' label='Break If' className={tw`contents`} />

        <SelectRHF
          control={control}
          name='errorHandling'
          label='On Error'
          className={tw`contents`}
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
