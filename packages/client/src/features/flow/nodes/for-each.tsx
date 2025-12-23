import { create } from '@bufbuild/protobuf';
import { eq, useLiveQuery } from '@tanstack/react-db';
import * as XF from '@xyflow/react';
import { Ulid } from 'id128';
import { use, useEffect } from 'react';
import { useForm } from 'react-hook-form';
import { useDebouncedCallback } from 'use-debounce';
import { ErrorHandling, HandleKind, NodeForEachSchema } from '@the-dev-tools/spec/buf/api/flow/v1/flow_pb';
import { NodeForEachCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/flow';
import { FieldLabel } from '@the-dev-tools/ui/field';
import { ForIcon } from '@the-dev-tools/ui/icons';
import { SelectItem, SelectRHF } from '@the-dev-tools/ui/select';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useApiCollection } from '~/api';
import { ReferenceFieldRHF } from '~/reference';
import { pick } from '~/utils/tanstack-db';
import { FlowContext } from '../context';
import { Handle } from '../handle';
import { NodeSettingsBody, NodeSettingsProps, NodeTitle, SimpleNode } from '../node';

export const ForEachNode = ({ id, selected }: XF.NodeProps) => {
  const nodeId = Ulid.fromCanonical(id).bytes;

  return (
    <SimpleNode
      className={tw`w-28 text-teal-500`}
      handles={
        <>
          <Handle nodeId={nodeId} position={XF.Position.Left} type='target' />
          <Handle kind={HandleKind.THEN} nodeId={nodeId} position={XF.Position.Right} type='source' />
          <Handle kind={HandleKind.LOOP} nodeId={nodeId} position={XF.Position.Bottom} type='source' />
        </>
      }
      icon={<ForIcon />}
      nodeId={nodeId}
      selected={selected}
    >
      <NodeTitle className={tw`text-left`}>For Each</NodeTitle>
    </SimpleNode>
  );
};

export const ForEachSettings = ({ nodeId }: NodeSettingsProps) => {
  const collection = useApiCollection(NodeForEachCollectionSchema);

  const data =
    useLiveQuery(
      (_) =>
        _.from({ item: collection })
          .where((_) => eq(_.item.nodeId, nodeId))
          .select((_) => pick(_.item, 'condition', 'errorHandling', 'path'))
          .findOne(),
      [collection, nodeId],
    ).data ?? create(NodeForEachSchema);

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
    <NodeSettingsBody nodeId={nodeId} title='For each loop'>
      <div className={tw`grid grid-cols-[auto_1fr] gap-x-8 gap-y-5`}>
        <FieldLabel>Array to Loop</FieldLabel>
        <ReferenceFieldRHF
          className={tw`min-w-[30%] justify-self-start`}
          control={control}
          name='path'
          readOnly={isReadOnly}
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
          isDisabled={isReadOnly}
          label='On Error'
          name='errorHandling'
          triggerClassName={tw`min-w-[30%] justify-between justify-self-start`}
        >
          <SelectItem id={ErrorHandling.UNSPECIFIED}>Throw</SelectItem>
          <SelectItem id={ErrorHandling.IGNORE}>Ignore</SelectItem>
          <SelectItem id={ErrorHandling.BREAK}>Break</SelectItem>
        </SelectRHF>
      </div>
    </NodeSettingsBody>
  );
};
