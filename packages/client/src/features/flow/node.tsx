import { create } from '@bufbuild/protobuf';
import { debounceStrategy, eq, useLiveQuery, usePacedMutations } from '@tanstack/react-db';
import * as XF from '@xyflow/react';
import { Array, HashMap, HashSet, Match, Option, pipe } from 'effect';
import { Ulid } from 'id128';
import { createContext, Dispatch, ReactNode, SetStateAction, useContext, useState } from 'react';
import {
  Button as AriaButton,
  Key,
  Tab,
  TabList,
  TabPanel,
  Tabs,
  Tooltip,
  TooltipTrigger,
  Tree,
} from 'react-aria-components';
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
import { CheckIcon } from '@the-dev-tools/ui/icons';
import { JsonTreeItem, jsonTreeItemProps } from '@the-dev-tools/ui/json-tree';
import { Select, SelectItem } from '@the-dev-tools/ui/select';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextInputField, useEditableTextState } from '@the-dev-tools/ui/text-field';
import { Connect, useApiCollection } from '~/api';
import { rootRouteApi } from '~/routes';
import { eqStruct, pick } from '~/utils/tanstack-db';
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

const nodeBodyStyles = tv({
  base: tw`
    relative size-16 overflow-clip rounded-xl border-2 border-white bg-white outline outline-slate-800 transition-colors
  `,
  variants: {
    selected: { true: tw`bg-slate-200` },
    state: {
      [FlowItemState.CANCELED]: tw`outline-slate-300`,
      [FlowItemState.FAILURE]: tw`outline-red-600`,
      [FlowItemState.RUNNING]: tw`outline-violet-600`,
      [FlowItemState.SUCCESS]: tw`outline-green-600`,
      [FlowItemState.UNSPECIFIED]: tw`outline-slate-800`,
    } satisfies Record<FlowItemState, string>,
  },
});

interface NodeBodyNewProps {
  children?: ReactNode;
  className?: string;
  icon: ReactNode;
  nodeId: Uint8Array;
  selected: boolean;
}

export const NodeBodyNew = ({ children, className, icon, nodeId, selected }: NodeBodyNewProps) => {
  const collection = useApiCollection(NodeCollectionSchema);

  const { state } =
    useLiveQuery(
      (_) =>
        _.from({ item: collection })
          .where(eqStruct({ nodeId }))
          .select((_) => pick(_.item, 'state'))
          .findOne(),
      [collection, nodeId],
    ).data ?? create(NodeSchema);

  return (
    <div className={nodeBodyStyles({ className, selected, state })}>
      <div className={tw`absolute inset-0 size-full translate-y-1/2 rounded-full bg-current opacity-30 blur-lg`} />

      <div className={tw`flex size-full items-center gap-1 p-2.5`}>
        <div className={tw`text-[2.5rem]`}>{icon}</div>

        {children}
      </div>
    </div>
  );
};

interface NodeStateIndicatorProps {
  children?: ReactNode;
  nodeId: Uint8Array;
}

export const NodeStateIndicator = ({ children, nodeId }: NodeStateIndicatorProps) => {
  const collection = useApiCollection(NodeCollectionSchema);

  const { info = 'testing', state = FlowItemState.CANCELED } =
    useLiveQuery(
      (_) =>
        _.from({ item: collection })
          .where(eqStruct({ nodeId }))
          .select((_) => pick(_.item, 'state', 'info'))
          .findOne(),
      [collection, nodeId],
    ).data ?? create(NodeSchema);

  let indicator = pipe(
    Match.value(state),
    Match.when(FlowItemState.RUNNING, () => (
      <TbRefresh className={tw`size-5 animate-spin text-violet-600`} style={{ animationDirection: 'reverse' }} />
    )),
    Match.when(FlowItemState.SUCCESS, () => <CheckIcon className={tw`size-5 text-green-600`} />),
    Match.when(FlowItemState.CANCELED, () => <TbCancel className={tw`size-5 text-slate-600`} />),
    Match.when(FlowItemState.FAILURE, () => <TbAlertTriangle className={tw`size-5 text-red-600`} />),
    Match.orElse(() => children),
  );

  if (indicator && info)
    indicator = (
      <TooltipTrigger delay={750}>
        <AriaButton className={tw`pointer-events-auto block cursor-help`}>{indicator}</AriaButton>
        <Tooltip className={tw`max-w-lg rounded-md bg-slate-800 px-2 py-1 text-xs text-white`}>{info}</Tooltip>
      </TooltipTrigger>
    );

  return indicator;
};

interface NodeTitleProps {
  children: ReactNode;
  className?: string;
}

export const NodeTitle = ({ children, className }: NodeTitleProps) => (
  <div
    className={twMerge(
      tw`flex items-center gap-1 text-center text-xs leading-4 font-semibold tracking-tight text-slate-800`,
      className,
    )}
  >
    {children}
  </div>
);

interface NodeNameProps {
  className?: string;
  nodeId: Uint8Array;
}

export const NodeName = ({ className, nodeId }: NodeNameProps) => {
  const collection = useApiCollection(NodeCollectionSchema);

  const { name } =
    useLiveQuery(
      (_) =>
        _.from({ item: collection })
          .where((_) => eq(_.item.nodeId, nodeId))
          .select((_) => pick(_.item, 'state', 'name', 'info'))
          .findOne(),
      [collection, nodeId],
    ).data ?? create(NodeSchema);

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    onSuccess: (_) => collection.utils.update({ name: _, nodeId }),
    value: name,
  });

  return (
    <div className={tw`relative`}>
      <AriaButton
        className={twMerge(
          tw`pointer-events-auto mx-auto block cursor-text text-center text-xs tracking-tight text-slate-500`,
          isEditing && tw`opacity-0`,
          className,
        )}
        onPress={() => void edit()}
      >
        {name}
      </AriaButton>

      {isEditing && (
        <TextInputField
          aria-label='New node name'
          inputClassName={tw`absolute top-0 left-1/2 w-24 -translate-x-1/2 bg-white px-1 py-0 text-xs`}
          {...textFieldProps}
        />
      )}
    </div>
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
