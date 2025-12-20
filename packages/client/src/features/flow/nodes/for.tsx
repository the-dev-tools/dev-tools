import { create } from '@bufbuild/protobuf';
import { eq, useLiveQuery } from '@tanstack/react-db';
import * as XF from '@xyflow/react';
import { Ulid } from 'id128';
import { use, useEffect } from 'react';
import { useForm } from 'react-hook-form';
import { FiX } from 'react-icons/fi';
import { useDebouncedCallback } from 'use-debounce';
import { ErrorHandling, HandleKind, NodeForSchema } from '@the-dev-tools/spec/buf/api/flow/v1/flow_pb';
import { NodeForCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/flow';
import { ButtonAsLink } from '@the-dev-tools/ui/button';
import { FieldLabel } from '@the-dev-tools/ui/field';
import { ForIcon } from '@the-dev-tools/ui/icons';
import { NumberFieldRHF } from '@the-dev-tools/ui/number-field';
import { SelectItem, SelectRHF } from '@the-dev-tools/ui/select';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useApiCollection } from '~/api';
import { ReferenceFieldRHF } from '~/reference';
import { pick } from '~/utils/tanstack-db';
import { FlowContext } from '../context';
import { Handle } from '../handle';
import { NodeBodyNew, NodeExecutionPanel, NodeName, NodePanelProps, NodeStateIndicator, NodeTitle } from '../node';

export const ForNode = ({ id, selected }: XF.NodeProps) => {
  const nodeId = Ulid.fromCanonical(id).bytes;

  return (
    <div className={tw`flex flex-col items-center`}>
      <div className={tw`relative`}>
        <NodeBodyNew className={tw`w-48 text-teal-500`} icon={<ForIcon />} nodeId={nodeId} selected={selected}>
          <div className={tw`flex-1`}>
            <NodeTitle className={tw`text-left`}>For</NodeTitle>
            <NodeName className={tw`ml-0 text-left`} nodeId={nodeId} />
          </div>

          <NodeStateIndicator nodeId={nodeId} />
        </NodeBodyNew>

        <Handle nodeId={nodeId} position={XF.Position.Left} type='target' />
        <Handle kind={HandleKind.THEN} nodeId={nodeId} position={XF.Position.Right} type='source' />
        <Handle kind={HandleKind.LOOP} nodeId={nodeId} position={XF.Position.Bottom} type='source' />
      </div>
    </div>
  );
};

export const ForPanel = ({ nodeId }: NodePanelProps) => {
  const collection = useApiCollection(NodeForCollectionSchema);

  const data =
    useLiveQuery(
      (_) =>
        _.from({ item: collection })
          .where((_) => eq(_.item.nodeId, nodeId))
          .select((_) => pick(_.item, 'condition', 'errorHandling', 'iterations'))
          .findOne(),
      [collection, nodeId],
    ).data ?? create(NodeForSchema);

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
