import { create } from '@bufbuild/protobuf';
import { eq, useLiveQuery } from '@tanstack/react-db';
import * as XF from '@xyflow/react';
import { Ulid } from 'id128';
import { use } from 'react';
import { FiPlus, FiX } from 'react-icons/fi';
import { NodeRunSubFlowSchema } from '@the-dev-tools/spec/buf/api/flow/v1/flow_pb';
import { FlowCollectionSchema, NodeRunSubFlowCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/flow';
import { Button } from '@the-dev-tools/ui/button';
import { FieldLabel } from '@the-dev-tools/ui/field';
import { FlowsIcon } from '@the-dev-tools/ui/icons';
import { Select, SelectItem } from '@the-dev-tools/ui/select';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextInputField } from '@the-dev-tools/ui/text-field';
import { ReferenceContext, ReferenceField } from '~/features/expression';
import { useApiCollection } from '~/shared/api';
import { pick } from '~/shared/lib';
import { routes } from '~/shared/routes';
import { FlowContext } from '../context';
import { Handle } from '../handle';
import { NodeSettingsBody, NodeSettingsProps, NodeTitle, SimpleNode } from '../node';

const defaultNodeRunSubFlow = create(NodeRunSubFlowSchema);

export const RunSubFlowNode = ({ id, selected }: XF.NodeProps) => {
  const nodeId = Ulid.fromCanonical(id).bytes;

  const collection = useApiCollection(NodeRunSubFlowCollectionSchema);

  const data =
    useLiveQuery(
      (_) =>
        _.from({ item: collection })
          .where((_) => eq(_.item.nodeId, nodeId))
          .select((_) => pick(_.item, 'targetFlowName'))
          .findOne(),
      [collection, nodeId],
    ).data ?? defaultNodeRunSubFlow;

  return (
    <SimpleNode
      className={tw`w-36 text-violet-500`}
      handles={
        <>
          <Handle nodeId={nodeId} position={XF.Position.Left} type='target' />
          <Handle nodeId={nodeId} position={XF.Position.Right} type='source' />
        </>
      }
      icon={<FlowsIcon />}
      nodeId={nodeId}
      selected={selected}
    >
      <NodeTitle className={tw`text-left`}>{data.targetFlowName || 'Run Sub-Flow'}</NodeTitle>
    </SimpleNode>
  );
};

export const RunSubFlowSettings = ({ nodeId }: NodeSettingsProps) => {
  const { workspaceId } = routes.dashboard.workspace.route.useLoaderData();

  const collection = useApiCollection(NodeRunSubFlowCollectionSchema);
  const flowCollection = useApiCollection(FlowCollectionSchema);

  const { flowId, isReadOnly = false } = use(FlowContext);

  const data =
    useLiveQuery(
      (_) =>
        _.from({ item: collection })
          .where((_) => eq(_.item.nodeId, nodeId))
          .select((_) => pick(_.item, 'inputs', 'targetFlowId', 'targetFlowName'))
          .findOne(),
      [collection, nodeId],
    ).data ?? defaultNodeRunSubFlow;

  const currentFlowKey = Ulid.construct(flowId).toCanonical();

  const { data: allFlows } = useLiveQuery(
    (_) =>
      _.from({ item: flowCollection })
        .where((_) => eq(_.item.workspaceId, workspaceId))
        .select((_) => pick(_.item, 'flowId', 'name'))
        .orderBy((_) => _.item.name),
    [flowCollection, workspaceId],
  );

  // Exclude the current flow to prevent recursion
  const flows = allFlows.filter((f) => Ulid.construct(f.flowId).toCanonical() !== currentFlowKey);

  const selectedFlowKey = data.targetFlowId ? Ulid.construct(data.targetFlowId).toCanonical() : null;

  return (
    <NodeSettingsBody nodeId={nodeId} title='Run Sub-Flow'>
      <ReferenceContext value={{ flowNodeId: nodeId, workspaceId }}>
      <div className={tw`flex flex-col gap-5`}>
        <Select
          aria-label='Target flow'
          isDisabled={isReadOnly}
          items={flows}
          label='Target Flow'
          onChange={(_) => {
            if (_ === null) return;
            const flow = flows.find((f) => Ulid.construct(f.flowId).toCanonical() === _);
            if (!flow) return;
            collection.utils.updatePaced({
              nodeId,
              targetFlowId: flow.flowId,
              targetFlowName: flow.name,
            });
          }}
          triggerClassName={tw`w-full justify-between`}
          value={selectedFlowKey}
        >
          {(flow) => (
            <SelectItem id={Ulid.construct(flow.flowId).toCanonical()} textValue={flow.name}>
              {flow.name}
            </SelectItem>
          )}
        </Select>

        <div className={tw`flex flex-col gap-4`}>
          <FieldLabel>Input Mappings</FieldLabel>
          <div className={tw`text-xs text-on-neutral-low`}>
            Map expressions from this flow to the sub-flow&apos;s input parameters.
          </div>

          <div className={tw`flex flex-col gap-3`}>
            {data.inputs.map((input, index) => (
              <div
                className={tw`flex items-start gap-2 rounded-lg border border-neutral bg-neutral-lowest p-3`}
                key={index}
              >
                <div className={tw`flex flex-1 flex-col gap-2`}>
                  <TextInputField
                    aria-label='Parameter name'
                    isReadOnly={isReadOnly}
                    onChange={(paramName) => {
                      const inputs = [...data.inputs];
                      inputs[index] = { ...inputs[index]!, paramName };
                      collection.utils.updatePaced({ inputs, nodeId });
                    }}
                    placeholder='Parameter name'
                    value={input.paramName}
                  />

                  <ReferenceField
                    onChange={(expression) => {
                      const inputs = [...data.inputs];
                      inputs[index] = { ...inputs[index]!, expression };
                      collection.utils.updatePaced({ inputs, nodeId });
                    }}
                    placeholder='Expression'
                    readOnly={isReadOnly}
                    value={input.expression}
                  />
                </div>

                {!isReadOnly && (
                  <Button
                    className={tw`mt-1 p-1 text-danger`}
                    onPress={() => {
                      const inputs = data.inputs.filter((_, i) => i !== index);
                      collection.utils.updatePaced({ inputs, nodeId });
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
                const inputs = [...data.inputs, { expression: '', paramName: '' }];
                collection.utils.updatePaced({ inputs, nodeId });
              }}
              variant='ghost'
            >
              <FiPlus className={tw`size-4 text-on-neutral-low`} />
              Add input mapping
            </Button>
          )}
        </div>
      </div>
      </ReferenceContext>
    </NodeSettingsBody>
  );
};
