import { Position } from '@xyflow/react';
import { Ulid } from 'id128';
import { use, useEffect } from 'react';
import { useForm } from 'react-hook-form';
import { FiPlus, FiX } from 'react-icons/fi';
import { useDebouncedCallback } from 'use-debounce';
import { NodeGetEndpoint, NodeUpdateEndpoint } from '@the-dev-tools/spec/meta/flow/node/v1/node.endpoints.ts';
import { ButtonAsLink } from '@the-dev-tools/ui/button';
import { CheckListAltIcon, IfIcon } from '@the-dev-tools/ui/icons';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useQuery } from '~data-client';
import { rootRouteApi } from '~routes';
import { ConditionField } from '../../condition';
import { FlowContext, Handle, HandleKindJson } from '../internal';
import { NodeBody, NodeContainer, NodeExecutionPanel, NodePanelProps, NodeProps } from '../node';

export const ConditionNode = (props: NodeProps) => (
  <NodeContainer
    {...props}
    handles={
      <>
        <Handle position={Position.Top} type='target' />
        <Handle
          id={'HANDLE_THEN' satisfies HandleKindJson}
          isConnectable={false}
          position={Position.Bottom}
          type='source'
        />
        <Handle
          id={'HANDLE_ELSE' satisfies HandleKindJson}
          isConnectable={false}
          position={Position.Bottom}
          type='source'
        />
      </>
    }
  >
    <ConditionNodeBody {...props} />
  </NodeContainer>
);

const ConditionNodeBody = (props: NodeProps) => {
  const nodeId = Ulid.fromCanonical(props.id).bytes;

  const { condition } = useQuery(NodeGetEndpoint, { nodeId });

  return (
    <NodeBody {...props} Icon={IfIcon}>
      <div className={tw`rounded-md border border-slate-200 bg-white shadow-xs`}>
        {condition ? (
          <div
            className={tw`
              flex justify-start gap-2 rounded-md border border-slate-200 p-3 text-xs leading-5 font-medium
              tracking-tight text-slate-800 shadow-xs
            `}
          >
            <CheckListAltIcon className={tw`size-5 text-slate-500`} />
            <span>Edit Condition</span>
          </div>
        ) : (
          <ButtonAsLink
            className={tw`
              flex w-full justify-start gap-1.5 rounded-md border border-slate-200 px-2 py-3 text-xs leading-4
              font-medium tracking-tight text-violet-600 shadow-xs
            `}
            search={(_) => ({ ..._, node: props.id })}
            to='.'
          >
            <FiPlus className={tw`size-4`} />
            <span>Setup Condition</span>
          </ButtonAsLink>
        )}
      </div>
    </NodeBody>
  );
};

export const ConditionPanel = ({ node: { condition, nodeId } }: NodePanelProps) => {
  const { dataClient } = rootRouteApi.useRouteContext();

  const { control, handleSubmit, watch } = useForm({
    resetOptions: { keepDirtyValues: true },
    values: condition!,
  });

  const { isReadOnly = false } = use(FlowContext);

  const update = useDebouncedCallback(async () => {
    await handleSubmit(async (condition) => {
      await dataClient.fetch(NodeUpdateEndpoint, { condition, nodeId });
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
          <div className={tw`text-md leading-5 text-slate-400`}>If Condition</div>
          <div className={tw`text-sm leading-5 font-medium text-slate-800`}>Node Name</div>
        </div>

        <div className={tw`flex-1`} />

        <ButtonAsLink className={tw`p-1`} search={(_) => ({ ..._, node: undefined })} to='.' variant='ghost'>
          <FiX className={tw`size-5 text-slate-500`} />
        </ButtonAsLink>
      </div>

      <div className={tw`m-5`}>
        <ConditionField control={control} isReadOnly={isReadOnly} path='condition' />
      </div>

      <NodeExecutionPanel nodeId={nodeId} />
    </>
  );
};
