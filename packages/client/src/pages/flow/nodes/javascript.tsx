import { create } from '@bufbuild/protobuf';
import { eq, useLiveQuery } from '@tanstack/react-db';
import CodeMirror from '@uiw/react-codemirror';
import * as XF from '@xyflow/react';
import { Ulid } from 'id128';
import { use } from 'react';
import { FiTerminal } from 'react-icons/fi';
import { NodeJsSchema } from '@the-dev-tools/spec/buf/api/flow/v1/flow_pb';
import { NodeJsCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/flow';
import { useTheme } from '@the-dev-tools/ui';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useCodeMirrorLanguageExtensions } from '~/features/expression';
import { useApiCollection } from '~/shared/api';
import { pick } from '~/shared/lib';
import { FlowContext } from '../context';
import { Handle } from '../handle';
import { NodeSettingsBody, NodeSettingsProps, SimpleNode } from '../node';

export const JavaScriptNode = ({ id, selected }: XF.NodeProps) => {
  const nodeId = Ulid.fromCanonical(id).bytes;

  return (
    <SimpleNode
      className={tw`text-amber-500`}
      handles={
        <>
          <Handle nodeId={nodeId} position={XF.Position.Left} type='target' />
          <Handle nodeId={nodeId} position={XF.Position.Right} type='source' />
        </>
      }
      icon={<FiTerminal />}
      nodeId={nodeId}
      selected={selected}
      title='JavaScript'
    />
  );
};

export const JavaScriptSettings = ({ nodeId }: NodeSettingsProps) => {
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

  const extensions = useCodeMirrorLanguageExtensions('javascript');
  const { resolvedTheme } = useTheme();

  return (
    <NodeSettingsBody nodeId={nodeId} title='JavaScript'>
      <CodeMirror
        extensions={extensions}
        height='100%'
        onChange={(_) => collection.utils.updatePaced({ code: _, nodeId })}
        readOnly={isReadOnly}
        theme={resolvedTheme}
        value={code}
      />
    </NodeSettingsBody>
  );
};
