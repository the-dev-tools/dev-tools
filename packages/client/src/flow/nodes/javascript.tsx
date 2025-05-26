import { useTransport } from '@connectrpc/connect-query';
import { useController } from '@data-client/react';
import CodeMirror from '@uiw/react-codemirror';
import { Position } from '@xyflow/react';
import { use, useState } from 'react';
import { FiTerminal, FiX } from 'react-icons/fi';

import { NodeUpdateEndpoint } from '@the-dev-tools/spec/meta/flow/node/v1/node.endpoints.ts';
import { ButtonAsLink } from '@the-dev-tools/ui/button';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useCodeMirrorLanguageExtensions } from '~code-mirror/extensions';

import { FlowContext, Handle } from '../internal';
import { FlowSearch } from '../layout';
import { NodeBody, NodeContainer, NodePanelProps, NodeProps } from '../node';

export const JavaScriptNode = (props: NodeProps) => (
  <NodeContainer
    {...props}
    handles={
      <>
        <Handle position={Position.Top} type='target' />
        <Handle position={Position.Bottom} type='source' />
      </>
    }
  >
    <JavaScriptNodeBody {...props} />
  </NodeContainer>
);

const JavaScriptNodeBody = (props: NodeProps) => (
  <NodeBody {...props} Icon={FiTerminal}>
    <div className={tw`shadow-xs overflow-auto rounded-md border border-slate-200`}>
      <div className={tw`shadow-xs border-b border-slate-600 bg-slate-700 px-3 py-2.5 text-sm leading-5 text-white`}>
        JavaScript
      </div>
      <div className={tw`bg-slate-800 px-3 py-5 text-center font-mono text-xs leading-4 text-slate-400`}>
        Double click to start writing code
      </div>
    </div>
  </NodeBody>
);

export const JavaScriptPanel = ({ node: { js, nodeId } }: NodePanelProps) => {
  const transport = useTransport();
  const controller = useController();

  const { code } = js!;
  const { isReadOnly = false } = use(FlowContext);

  const [value, setValue] = useState(code);

  const extensions = useCodeMirrorLanguageExtensions('javascript');

  return (
    <>
      <div className={tw`flex items-center border-b border-slate-200 bg-white px-5 py-2`}>
        <div className='min-w-0'>
          <div className={tw`text-md leading-5 text-slate-400`}>Task</div>
          <div className={tw`truncate text-sm font-medium leading-5 text-slate-800`}>JavaScript</div>
        </div>

        <div className={tw`flex-1`} />

        <ButtonAsLink
          className={tw`p-1`}
          href={{ search: (_: Partial<FlowSearch>) => ({ ..._, node: undefined }), to: '.' }}
          variant='ghost'
        >
          <FiX className={tw`size-5 text-slate-500`} />
        </ButtonAsLink>
      </div>

      <CodeMirror
        className={tw`flex-1 overflow-auto`}
        extensions={extensions}
        height='100%'
        onBlur={() => controller.fetch(NodeUpdateEndpoint, transport, { js: { code: value }, nodeId })}
        onChange={setValue}
        readOnly={isReadOnly}
        value={value}
      />
    </>
  );
};
