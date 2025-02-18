import { getRouteApi } from '@tanstack/react-router';
import {
  Handle as HandleCore,
  HandleProps,
  ReactFlowState,
  useNodeConnections,
  useOnSelectionChange,
  useStore,
  useStoreApi,
} from '@xyflow/react';
import { Option, pipe } from 'effect';
import { Ulid } from 'id128';
import { useState } from 'react';
import { tv } from 'tailwind-variants';

import {
  Handle as HandleKind,
  HandleJson as HandleKindJson,
  HandleSchema as HandleKindSchema,
} from '@the-dev-tools/spec/flow/edge/v1/edge_pb';
import { NodeState } from '@the-dev-tools/spec/flow/v1/flow_pb';
import { tw } from '@the-dev-tools/ui/tailwind-literal';

import { Edge } from './edge';
import { Node } from './node';

export { HandleKind, HandleKindSchema, type HandleKindJson };

const handleInnerStyles = tv({
  base: tw`pointer-events-none`,
  variants: {
    state: {
      [NodeState.UNSPECIFIED]: tw`text-slate-800`,
      [NodeState.RUNNING]: tw`text-violet-600`,
      [NodeState.SUCCESS]: tw`text-green-600`,
      [NodeState.FAILURE]: tw`text-red-600`,
    } satisfies Record<NodeState, string>,
  },
});

export const Handle = (props: HandleProps) => {
  const { id, type } = props;

  const connection = useNodeConnections({
    ...(id ? { handleId: id } : {}),
    handleType: type,
  })[0];

  const state = useStore((storeCore) => {
    // https://github.com/xyflow/xyflow/issues/4468
    const store = storeCore as unknown as ReactFlowState<Node, Edge>;

    return pipe(
      Option.fromNullable(connection),
      Option.flatMapNullable((_) => store.edgeLookup.get(_.edgeId)?.data?.state),
      Option.getOrElse(() => NodeState.UNSPECIFIED),
    );
  });

  return (
    <HandleCore
      className={tw`-z-10 size-5 overflow-visible rounded-full border-none bg-transparent shadow-sm`}
      {...props}
    >
      <svg viewBox='-10 -10 20 20' className={handleInnerStyles({ state })}>
        <circle className={tw`fill-slate-300`} r={10} />
        <circle className={tw`fill-slate-200`} r={9} />
        <circle className={tw`fill-current`} r={4} />
        {connection && <path className={tw`stroke-current stroke-1`} d='M 0 -10 L 0 10' />}
      </svg>
    </HandleCore>
  );
};

export const useSelectedNodeId = () => {
  const [selectedNodeId, setSelectedNodeId] = useState<Uint8Array | undefined>(undefined);

  useOnSelectionChange({
    onChange: ({ nodes }) => {
      if (nodes.length !== 1) return void setSelectedNodeId(undefined);
      const id = Ulid.fromCanonical(nodes[0]!.id).bytes;
      setSelectedNodeId(id);
    },
  });

  return selectedNodeId;
};

export const useSetSelectedNodes = () => {
  const { getState } = useStoreApi();
  return (nodeIds?: string[]) => {
    getState().resetSelectedElements();
    if (nodeIds?.length) getState().addSelectedNodes(nodeIds);
  };
};

export const workspaceRoute = getRouteApi('/_authorized/workspace/$workspaceIdCan');
export const flowRoute = getRouteApi('/_authorized/workspace/$workspaceIdCan/flow/$flowIdCan');
