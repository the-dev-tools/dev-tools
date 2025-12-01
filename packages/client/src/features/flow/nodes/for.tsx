import { eq, useLiveQuery } from '@tanstack/react-db';
import * as XF from '@xyflow/react';
import { use, useEffect } from 'react';
import { useForm } from 'react-hook-form';
import { FiX } from 'react-icons/fi';
import { useDebouncedCallback } from 'use-debounce';
import { ErrorHandling, HandleKind } from '@the-dev-tools/spec/api/flow/v1/flow_pb';
import { NodeForCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/flow';
import { ButtonAsLink } from '@the-dev-tools/ui/button';
import { FieldLabel } from '@the-dev-tools/ui/field';
import { CheckListAltIcon, ForIcon } from '@the-dev-tools/ui/icons';
import { NumberFieldRHF } from '@the-dev-tools/ui/number-field';
import { SelectItem, SelectRHF } from '@the-dev-tools/ui/select';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useApiCollection } from '~/api';
import { ReferenceFieldRHF } from '~/reference';
import { pick } from '~/utils/tanstack-db';
import { FlowContext } from '../context';
import { Handle } from '../handle';
import { NodeBody, NodeContainer, NodeExecutionPanel, NodePanelProps } from '../node';

export const ForNode = (props: XF.NodeProps) => (
  <NodeContainer
    {...props}
    handles={
      <>
        <Handle position={XF.Position.Top} type='target' />
        <Handle id={HandleKind.LOOP.toString()} isConnectable={false} position={XF.Position.Bottom} type='source' />
        <Handle id={HandleKind.THEN.toString()} isConnectable={false} position={XF.Position.Bottom} type='source' />
      </>
    }
  >
    <NodeBody {...props} Icon={ForIcon}>
      <div className={tw`rounded-md border border-slate-200 bg-white shadow-xs`}>
        <div
          className={tw`
            flex w-full justify-start gap-1.5 rounded-md border border-slate-200 px-2 py-3 text-xs leading-4 font-medium
            tracking-tight text-slate-800 shadow-xs
          `}
        >
          <CheckListAltIcon className={tw`size-5 text-slate-500`} />
          <span>Edit Loop</span>
        </div>
      </div>
    </NodeBody>
  </NodeContainer>
);

export const ForPanel = ({ nodeId }: NodePanelProps) => {
  const collection = useApiCollection(NodeForCollectionSchema);

  const data = useLiveQuery(
    (_) =>
      _.from({ item: collection })
        .where((_) => eq(_.item.nodeId, nodeId))
        .select((_) => pick(_.item, 'condition', 'errorHandling', 'iterations'))
        .findOne(),
    [collection, nodeId],
  ).data!;

  const { control, handleSubmit, watch } = useForm({
    resetOptions: { keepDirtyValues: true },
    values: data,
  });

  const { isReadOnly = false } = use(FlowContext);

  const update = useDebouncedCallback(async () => {
    await handleSubmit((data) => void collection.utils.update({ nodeId, ...data }))();
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
          <div className={tw`text-md leading-5 text-slate-400`}>For Loop</div>
          <div className={tw`text-sm leading-5 font-medium text-slate-800`}>Node Name</div>
        </div>

        <div className={tw`flex-1`} />

        <ButtonAsLink className={tw`p-1`} search={(_) => ({ ..._, node: undefined })} to='.' variant='ghost'>
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

        <FieldLabel>Break If</FieldLabel>
        <ReferenceFieldRHF
          className={tw`min-w-[30%] justify-self-start`}
          control={control}
          name='condition'
          readOnly={isReadOnly}
        />

        <SelectRHF
          className={tw`contents`}
          control={control}
          disabled={isReadOnly}
          label='On Error'
          name='errorHandling'
          triggerClassName={tw`min-w-[30%] justify-between justify-self-start`}
        >
          <SelectItem id={ErrorHandling.UNSPECIFIED}>Throw</SelectItem>
          <SelectItem id={ErrorHandling.IGNORE}>Ignore</SelectItem>
          <SelectItem id={ErrorHandling.BREAK}>Break</SelectItem>
        </SelectRHF>
      </div>

      <NodeExecutionPanel nodeId={nodeId} />
    </>
  );
};
