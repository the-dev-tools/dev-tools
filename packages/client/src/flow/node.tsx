import { create, enumFromJson, enumToJson, isEnumJson, Message, MessageInitShape } from '@bufbuild/protobuf';
import { useRouteContext } from '@tanstack/react-router';
import { getConnectedEdges, Node as NodeCore, NodeProps as NodePropsCore, useReactFlow } from '@xyflow/react';
import { Array, Match, Option, pipe, Struct } from 'effect';
import { Ulid } from 'id128';
import { ReactNode, Suspense, use, useCallback, useRef, useState } from 'react';
import { Key, MenuTrigger, Tab, TabList, TabPanel, Tabs, Tooltip, TooltipTrigger } from 'react-aria-components';
import { IconType } from 'react-icons';
import { FiMoreHorizontal } from 'react-icons/fi';
import { TbAlertTriangle, TbCancel, TbRefresh } from 'react-icons/tb';
import { twMerge } from 'tailwind-merge';
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
  NodeExecutionGetEndpoint,
  NodeExecutionListEndpoint,
} from '@the-dev-tools/spec/meta/flow/node/execution/v1/execution.endpoints.ts';
import { NodeExecutionGetResponseEntity } from '@the-dev-tools/spec/meta/flow/node/execution/v1/execution.entities.js';
import {
  NodeCreateEndpoint,
  NodeGetEndpoint,
  NodeUpdateEndpoint,
} from '@the-dev-tools/spec/meta/flow/node/v1/node.endpoints.ts';
import { Button } from '@the-dev-tools/ui/button';
import { CheckIcon, Spinner } from '@the-dev-tools/ui/icons';
import { JsonTree } from '@the-dev-tools/ui/json-tree';
import { ListBoxItem } from '@the-dev-tools/ui/list-box';
import { Menu, MenuItem, MenuItemLink, useContextMenuState } from '@the-dev-tools/ui/menu';
import { Select } from '@the-dev-tools/ui/select';
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

interface NodeExecutionPanelProps {
  nodeId: Uint8Array;
  renderOutput?: (execution: NodeExecutionGetResponseEntity) => ReactNode;
}

export const NodeExecutionPanel = ({ nodeId, renderOutput }: NodeExecutionPanelProps) => {
  const { items } = useQuery(NodeExecutionListEndpoint, { nodeId });

  const firstItem = pipe(
    Array.head(items),
    Option.map((_) => Ulid.construct(_.nodeExecutionId).toCanonical()),
    Option.getOrNull,
  );

  const [selectedKey, setSelectedKey] = useState<Key | null>(firstItem);

  if (selectedKey === null && firstItem !== null) setSelectedKey(firstItem);

  return (
    <div className={tw`mx-5 my-4 overflow-auto rounded-lg border border-slate-200`}>
      <div
        className={tw`
          flex items-center justify-between border-b border-slate-200 bg-slate-50 px-3 py-2 text-md leading-5
          font-medium tracking-tight text-slate-800
        `}
      >
        <div>Execution data</div>

        {items.length > 0 && (
          <Select
            aria-label='Node execution'
            listBoxItems={items}
            onSelectionChange={setSelectedKey}
            selectedKey={selectedKey}
          >
            {(_) => <ListBoxItem id={Ulid.construct(_.nodeExecutionId).toCanonical()}>{_.name}</ListBoxItem>}
          </Select>
        )}
      </div>

      <div className={tw`px-5 py-3`}>
        {typeof selectedKey !== 'string' ? (
          <div className={tw`text-sm`}>This node has not been executed yet</div>
        ) : (
          <Suspense
            fallback={
              <div className={tw`flex h-full items-center justify-center p-4`}>
                <Spinner className={tw`size-8`} />
              </div>
            }
          >
            <NodeExecutionTabs
              nodeExecutionId={Ulid.fromCanonical(selectedKey).bytes}
              renderOutput={renderOutput}
              // {...(renderOutput && { renderOutput })}
            />
          </Suspense>
        )}
      </div>
    </div>
  );
};

interface NodeExecutionTabsProps {
  nodeExecutionId: Uint8Array;
  renderOutput?: ((execution: NodeExecutionGetResponseEntity) => ReactNode) | undefined;
}

const NodeExecutionTabs = ({ nodeExecutionId, renderOutput }: NodeExecutionTabsProps) => {
  const data = useQuery(NodeExecutionGetEndpoint, { nodeExecutionId });

  return (
    <Tabs className={tw`flex h-full flex-col pb-4`}>
      <TabList className={tw`flex items-center gap-3 border-b border-slate-200 text-md`}>
        <Tab
          className={({ isSelected }) =>
            twMerge(
              tw`
                -mb-px cursor-pointer border-b-2 border-transparent py-2 text-md leading-5 font-medium tracking-tight
                text-slate-500 transition-colors
              `,
              isSelected && tw`border-b-violet-700 text-slate-800`,
            )
          }
          id='input'
        >
          Input
        </Tab>

        <Tab
          className={({ isSelected }) =>
            twMerge(
              tw`
                -mb-px cursor-pointer border-b-2 border-transparent py-2 text-md leading-5 font-medium tracking-tight
                text-slate-500 transition-colors
              `,
              isSelected && tw`border-b-violet-700 text-slate-800`,
            )
          }
          id='output'
        >
          Output
        </Tab>
      </TabList>

      <div className={tw`flex-1 overflow-auto pt-4`}>
        <Suspense
          fallback={
            <div className={tw`flex h-full items-center justify-center`}>
              <Spinner className={tw`size-12`} />
            </div>
          }
        >
          <TabPanel id='input'>{data.input && <JsonTree value={data.input} />}</TabPanel>

          <TabPanel id='output'>
            {renderOutput?.(data)}
            {!renderOutput && data.output && <JsonTree value={data.output} />}
          </TabPanel>
        </Suspense>
      </div>
    </Tabs>
  );
};
