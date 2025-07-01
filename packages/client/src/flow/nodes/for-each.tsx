import { useRouteContext } from '@tanstack/react-router';
import { Position } from '@xyflow/react';
import { use, useEffect } from 'react';
import { useForm } from 'react-hook-form';
import { FiX } from 'react-icons/fi';
import { useDebouncedCallback } from 'use-debounce';
import { ErrorHandling } from '@the-dev-tools/spec/flow/node/v1/node_pb';
import { NodeUpdateEndpoint } from '@the-dev-tools/spec/meta/flow/node/v1/node.endpoints.ts';
import { ButtonAsLink } from '@the-dev-tools/ui/button';
import { FieldLabel } from '@the-dev-tools/ui/field';
import { CheckListAltIcon, ForIcon } from '@the-dev-tools/ui/icons';
import { ListBoxItem } from '@the-dev-tools/ui/list-box';
import { SelectRHF } from '@the-dev-tools/ui/select';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { ReferenceFieldRHF } from '~reference';
import { ConditionField } from '../../condition';
import { FlowContext, Handle, HandleKindJson } from '../internal';
import { NodeBody, NodeContainer, NodePanelProps, NodeProps } from '../node';

export const ForEachNode = (props: NodeProps) => (
  <NodeContainer
    {...props}
    handles={
      <>
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
    }
  >
    <ForEachNodeBody {...props} />
  </NodeContainer>
);

const ForEachNodeBody = (props: NodeProps) => {
  const { id } = props;

  return (
    <NodeBody {...props} Icon={ForIcon}>
      <div className={tw`rounded-md border border-slate-200 bg-white shadow-xs`}>
        <ButtonAsLink
          className={tw`
            flex w-full justify-start gap-1.5 rounded-md border border-slate-200 px-2 py-3 text-xs leading-4 font-medium
            tracking-tight text-slate-800 shadow-xs
          `}
          search={(_) => ({ ..._, node: id })}
          to='.'
        >
          <CheckListAltIcon className={tw`size-5 text-slate-500`} />
          <span>Edit Loop</span>
        </ButtonAsLink>
      </div>
    </NodeBody>
  );
};

export const ForEachPanel = ({ node: { forEach, nodeId } }: NodePanelProps) => {
  const { dataClient } = useRouteContext({ from: '__root__' });

  const { control, handleSubmit, watch } = useForm({
    resetOptions: { keepDirtyValues: true },
    values: forEach!,
  });

  const { isReadOnly = false } = use(FlowContext);

  const update = useDebouncedCallback(async () => {
    await handleSubmit(async (forEach) => {
      await dataClient.fetch(NodeUpdateEndpoint, { forEach, nodeId });
    })();
  }, 200);

  useEffect(() => {
    const subscription = watch((_, { type }) => {
      if (type === 'change') void update();
    });
    return () => void subscription.unsubscribe();
  }, [update, watch]);

  return (
    <>
      <div className={tw`sticky top-0 z-10 flex items-center border-b border-slate-200 bg-white px-5 py-2`}>
        <div>
          <div className={tw`text-md leading-5 text-slate-400`}>For Each Loop</div>
          <div className={tw`text-sm leading-5 font-medium text-slate-800`}>Node Name</div>
        </div>

        <div className={tw`flex-1`} />

        <ButtonAsLink className={tw`p-1`} search={(_) => ({ ..._, node: undefined })} to='.' variant='ghost'>
          <FiX className={tw`size-5 text-slate-500`} />
        </ButtonAsLink>
      </div>

      <div className={tw`m-5 grid grid-cols-[auto_1fr] gap-x-8 gap-y-5`}>
        <FieldLabel>Array to Loop</FieldLabel>
        <ReferenceFieldRHF
          className={tw`min-w-[30%] justify-self-start`}
          control={control}
          name='path'
          readOnly={isReadOnly}
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
          isDisabled={isReadOnly}
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
