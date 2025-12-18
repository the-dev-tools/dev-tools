import { create } from '@bufbuild/protobuf';
import { debounceStrategy, eq, useLiveQuery, usePacedMutations } from '@tanstack/react-db';
import * as XF from '@xyflow/react';
import { Array, HashMap, HashSet, Match, Option, pipe } from 'effect';
import { Ulid } from 'id128';
import { createContext, Dispatch, ReactNode, SetStateAction, use, useContext, useRef, useState } from 'react';
import {
  Button as AriaButton,
  Key,
  MenuTrigger,
  Tab,
  TabList,
  TabPanel,
  Tabs,
  Tooltip,
  TooltipTrigger,
  Tree,
} from 'react-aria-components';
import { FiMoreHorizontal } from 'react-icons/fi';
import { IconType } from 'react-icons/lib';
import { TbAlertTriangle, TbCancel, TbRefresh } from 'react-icons/tb';
import { twMerge } from 'tailwind-merge';
import { tv } from 'tailwind-variants';
import {
  FlowItemState,
  FlowService,
  NodeExecutionSchema,
  NodeSchema,
} from '@the-dev-tools/spec/buf/api/flow/v1/flow_pb';
import { NodeCollectionSchema, NodeExecutionCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/flow';
import { Button } from '@the-dev-tools/ui/button';
import { CheckIcon } from '@the-dev-tools/ui/icons';
import { JsonTreeItem, jsonTreeItemProps } from '@the-dev-tools/ui/json-tree';
import { Menu, MenuItem, MenuItemLink, useContextMenuState } from '@the-dev-tools/ui/menu';
import { Select, SelectItem } from '@the-dev-tools/ui/select';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextInputField, useEditableTextState } from '@the-dev-tools/ui/text-field';
import { useEscapePortal } from '@the-dev-tools/ui/utils';
import { Connect, useApiCollection } from '~/api';
import { rootRouteApi } from '~/routes';
import { pick } from '~/utils/tanstack-db';
import { FlowContext } from './context';

export interface NodeStateContext {
  setNodeSelection: Dispatch<SetStateAction<HashSet.HashSet<string>>>;
}

export const NodeStateContext = createContext({} as NodeStateContext);

export const useNodesState = () => {
  const { transport } = rootRouteApi.useRouteContext();
  const { flowId } = useContext(FlowContext);

  const collection = useApiCollection(NodeCollectionSchema);

  const [selection, setSelection] = useState(HashSet.empty<string>());
  const [dimensions, setDimensions] = useState(HashMap.empty<string, { height: number; width: number }>());

  const items = pipe(
    useLiveQuery(
      (_) =>
        _.from({ item: collection })
          .where((_) => eq(_.item.flowId, flowId))
          .select((_) => pick(_.item, 'nodeId', 'position', 'kind')),
      [flowId, collection],
    ).data,
    Array.map((_): XF.Node => {
      const id = Ulid.construct(_.nodeId).toCanonical();
      return {
        data: {},
        id,
        measured: pipe(
          HashMap.get(dimensions, id),
          Option.getOrElse(() => ({ height: 0, width: 0 })),
        ),
        origin: [0.5, 0],
        position: _.position,
        selected: HashSet.has(selection, id),
        type: _.kind.toString(),
      };
    }),
  );

  const handlePositionChange = usePacedMutations<XF.NodePositionChange>({
    mutationFn: async ({ transaction }) => {
      const mutationTime = Date.now();
      const items = transaction.mutations.map((_) => ({
        ...collection.utils.parseKeyUnsafe(_.key as string),
        ..._.changes,
      }));
      await Connect.request({ input: { items }, method: FlowService.method.nodeUpdate, transport });
      await collection.utils.waitForSync(mutationTime);
    },
    onMutate: (_) => {
      if (!_.position) return;
      const { x, y } = _.position;
      const key = collection.utils.getKey({ nodeId: Ulid.fromCanonical(_.id).bytes });
      collection.update(key, (_) => {
        _.position.x = x;
        _.position.y = y;
      });
    },
    strategy: debounceStrategy({ wait: 500 }),
  });

  const onChange: XF.OnNodesChange = (_) => {
    const changes = Array.groupBy(_, (_) => _.type) as { [T in XF.NodeChange as T['type']]?: T[] };

    setSelection(
      HashSet.mutate(
        (selection) =>
          void changes.select?.forEach((_) => {
            if (_.selected) HashSet.add(selection, _.id);
            else HashSet.remove(selection, _.id);
          }),
      ),
    );

    setDimensions(
      HashMap.mutate(
        (dimensions) =>
          void changes.dimensions?.forEach((_) => {
            if (_.dimensions) HashMap.set(dimensions, _.id, _.dimensions);
          }),
      ),
    );

    changes.position?.forEach(handlePositionChange);

    if (changes.remove?.length)
      pipe(
        changes.remove,
        Array.map((_) => collection.utils.getKeyObject({ nodeId: Ulid.fromCanonical(_.id).bytes })),
        (_) => collection.utils.delete(_),
      );
  };

  return { nodes: items, onNodesChange: onChange, setNodeSelection: setSelection };
};

const nodeContainerStyles = tv({
  // eslint-disable-next-line better-tailwindcss/no-unregistered-classes
  base: tw`nopan relative w-80 rounded-lg bg-slate-200 p-1 shadow-xs outline-1 transition-colors`,
  variants: {
    isSelected: { true: tw`bg-slate-300` },
    state: {
      [FlowItemState.CANCELED]: tw`outline-slate-600`,
      [FlowItemState.FAILURE]: tw`outline-red-600`,
      [FlowItemState.RUNNING]: tw`outline-violet-600`,
      [FlowItemState.SUCCESS]: tw`outline-green-600`,
      [FlowItemState.UNSPECIFIED]: tw`outline-slate-300`,
    } satisfies Record<FlowItemState, string>,
  },
});

interface NodeContainerProps extends XF.NodeProps {
  children: ReactNode;
  handles?: ReactNode;
}

export const NodeContainer = ({ children, handles, id, selected }: NodeContainerProps) => {
  const collection = useApiCollection(NodeCollectionSchema);

  const { state } =
    useLiveQuery(
      (_) =>
        _.from({ item: collection })
          .where((_) => eq(_.item.nodeId, Ulid.fromCanonical(id).bytes))
          .select((_) => pick(_.item, 'state'))
          .findOne(),
      [collection, id],
    ).data ?? create(NodeSchema);

  return (
    <div className={nodeContainerStyles({ isSelected: selected, state })}>
      {children}
      {handles}
    </div>
  );
};

interface NodeBodyProps extends XF.NodeProps {
  children: ReactNode;
  Icon: IconType;
}

export const NodeBody = ({ children, Icon, id }: NodeBodyProps) => {
  const collection = useApiCollection(NodeCollectionSchema);

  const nodeId = Ulid.fromCanonical(id).bytes;

  const { info, name, state } =
    useLiveQuery(
      (_) =>
        _.from({ item: collection })
          .where((_) => eq(_.item.nodeId, nodeId))
          .select((_) => pick(_.item, 'state', 'name', 'info'))
          .findOne(),
      [collection, nodeId],
    ).data ?? create(NodeSchema);

  const { deleteElements, getZoom } = XF.useReactFlow();
  const { isReadOnly = false } = use(FlowContext);

  const ref = useRef<HTMLDivElement>(null);
  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  const { escapeRef, escapeRender } = useEscapePortal<HTMLButtonElement>(ref);

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    onSuccess: (_) => collection.utils.update({ name: _, nodeId }),
    value: name,
  });

  let stateIndicator = pipe(
    Match.value(state),
    Match.when(FlowItemState.RUNNING, () => (
      <TbRefresh className={tw`size-5 animate-spin text-violet-600`} style={{ animationDirection: 'reverse' }} />
    )),
    Match.when(FlowItemState.SUCCESS, () => <CheckIcon className={tw`size-5 text-green-600`} />),
    Match.when(FlowItemState.CANCELED, () => <TbCancel className={tw`size-5 text-slate-600`} />),
    Match.when(FlowItemState.FAILURE, () => <TbAlertTriangle className={tw`size-5 text-red-600`} />),
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

        <AriaButton
          className={tw`cursor-text truncate text-xs leading-5 font-medium tracking-tight`}
          onPress={() => void edit()}
          ref={escapeRef}
        >
          {name}
        </AriaButton>

        {isEditing &&
          escapeRender(
            <TextInputField
              aria-label='New node name'
              inputClassName={tw`-mx-2 mt-2 bg-white py-0.75`}
              {...textFieldProps}
            />,
            getZoom(),
          )}

        <div className={tw`flex-1`} />

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

              <MenuItem onAction={() => void deleteElements({ nodes: [{ id }] })} variant='danger'>
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

export interface NodePanelProps {
  nodeId: Uint8Array;
}

export interface NodeExecutionOutputProps {
  nodeExecutionId: Uint8Array;
}

interface NodeExecutionPanelProps {
  nodeId: Uint8Array;
  Output?: (props: NodeExecutionOutputProps) => ReactNode;
}

export const NodeExecutionPanel = ({ nodeId, Output }: NodeExecutionPanelProps) => {
  const collection = useApiCollection(NodeExecutionCollectionSchema);

  const { data: items } = useLiveQuery(
    (_) =>
      _.from({ item: collection })
        .where((_) => eq(_.item.nodeId, nodeId))
        .select((_) => pick(_.item, 'nodeExecutionId', 'name'))
        .orderBy((_) => _.item.nodeExecutionId, 'desc'),
    [collection, nodeId],
  );

  const firstItem = pipe(
    Array.head(items),
    Option.map((_) => collection.utils.getKey(_)),
    Option.getOrNull,
  );

  const [prevFirstItem, setPrevFirstItem] = useState<Key | null>(firstItem);
  const [selectedKey, setSelectedKey] = useState<Key | null>(firstItem);

  if (prevFirstItem !== firstItem) {
    setSelectedKey(firstItem);
    setPrevFirstItem(firstItem);
  }

  // Fix React Aria over-rendering non-visible components
  // https://github.com/adobe/react-spectrum/issues/8783#issuecomment-3233350825
  // TODO: move the workaround to an improved select component
  const [isOpen, setIsOpen] = useState(false);
  const listBoxItems = isOpen ? items : items.filter((_) => collection.utils.getKey(_) === selectedKey);

  return (
    <div className={tw`mx-5 my-4 overflow-auto rounded-lg border border-slate-200`}>
      <div
        className={tw`
          flex items-center justify-between border-b border-slate-200 bg-slate-50 px-3 py-2 text-md leading-5
          font-medium tracking-tight text-slate-800
        `}
      >
        <div>Execution data</div>

        {items.length > 1 && (
          <Select
            aria-label='Node execution'
            isOpen={isOpen}
            items={listBoxItems}
            onOpenChange={setIsOpen}
            onSelectionChange={setSelectedKey}
            selectedKey={selectedKey}
          >
            {(_) => <SelectItem id={collection.utils.getKey(_)}>{_.name}</SelectItem>}
          </Select>
        )}
      </div>

      <div className={tw`px-5 py-3`}>
        {typeof selectedKey !== 'string' ? (
          <div className={tw`text-sm`}>This node has not been executed yet</div>
        ) : (
          <NodeExecutionTabs
            key={selectedKey}
            nodeExecutionId={collection.utils.parseKeyUnsafe(selectedKey).nodeExecutionId}
            Output={Output}
          />
        )}
      </div>
    </div>
  );
};

interface NodeExecutionTabsProps {
  nodeExecutionId: Uint8Array;
  Output?: ((props: NodeExecutionOutputProps) => ReactNode) | undefined;
}

const NodeExecutionTabs = ({ nodeExecutionId, Output }: NodeExecutionTabsProps) => {
  const collection = useApiCollection(NodeExecutionCollectionSchema);

  const { input, output } =
    useLiveQuery(
      (_) =>
        _.from({ item: collection })
          .where((_) => eq(_.item.nodeExecutionId, nodeExecutionId))
          .select((_) => pick(_.item, 'input', 'output'))
          .findOne(),
      [collection, nodeExecutionId],
    ).data ?? create(NodeExecutionSchema);

  return (
    <Tabs className={tw`flex h-full flex-col pb-4`} defaultSelectedKey='output'>
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

      <div className={tw`flex-1 pt-4`}>
        <TabPanel id='input'>
          {input && (
            <Tree aria-label='Input values' defaultExpandedKeys={['root']} items={jsonTreeItemProps(input)!}>
              {(_) => <JsonTreeItem {..._} />}
            </Tree>
          )}
        </TabPanel>

        <TabPanel id='output'>
          {Output ? (
            <Output nodeExecutionId={nodeExecutionId} />
          ) : (
            output && (
              <Tree aria-label='Output values' defaultExpandedKeys={['root']} items={jsonTreeItemProps(output)!}>
                {(_) => <JsonTreeItem {..._} />}
              </Tree>
            )
          )}
        </TabPanel>
      </div>
    </Tabs>
  );
};
