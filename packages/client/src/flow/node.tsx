import { create, enumFromJson, enumToJson, isEnumJson, Message, MessageInitShape } from '@bufbuild/protobuf';
import { useRouteContext } from '@tanstack/react-router';
import { getConnectedEdges, Node as NodeCore, NodeProps as NodePropsCore, useReactFlow } from '@xyflow/react';
import { Array, Match, pipe, Struct } from 'effect';
import { Ulid } from 'id128';
import { ReactNode, Suspense, use, useCallback, useRef } from 'react';
import { MenuTrigger, Tooltip, TooltipTrigger } from 'react-aria-components';
import { IconType } from 'react-icons';
import { FiMoreHorizontal } from 'react-icons/fi';
import { TbAlertTriangle, TbCancel, TbRefresh } from 'react-icons/tb';
import { tv } from 'tailwind-variants';
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
  NodeGetEndpoint,
  NodeUpdateEndpoint,
} from '@the-dev-tools/spec/meta/flow/node/v1/node.endpoints.ts';
import { Button } from '@the-dev-tools/ui/button';
import { CheckIcon, Spinner } from '@the-dev-tools/ui/icons';
import { Menu, MenuItem, MenuItemLink, useContextMenuState } from '@the-dev-tools/ui/menu';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextField, useEditableTextState } from '@the-dev-tools/ui/text-field';
import { useEscapePortal } from '@the-dev-tools/ui/utils';
import { GenericMessage } from '~api/utils';
import { useMutate, useQuery } from '~data-client';
import { useNodeDuplicate } from './copy-paste';
import { FlowContext } from './internal';

export interface NodeData extends Pick<NodeListItem, 'info' | 'noOp' | 'state'> {}
export interface Node extends NodeCore<NodeData> {}
export interface NodeProps extends NodePropsCore<Node> {}

export interface NodePanelProps {
  node: NodeGetResponse;
}

export const Node = {
  fromDTO: ({ kind, nodeId, position, ...data }: GenericMessage<NodeListItem>, extra?: Partial<Node>): Node => ({
    data: Struct.pick(data, 'info', 'noOp', 'state'),
    id: Ulid.construct(nodeId).toCanonical(),
    origin: [0.5, 0],
    position: Struct.pick(position!, 'x', 'y'),
    type: enumToJson(NodeKindSchema, kind),
    ...extra,
  }),

  toDTO: (_: Node): Pick<NodeListItem, 'kind' | 'nodeId' | 'position'> => ({
    kind: isEnumJson(NodeKindSchema, _.type) ? enumFromJson(NodeKindSchema, _.type) : NodeKind.UNSPECIFIED,
    nodeId: Ulid.fromCanonical(_.id).bytes,
    position: create(PositionSchema, _.position),
  }),
};

const nodeContainerStyles = tv({
  // eslint-disable-next-line better-tailwindcss/no-unregistered-classes
  base: tw`nopan relative w-80 rounded-lg bg-slate-200 p-1 shadow-xs outline-1 transition-colors`,
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

  const duplicate = useNodeDuplicate(id);

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
        className={tw`flex items-center gap-3 px-1 pt-0.5 pb-1.5`}
        onContextMenu={(event) => {
          const offset = ref.current?.getBoundingClientRect();
          if (!offset) return;
          onContextMenu(event, offset, getZoom());
        }}
        ref={ref}
      >
        <Icon className={tw`size-5 text-slate-500`} />

        <div className={tw`h-4 w-px bg-slate-300`} />

        <div className={tw`flex-1 truncate text-xs leading-5 font-medium tracking-tight`} ref={escape.ref}>
          {name}
        </div>

        {isEditing &&
          escape.render(
            <TextField
              aria-label='New node name'
              className={tw`w-full`}
              inputClassName={tw`-mx-2 mt-2 bg-white py-0.75`}
              isDisabled={nodeUpdateLoading}
              {...textFieldProps}
            />,
            getZoom(),
          )}

        {stateIndicator}

        {!isReadOnly && (
          <MenuTrigger {...menuTriggerProps}>
            {/* eslint-disable-next-line better-tailwindcss/no-unregistered-classes */}
            <Button className={tw`nodrag p-0.5`} variant='ghost'>
              <FiMoreHorizontal className={tw`size-4 text-slate-500`} />
            </Button>

            <Menu {...menuProps}>
              <MenuItemLink search={(_) => ({ ..._, node: id })} to='.'>
                Edit
              </MenuItemLink>

              <MenuItem onAction={() => void edit()}>Rename</MenuItem>

              <MenuItem onAction={() => void duplicate()}>Duplicate</MenuItem>

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
      return create(NodeListItemSchema, { ...data, nodeId });
    },
    [dataClient, flowId],
  );
};
