import { create } from '@bufbuild/protobuf';
import { eq, useLiveQuery } from '@tanstack/react-db';
import * as XF from '@xyflow/react';
import { Ulid } from 'id128';
import { use } from 'react';
import { ErrorHandling, HandleKind, NodeForSchema } from '@the-dev-tools/spec/buf/api/flow/v1/flow_pb';
import { NodeForCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/flow';
import { FieldLabel } from '@the-dev-tools/ui/field';
import { ForIcon } from '@the-dev-tools/ui/icons';
import { NumberField } from '@the-dev-tools/ui/number-field';
import { Select, SelectItem } from '@the-dev-tools/ui/select';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { ReferenceField } from '~/features/expression';
import { useApiCollection } from '~/shared/api';
import { pick } from '~/shared/lib';
import { FlowContext } from '../context';
import { Handle } from '../handle';
import { NodeSettingsBody, NodeSettingsProps, NodeTitle, SimpleNode } from '../node';

export const ForNode = ({ id, selected }: XF.NodeProps) => {
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
      <NodeTitle className={tw`text-left`}>For</NodeTitle>
    </SimpleNode>
  );
};

export const ForSettings = ({ nodeId }: NodeSettingsProps) => {
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

  const { isReadOnly = false } = use(FlowContext);

  return (
    <NodeSettingsBody nodeId={nodeId} title='For loop'>
      <div className={tw`grid grid-cols-[auto_1fr] gap-x-8 gap-y-5`}>
        <NumberField
          className={tw`contents`}
          groupClassName={tw`w-full justify-self-start`}
          isReadOnly={isReadOnly}
          label='Iterations'
          onChange={(_) => collection.utils.updatePaced({ iterations: _, nodeId })}
          value={data.iterations}
        />

        <FieldLabel>Break If</FieldLabel>
        <ReferenceField
          className={tw`w-full justify-self-start`}
          onChange={(_) => collection.utils.updatePaced({ condition: _, nodeId })}
          readOnly={isReadOnly}
          value={data.condition}
        />

        <Select
          className={tw`contents`}
          isDisabled={isReadOnly}
          label='On Error'
          onChange={(_) =>
            collection.utils.updatePaced({
              errorHandling: typeof _ === 'number' ? _ : ErrorHandling.UNSPECIFIED,
              nodeId,
            })
          }
          triggerClassName={tw`w-full justify-between justify-self-start`}
          value={data.errorHandling}
        >
          <SelectItem id={ErrorHandling.UNSPECIFIED}>Throw</SelectItem>
          <SelectItem id={ErrorHandling.IGNORE}>Ignore</SelectItem>
          <SelectItem id={ErrorHandling.BREAK}>Break</SelectItem>
        </Select>
      </div>
    </NodeSettingsBody>
  );
};
