import { create } from '@bufbuild/protobuf';
import { eq, useLiveQuery } from '@tanstack/react-db';
import CodeMirror from '@uiw/react-codemirror';
import * as XF from '@xyflow/react';
import { Ulid } from 'id128';
import { use, useState } from 'react';
import { FiTerminal, FiX } from 'react-icons/fi';
import { NodeJsSchema } from '@the-dev-tools/spec/buf/api/flow/v1/flow_pb';
import { NodeJsCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/flow';
import { ButtonAsLink } from '@the-dev-tools/ui/button';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useApiCollection } from '~/api';
import { useCodeMirrorLanguageExtensions } from '~/code-mirror/extensions';
import { pick } from '~/utils/tanstack-db';
import { FlowContext } from '../context';
import { Handle } from '../handle';
import { NodeBodyNew, NodeExecutionPanel, NodeName, NodePanelProps, NodeStateIndicator, NodeTitle } from '../node';

export const JavaScriptNode = ({ id, selected }: XF.NodeProps) => {
  const nodeId = Ulid.fromCanonical(id).bytes;

  return (
    <div className={tw`pointer-events-none flex flex-col items-center`}>
      <div className={tw`pointer-events-auto relative`}>
        <NodeBodyNew className={tw`text-amber-500`} icon={<FiTerminal />} nodeId={nodeId} selected={selected} />

        <Handle nodeId={nodeId} position={XF.Position.Left} type='target' />
        <Handle nodeId={nodeId} position={XF.Position.Right} type='source' />
      </div>

      <NodeTitle className={tw`mt-1`}>JavaScript</NodeTitle>
      <NodeName nodeId={nodeId} />
      <NodeStateIndicator nodeId={nodeId} />
    </div>
  );
};

export const JavaScriptPanel = ({ nodeId }: NodePanelProps) => {
  const collection = useApiCollection(NodeJsCollectionSchema);

  const { code } =
    useLiveQuery(
      (_) =>
        _.from({ item: collection })
          .where((_) => eq(_.item.nodeId, nodeId))
          .select((_) => pick(_.item, 'code'))
          .findOne(),
      [collection, nodeId],
    ).data ?? create(NodeJsSchema);

  const { isReadOnly = false } = use(FlowContext);

  const [value, setValue] = useState(code);

  const extensions = useCodeMirrorLanguageExtensions('javascript');

  return (
    <>
      <div className={tw`flex items-center border-b border-slate-200 bg-white px-5 py-2`}>
        <div className='min-w-0'>
          <div className={tw`text-md leading-5 text-slate-400`}>Task</div>
          <div className={tw`truncate text-sm leading-5 font-medium text-slate-800`}>JavaScript</div>
        </div>

        <div className={tw`flex-1`} />

        <ButtonAsLink className={tw`p-1`} search={(_) => ({ ..._, node: undefined })} to='.' variant='ghost'>
          <FiX className={tw`size-5 text-slate-500`} />
        </ButtonAsLink>
      </div>

      <div className={tw`mx-5 my-4 overflow-auto rounded-lg border border-slate-200`}>
        <div
          className={tw`
            border-b border-slate-200 bg-slate-50 px-3 py-2 text-md leading-5 font-medium tracking-tight text-slate-800
          `}
        >
          Code
        </div>

        <CodeMirror
          extensions={extensions}
          height='100%'
          onBlur={() => void collection.utils.update({ code: value, nodeId })}
          onChange={setValue}
          readOnly={isReadOnly}
          value={value}
        />
      </div>

      <NodeExecutionPanel nodeId={nodeId} />
    </>
  );
};
