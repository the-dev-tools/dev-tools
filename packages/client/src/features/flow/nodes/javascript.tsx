import { create } from '@bufbuild/protobuf';
import { eq, useLiveQuery } from '@tanstack/react-db';
import CodeMirror from '@uiw/react-codemirror';
import * as XF from '@xyflow/react';
import { Ulid } from 'id128';
import { use, useState } from 'react';
import { FiTerminal } from 'react-icons/fi';
import { NodeJsSchema } from '@the-dev-tools/spec/buf/api/flow/v1/flow_pb';
import { NodeJsCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/flow';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useApiCollection } from '~/api';
import { useCodeMirrorLanguageExtensions } from '~/code-mirror/extensions';
import { pick } from '~/utils/tanstack-db';
import { FlowContext } from '../context';
import { Handle } from '../handle';
import { NodeBodyNew, NodeName, NodePanelProps, NodeSettings, NodeStateIndicator, NodeTitle } from '../node';

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

export const JavaScriptSettings = ({ nodeId }: NodePanelProps) => {
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
    <NodeSettings nodeId={nodeId} title='JavaScript'>
      <CodeMirror
        extensions={extensions}
        height='100%'
        onBlur={() => void collection.utils.update({ code: value, nodeId })}
        onChange={setValue}
        readOnly={isReadOnly}
        value={value}
      />
    </NodeSettings>
  );
};
