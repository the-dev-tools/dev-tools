import { create } from '@bufbuild/protobuf';
import { eq, useLiveQuery } from '@tanstack/react-db';
import * as XF from '@xyflow/react';
import { Ulid } from 'id128';
import { use, useEffect } from 'react';
import { useForm } from 'react-hook-form';
import { useDebouncedCallback } from 'use-debounce';
import { HandleKind, NodeConditionSchema } from '@the-dev-tools/spec/buf/api/flow/v1/flow_pb';
import { NodeConditionCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/flow';
import { IfIcon } from '@the-dev-tools/ui/icons';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useApiCollection } from '~/api';
import { ReferenceFieldRHF } from '~/reference';
import { pick } from '~/utils/tanstack-db';
import { FlowContext } from '../context';
import { Handle } from '../handle';
import { NodeSettingsBody, NodeSettingsProps, NodeTitle, SimpleNode } from '../node';

export const ConditionNode = ({ id, selected }: XF.NodeProps) => {
  const nodeId = Ulid.fromCanonical(id).bytes;

  return (
    <SimpleNode
      className={tw`w-28 text-sky-500`}
      handles={
        <>
          <Handle nodeId={nodeId} position={XF.Position.Left} type='target' />
          <Handle kind={HandleKind.THEN} nodeId={nodeId} position={XF.Position.Right} type='source' />
          <Handle kind={HandleKind.ELSE} nodeId={nodeId} position={XF.Position.Bottom} type='source' />
        </>
      }
      icon={<IfIcon />}
      nodeId={nodeId}
      selected={selected}
    >
      <NodeTitle className={tw`text-left`}>If</NodeTitle>
    </SimpleNode>
  );
};

export const ConditionSettings = ({ nodeId }: NodeSettingsProps) => {
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
    <NodeSettingsBody nodeId={nodeId} title='If'>
      <ReferenceFieldRHF control={control} name={'condition'} readOnly={isReadOnly} />
    </NodeSettingsBody>
  );
};
