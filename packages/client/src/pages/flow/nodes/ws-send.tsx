import { create } from '@bufbuild/protobuf';
import { eq, useLiveQuery } from '@tanstack/react-db';
import * as XF from '@xyflow/react';
import { Ulid } from 'id128';
import { use } from 'react';
import { FiSend } from 'react-icons/fi';
import { NodeKind, NodeWsSendSchema } from '@the-dev-tools/spec/buf/api/flow/v1/flow_pb';
import { NodeCollectionSchema, NodeWsSendCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/flow';
import { FieldLabel } from '@the-dev-tools/ui/field';
import { Select, SelectItem } from '@the-dev-tools/ui/select';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { ReferenceField } from '~/features/expression';
import { useApiCollection } from '~/shared/api';
import { eqStruct, pick } from '~/shared/lib';
import { FlowContext } from '../context';
import { Handle } from '../handle';
import { NodeSettingsBody, NodeSettingsProps, SimpleNode } from '../node';

const defaultNodeWsSend = create(NodeWsSendSchema);

export const WsSendNode = ({ id, selected }: XF.NodeProps) => {
  const nodeId = Ulid.fromCanonical(id).bytes;

  return (
    <SimpleNode
      className={tw`text-indigo-500`}
      handles={
        <>
          <Handle nodeId={nodeId} position={XF.Position.Left} type='target' />
          <Handle nodeId={nodeId} position={XF.Position.Right} type='source' />
        </>
      }
      icon={<FiSend />}
      nodeId={nodeId}
      selected={selected}
      title='WS Send'
    />
  );
};

export const WsSendSettings = ({ nodeId }: NodeSettingsProps) => {
  const collection = useApiCollection(NodeWsSendCollectionSchema);
  const nodeCollection = useApiCollection(NodeCollectionSchema);

  const { flowId, isReadOnly = false } = use(FlowContext);

  const { message, wsConnectionNodeName } =
    useLiveQuery(
      (_) =>
        _.from({ item: collection })
          .where((_) => eq(_.item.nodeId, nodeId))
          .select((_) => pick(_.item, 'message', 'wsConnectionNodeName'))
          .findOne(),
      [collection, nodeId],
    ).data ?? defaultNodeWsSend;

  const { data: wsConnectionNodes } = useLiveQuery(
    (_) =>
      _.from({ item: nodeCollection })
        .where(eqStruct({ flowId, kind: NodeKind.WS_CONNECTION }))
        .select((_) => pick(_.item, 'name', 'nodeId')),
    [flowId, nodeCollection],
  );

  return (
    <NodeSettingsBody nodeId={nodeId} title='WebSocket Send'>
      <FieldLabel>Connection</FieldLabel>
      <Select
        aria-label='WebSocket Connection'
        isDisabled={isReadOnly}
        items={wsConnectionNodes}
        onChange={(_) => {
          if (_ === null) return;
          collection.utils.updatePaced({ nodeId, wsConnectionNodeName: String(_) });
        }}
        value={wsConnectionNodeName || null}
      >
        {(_) => <SelectItem id={_.name}>{_.name}</SelectItem>}
      </Select>

      <FieldLabel className={tw`mt-4`}>Message</FieldLabel>
      <ReferenceField
        onChange={(_) => collection.utils.updatePaced({ message: _, nodeId })}
        readOnly={isReadOnly}
        value={message}
      />
    </NodeSettingsBody>
  );
};
