import { create, enumFromJson, enumToJson, equals, isEnumJson, Message, MessageInitShape } from '@bufbuild/protobuf';
import { useRouteContext } from '@tanstack/react-router';
import {
  getConnectedEdges,
  Node as NodeCore,
  NodeProps as NodePropsCore,
  useNodesState,
  useReactFlow,
} from '@xyflow/react';
import { Array, HashMap, Match, Option, pipe, Struct } from 'effect';
import { Ulid } from 'id128';
import { ReactNode, Suspense, use, useCallback, useRef } from 'react';
import { MenuTrigger, Tooltip, TooltipTrigger } from 'react-aria-components';
import { IconType } from 'react-icons';
import { FiMoreHorizontal } from 'react-icons/fi';
import { TbAlertTriangle, TbCancel, TbRefresh } from 'react-icons/tb';
import { tv } from 'tailwind-variants';
import { useDebouncedCallback } from 'use-debounce';

import {
  NodeGetResponse,
  NodeKind,
  NodeKindSchema,
  NodeListItem,
  NodeListItemSchema,
  NodeSchema,
  NodeState,
  PositionSchema,
} from '@the-dev-tools/spec/flow/node/v1/node_pb';
import {
  NodeCreateEndpoint,
  NodeDeleteEndpoint,
  NodeGetEndpoint,
  NodeListEndpoint,
  NodeUpdateEndpoint,
} from '@the-dev-tools/spec/meta/flow/node/v1/node.endpoints.ts';
import { Button } from '@the-dev-tools/ui/button';
import { CheckIcon, Spinner } from '@the-dev-tools/ui/icons';
import { Menu, MenuItem, useContextMenuState } from '@the-dev-tools/ui/menu';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextField, useEditableTextState } from '@the-dev-tools/ui/text-field';
import { useEscapePortal } from '@the-dev-tools/ui/utils';
import { useMutate, useQuery } from '~data-client';

import { GenericMessage } from '~api/utils';
import { FlowContext } from './internal';
import { FlowSearch } from './layout';

export interface NodeData extends Pick<NodeListItem, 'info' | 'noOp' | 'state'> {}
export interface Node extends NodeCore<NodeData> {}
export interface NodeProps extends NodePropsCore<Node> {}

export interface NodePanelProps {
  node: NodeGetResponse;
}

export const Node = {
  fromDTO: ({ kind, nodeId, position, ...data }: GenericMessage<NodeListItem>): Node => ({
    data: Struct.pick(data, 'info', 'noOp', 'state'),
    id: Ulid.construct(nodeId).toCanonical(),
    origin: [0.5, 0],
    position: Struct.pick(position!, 'x', 'y'),
    type: enumToJson(NodeKindSchema, kind),
  }),

  toDTO: (_: Node): Pick<NodeListItem, 'kind' | 'nodeId' | 'position'> => ({
    kind: isEnumJson(NodeKindSchema, _.type) ? enumFromJson(NodeKindSchema, _.type) : NodeKind.UNSPECIFIED,
    nodeId: Ulid.fromCanonical(_.id).bytes,
    position: create(PositionSchema, _.position),
  }),
};

const nodeContainerStyles = tv({
  base: tw`nopan shadow-xs relative w-80 rounded-lg bg-slate-200 p-1 outline-1 transition-colors`,
  variants: {
    isSelected: { true: tw`bg-slate-300` },
    state: {
      [NodeState.CANCELED]: tw`outline-slate-600`,
      [NodeState.FAILURE]: tw`outline-red-600`,
      [NodeState.RUNNING]: tw`outline-violet-600`,
      [NodeState.SUCCESS]: tw`outline-green-600`,
      [NodeState.UNSPECIFIED]: tw`outline-slate-300`,
    } satisfies Record<NodeState, string>,
  },
});

interface NodeContainerProps extends NodeProps {
  children: ReactNode;
  handles?: ReactNode;
}

export const NodeContainer = ({ children, data: { state }, handles, selected }: NodeContainerProps) => (
  <div className={nodeContainerStyles({ isSelected: selected, state })}>
    <Suspense
      fallback={
        <div className={tw`flex h-full items-center justify-center`}>
          <Spinner className={tw`size-8`} />
        </div>
      }
    >
      {children}
    </Suspense>

    {handles}
  </div>
);

interface NodeBodyProps extends NodeProps {
  children: ReactNode;
  Icon: IconType;
}

export const NodeBody = ({ children, data: { info, state }, Icon, id }: NodeBodyProps) => {
  const nodeId = Ulid.fromCanonical(id).bytes;

  const { name } = useQuery(NodeGetEndpoint, { nodeId });

  const { deleteElements, getEdges, getNode, getZoom } = useReactFlow();
  const { isReadOnly = false } = use(FlowContext);

  const [nodeUpdate, nodeUpdateLoading] = useMutate(NodeUpdateEndpoint);

  const ref = useRef<HTMLDivElement>(null);
  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  const escape = useEscapePortal(ref);

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    onSuccess: (_) => nodeUpdate({ name: _, nodeId }),
    value: name,
  });

  let stateIndicator = pipe(
    Match.value(state),
    Match.when(NodeState.RUNNING, () => (
      <TbRefresh className={tw`size-5 animate-spin text-violet-600`} style={{ animationDirection: 'reverse' }} />
    )),
    Match.when(NodeState.SUCCESS, () => <CheckIcon className={tw`size-5 text-green-600`} />),
    Match.when(NodeState.CANCELED, () => <TbCancel className={tw`size-5 text-slate-600`} />),
    Match.when(NodeState.FAILURE, () => <TbAlertTriangle className={tw`size-5 text-red-600`} />),
    Match.orElse(() => null),
  );

  if (stateIndicator && info)
    stateIndicator = (
      <TooltipTrigger delay={750}>
        <Button className={tw`p-0`} variant='ghost'>
          {stateIndicator}
        </Button>
        <Tooltip className={tw`max-w-lg rounded-md bg-slate-800 px-2 py-1 text-xs text-white`}>{info}</Tooltip>
      </TooltipTrigger>
    );

  return (
    <>
      <div
        className={tw`flex items-center gap-3 px-1 pb-1.5 pt-0.5`}
        onContextMenu={(event) => {
          const offset = ref.current?.getBoundingClientRect();
          if (!offset) return;
          onContextMenu(event, offset, getZoom());
        }}
        ref={ref}
      >
        <Icon className={tw`size-5 text-slate-500`} />

        <div className={tw`h-4 w-px bg-slate-300`} />

        <div className={tw`flex-1 truncate text-xs font-medium leading-5 tracking-tight`} ref={escape.ref}>
          {name}
        </div>

        {isEditing &&
          escape.render(
            <TextField
              aria-label='New node name'
              className={tw`w-full`}
              inputClassName={tw`py-0.75 -mx-2 mt-2 bg-white`}
              isDisabled={nodeUpdateLoading}
              {...textFieldProps}
            />,
            getZoom(),
          )}

        {stateIndicator}

        {!isReadOnly && (
          <MenuTrigger {...menuTriggerProps}>
            <Button className={tw`nodrag p-0.5`} variant='ghost'>
              <FiMoreHorizontal className={tw`size-4 text-slate-500`} />
            </Button>

            <Menu {...menuProps}>
              <MenuItem href={{ search: (_: Partial<FlowSearch>) => ({ ..._, node: id }), to: '.' }}>Edit</MenuItem>

              <MenuItem onAction={() => void edit()}>Rename</MenuItem>

              <MenuItem
                onAction={async () => {
                  const node = getNode(id);

                  const { createEdges = [], edges = [] } = pipe(
                    getConnectedEdges([node!], getEdges()),
                    Array.groupBy((_) => (_.id.startsWith('create-') ? 'createEdges' : 'edges')),
                  );

                  await deleteElements({
                    edges: [...createEdges, ...edges],
                    nodes: [{ id }, ...createEdges.map((_) => ({ id: _.target }))],
                  });
                }}
                variant='danger'
              >
                Delete
              </MenuItem>
            </Menu>
          </MenuTrigger>
        )}
      </div>

      {children}
    </>
  );
};

export const useMakeNode = () => {
  const { dataClient } = useRouteContext({ from: '__root__' });

  const { flowId } = use(FlowContext);

  return useCallback(
    async (data: Omit<MessageInitShape<typeof NodeSchema>, keyof Message>) => {
      const { nodeId } = await dataClient.fetch(NodeCreateEndpoint, { flowId, ...data });
      return create(NodeListItemSchema, { nodeId, ...data });
    },
    [dataClient, flowId],
  );
};

export const useNodeStateSynced = () => {
  const { dataClient } = useRouteContext({ from: '__root__' });

  const { flowId, isReadOnly = false } = use(FlowContext);

  const { items: nodesServer } = useQuery(NodeListEndpoint, { flowId });

  const [nodesClient, setNodesClient, onNodesChange] = useNodesState(nodesServer.map(Node.fromDTO));

  const sync = useDebouncedCallback(async () => {
    const nodeServerMap = pipe(
      nodesServer.map((_) => {
        const id = Ulid.construct(_.nodeId).toCanonical();
        const value = pipe(Struct.pick(_, 'kind', 'nodeId', 'position'), (_) => create(NodeListItemSchema, _));
        return [id, value] as const;
      }),
      HashMap.fromIterable,
    );

    const nodeClientMap = pipe(
      nodesClient.map((_) => {
        const value = create(NodeListItemSchema, Node.toDTO(_));
        return [_.id, value] as const;
      }),
      HashMap.fromIterable,
    );

    const nodes: Record<string, [string, ReturnType<typeof Node.toDTO>][]> = pipe(
      HashMap.union(nodeServerMap, nodeClientMap),
      HashMap.entries,
      Array.groupBy(([id]) => {
        const nodeServer = HashMap.get(nodeServerMap, id);
        const nodeClient = HashMap.get(nodeClientMap, id);

        if (Option.isNone(nodeServer)) return 'create';
        if (Option.isNone(nodeClient)) return 'delete';

        return equals(NodeListItemSchema, nodeServer.value, nodeClient.value) ? 'ignore' : 'update';
      }),
    );

    await pipe(
      nodes['create'] ?? [],
      Array.filterMap(([_id, node]) =>
        pipe(
          Option.liftPredicate(node, (_) => !_.nodeId.length),
          Option.map((node) => dataClient.fetch(NodeCreateEndpoint, node)),
        ),
      ),
      (_) => Promise.allSettled(_),
    );

    await pipe(
      nodes['delete'] ?? [],
      Array.map(([_id, node]) => dataClient.fetch(NodeDeleteEndpoint, node)),
      (_) => Promise.allSettled(_),
    );

    await pipe(
      nodes['update'] ?? [],
      Array.map(([_id, node]) => dataClient.fetch(NodeUpdateEndpoint, node)),
      (_) => Promise.allSettled(_),
    );
  }, 500);

  const onNodesChangeSync: typeof onNodesChange = (changes) => {
    onNodesChange(changes);
    if (isReadOnly) return;
    void sync();
  };

  return [nodesClient, setNodesClient, onNodesChangeSync] as const;
};
