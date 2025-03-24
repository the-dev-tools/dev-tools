import CodeMirror from '@uiw/react-codemirror';
import { Position } from '@xyflow/react';
import { use, useState } from 'react';
import { FiTerminal, FiX } from 'react-icons/fi';

import { useConnectMutation } from '@the-dev-tools/api/connect-query';
import { nodeUpdate } from '@the-dev-tools/spec/flow/node/v1/node-NodeService_connectquery';
import { ButtonAsLink } from '@the-dev-tools/ui/button';
import { tw } from '@the-dev-tools/ui/tailwind-literal';

import { useCodeMirrorExtensions } from '../../code-mirror';
import { FlowContext, Handle } from '../internal';
import { FlowSearch } from '../layout';
import { NodeBase, NodePanelProps, NodeProps } from '../node';

export const JavaScriptNode = (props: NodeProps) => (
  <>
    <NodeBase {...props} Icon={FiTerminal}>
      <div className={tw`shadow-xs overflow-auto rounded-md border border-slate-200`}>
        <div className={tw`shadow-xs border-b border-slate-600 bg-slate-700 px-3 py-2.5 text-sm leading-5 text-white`}>
          JavaScript
        </div>
        <div className={tw`bg-slate-800 px-3 py-5 text-center font-mono text-xs leading-4 text-slate-400`}>
          Double click to start writing code
        </div>
      </div>
    </NodeBase>

    <Handle type='target' position={Position.Top} />
    <Handle type='source' position={Position.Bottom} />
  </>
);

export const JavaScriptPanel = ({ node: { nodeId, js } }: NodePanelProps) => {
  const { code } = js!;
  const { isReadOnly = false } = use(FlowContext);

  const updateMutation = useConnectMutation(nodeUpdate);

  const [value, setValue] = useState(code);

  const extensions = useCodeMirrorExtensions('javascript');

  return (
    <>
      <div className={tw`flex items-center border-b border-slate-200 bg-white px-5 py-2`}>
        <div className='min-w-0'>
          <div className={tw`text-md leading-5 text-slate-400`}>Task</div>
          <div className={tw`truncate text-sm font-medium leading-5 text-slate-800`}>JavaScript</div>
        </div>

        <div className={tw`flex-1`} />

        <ButtonAsLink
          variant='ghost'
          className={tw`p-1`}
          href={{ to: '.', search: (_: Partial<FlowSearch>) => ({ ..._, node: undefined }) }}
        >
          <FiX className={tw`size-5 text-slate-500`} />
        </ButtonAsLink>
      </div>

      <CodeMirror
        value={value}
        onChange={setValue}
        onBlur={() => void updateMutation.mutate({ nodeId, js: { code: value } })}
        height='100%'
        className={tw`flex-1 overflow-auto`}
        extensions={extensions}
        readOnly={isReadOnly}
      />
    </>
  );
};
