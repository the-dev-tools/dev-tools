import { create } from '@bufbuild/protobuf';
import { eq, useLiveQuery } from '@tanstack/react-db';
import * as XF from '@xyflow/react';
import { Ulid } from 'id128';
import { use } from 'react';
import { FiCornerDownLeft, FiPlus, FiX } from 'react-icons/fi';
import { NodeSubFlowReturnSchema } from '@the-dev-tools/spec/buf/api/flow/v1/flow_pb';
import { NodeSubFlowReturnCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/flow';
import { Button } from '@the-dev-tools/ui/button';
import { FieldLabel } from '@the-dev-tools/ui/field';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextInputField } from '@the-dev-tools/ui/text-field';
import { ReferenceField } from '~/features/expression';
import { useApiCollection } from '~/shared/api';
import { pick } from '~/shared/lib';
import { FlowContext } from '../context';
import { Handle } from '../handle';
import { NodeSettingsBody, NodeSettingsProps, SimpleNode } from '../node';

const defaultNodeSubFlowReturn = create(NodeSubFlowReturnSchema);

export const SubFlowReturnNode = ({ id, selected }: XF.NodeProps) => {
  const nodeId = Ulid.fromCanonical(id).bytes;

  return (
    <SimpleNode
      className={tw`text-orange-500`}
      handles={<Handle nodeId={nodeId} position={XF.Position.Left} type='target' />}
      icon={<FiCornerDownLeft />}
      nodeId={nodeId}
      selected={selected}
      title='Sub-Flow Return'
    />
  );
};

export const SubFlowReturnSettings = ({ nodeId }: NodeSettingsProps) => {
  const collection = useApiCollection(NodeSubFlowReturnCollectionSchema);

  const data =
    useLiveQuery(
      (_) =>
        _.from({ item: collection })
          .where((_) => eq(_.item.nodeId, nodeId))
          .select((_) => pick(_.item, 'outputs'))
          .findOne(),
      [collection, nodeId],
    ).data ?? defaultNodeSubFlowReturn;

  const { isReadOnly = false } = use(FlowContext);

  return (
    <NodeSettingsBody nodeId={nodeId} title='Sub-Flow Return'>
      <div className={tw`flex flex-col gap-4`}>
        <FieldLabel>Output Mappings</FieldLabel>
        <div className={tw`text-xs text-on-neutral-low`}>
          Define values to return to the calling flow. Expressions are evaluated against this flow&apos;s variables.
        </div>

        <div className={tw`flex flex-col gap-3`}>
          {data.outputs.map((output, index) => (
            <div
              className={tw`flex items-start gap-2 rounded-lg border border-neutral bg-neutral-lowest p-3`}
              key={index}
            >
              <div className={tw`flex flex-1 flex-col gap-2`}>
                <TextInputField
                  aria-label='Output name'
                  isReadOnly={isReadOnly}
                  onChange={(name) => {
                    const outputs = [...data.outputs];
                    outputs[index] = { ...outputs[index]!, name };
                    collection.utils.updatePaced({ nodeId, outputs });
                  }}
                  placeholder='Output name'
                  value={output.name}
                />

                <ReferenceField
                  onChange={(expression) => {
                    const outputs = [...data.outputs];
                    outputs[index] = { ...outputs[index]!, expression };
                    collection.utils.updatePaced({ nodeId, outputs });
                  }}
                  placeholder='Expression'
                  readOnly={isReadOnly}
                  value={output.expression}
                />
              </div>

              {!isReadOnly && (
                <Button
                  className={tw`mt-1 p-1 text-danger`}
                  onPress={() => {
                    const outputs = data.outputs.filter((_, i) => i !== index);
                    collection.utils.updatePaced({ nodeId, outputs });
                  }}
                  variant='ghost'
                >
                  <FiX className={tw`size-4`} />
                </Button>
              )}
            </div>
          ))}
        </div>

        {!isReadOnly && (
          <Button
            className={tw`w-full justify-start`}
            onPress={() => {
              const outputs = [...data.outputs, { expression: '', name: '' }];
              collection.utils.updatePaced({ nodeId, outputs });
            }}
            variant='ghost'
          >
            <FiPlus className={tw`size-4 text-on-neutral-low`} />
            Add output
          </Button>
        )}
      </div>
    </NodeSettingsBody>
  );
};
