import { create } from '@bufbuild/protobuf';
import {
  createCollection,
  createLiveQueryCollection,
  debounceStrategy,
  eq,
  localOnlyCollectionOptions,
  useLiveQuery,
  usePacedMutations,
} from '@tanstack/react-db';
import * as XF from '@xyflow/react';
import { Array, Match, Option, pipe, Schema } from 'effect';
import { Ulid } from 'id128';
import { ReactNode, useContext, useState } from 'react';
import { Button as AriaButton, Key, Tooltip, TooltipTrigger, Tree } from 'react-aria-components';
import { FiX } from 'react-icons/fi';
import { TbAlertTriangle, TbCancel, TbRefresh } from 'react-icons/tb';
import { Panel, Group as PanelGroup, useDefaultLayout } from 'react-resizable-panels';
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
import { SearchEmptyIllustration } from '@the-dev-tools/ui/illustrations';
import { JsonTreeItem, jsonTreeItemProps } from '@the-dev-tools/ui/json-tree';
import { PanelResizeHandle } from '@the-dev-tools/ui/resizable-panel';
import { Select, SelectItem } from '@the-dev-tools/ui/select';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextInputField, useEditableTextState } from '@the-dev-tools/ui/text-field';
import { request, useApiCollection } from '~/shared/api';
import { eqStruct, pick } from '~/shared/lib';
import { routes } from '~/shared/routes';
import { FlowContext } from './context';

class NodeClient extends Schema.Class<NodeClient>('NodeClient')({
  dimensions: pipe(
    Schema.Struct({ height: Schema.Number, width: Schema.Number }),
    Schema.optionalWith({ default: () => ({ height: 0, width: 0 }) }),
  ),
  nodeId: Schema.Uint8ArrayFromSelf,
  selected: pipe(Schema.Boolean, Schema.optionalWith({ default: () => false })),
}) {}

export const nodeClientCollection = createCollection(
  localOnlyCollectionOptions({
    getKey: (_) => Ulid.construct(_.nodeId).toCanonical(),
    schema: Schema.standardSchemaV1(NodeClient),
  }),
);

export const useNodesState = () => {
  const { transport } = routes.root.useRouteContext();
  const { flowId } = useContext(FlowContext);

  const nodeServerCollection = useApiCollection(NodeCollectionSchema);

  const items: XF.Node[] = useLiveQuery(
    (_) => {
      const server = _.from({ server: nodeServerCollection })
        .where((_) => eq(_.server.flowId, flowId))
        .fn.select((_) => ({ ..._.server, nodeId: Ulid.construct(_.server.nodeId).toCanonical() }));

      // This is suboptimal, but without creating a live query the data does not resolve sometimes for some reason
      const client = createLiveQueryCollection((_) =>
        _.from({ client: nodeClientCollection }).fn.select((_) => ({
          // eslint-disable-next-line @typescript-eslint/no-misused-spread
          ..._.client,
          nodeId: Ulid.construct(_.client.nodeId).toCanonical(),
        })),
      );

      return _.from({ server })
        .join({ client }, (_) => eq(_.server.nodeId, _.client.nodeId))
        .fn.select(
          (_): XF.Node => ({
            data: {},
            id: _.server.nodeId,
            measured: _.client?.dimensions ?? { height: 0, width: 0 },
            origin: [0.5, 0],
            position: _.server.position,
            selected: _.client?.selected ?? false,
            type: _.server.kind.toString(),
          }),
        );
    },
    [flowId, nodeServerCollection],
  ).data;

  const handlePositionChange = usePacedMutations<XF.NodePositionChange>({
    mutationFn: async ({ transaction }) => {
      const mutationTime = Date.now();
      const items = transaction.mutations.map((_) => ({
        ...nodeServerCollection.utils.parseKeyUnsafe(_.key as string),
        ..._.changes,
      }));
      await request({ input: { items }, method: FlowService.method.nodeUpdate, transport });
      await nodeServerCollection.utils.waitForSync(mutationTime);
    },
    onMutate: (_) => {
      if (!_.position) return;
      const { x, y } = _.position;
      const key = nodeServerCollection.utils.getKey({ nodeId: Ulid.fromCanonical(_.id).bytes });
      nodeServerCollection.update(key, (_) => {
        _.position.x = x;
        _.position.y = y;
      });
    },
    strategy: debounceStrategy({ wait: 500 }),
  });

  const onChange: XF.OnNodesChange = (_) => {
    const changes = Array.groupBy(_, (_) => _.type) as { [T in XF.NodeChange as T['type']]?: T[] };

    changes.select?.forEach(({ id, selected }) => {
      if (!nodeClientCollection.has(id)) nodeClientCollection.insert({ nodeId: Ulid.fromCanonical(id).bytes });
      nodeClientCollection.update(id, (_) => (_.selected = selected));
    });

    changes.dimensions?.forEach(({ dimensions, id }) => {
      if (!dimensions) return;
      if (!nodeClientCollection.has(id)) nodeClientCollection.insert({ nodeId: Ulid.fromCanonical(id).bytes });
      nodeClientCollection.update(id, (_) => (_.dimensions = dimensions));
    });

    changes.position?.forEach(handlePositionChange);

    if (changes.remove?.length) {
      pipe(
        changes.remove.map((_) => nodeServerCollection.utils.getKeyObject({ nodeId: Ulid.fromCanonical(_.id).bytes })),
        nodeServerCollection.utils.delete,
      );

      pipe(
        changes.remove.map((_) => _.id),
        nodeClientCollection.delete,
      );
    }
  };

  return { nodes: items, onNodesChange: onChange };
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

interface NodeBodyProps {
  children?: ReactNode;
  className?: string | undefined;
  icon: ReactNode;
  nodeId: Uint8Array;
  selected: boolean;
}

export const NodeBody = ({ children, className, icon, nodeId, selected }: NodeBodyProps) => {
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
      <div className={tw`absolute inset-0 size-full translate-y-1/2 rounded-full bg-current opacity-20 blur-lg`} />

      <div className={tw`flex size-full items-center gap-1 p-2.5`}>
        <div className={tw`text-[2.5rem]`}>{icon}</div>

        <div className={tw`absolute right-0 bottom-0`}>
          <NodeStateIndicator nodeId={nodeId} />
        </div>

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

  const { info, state } =
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
      tw`flex items-center gap-1 text-xs leading-4 font-semibold tracking-tight text-slate-800`,
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
          .select((_) => pick(_.item, 'name'))
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
          tw`pointer-events-auto block cursor-text text-xs tracking-tight text-slate-500`,
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
          inputClassName={tw`absolute top-0 left-0 w-24 bg-white px-1 py-0 text-xs`}
          {...textFieldProps}
        />
      )}
    </div>
  );
};

interface SimpleNodeProps {
  children?: ReactNode;
  className?: string;
  handles?: ReactNode;
  icon: ReactNode;
  nodeId: Uint8Array;
  selected: boolean;
  title?: ReactNode;
}

export const SimpleNode = ({ children, className, handles, icon, nodeId, selected, title }: SimpleNodeProps) => (
  <div className={tw`pointer-events-none flex flex-col`}>
    <NodeName className={tw`mb-1`} nodeId={nodeId} />

    <div className={tw`pointer-events-auto relative self-start`}>
      <NodeBody className={className} icon={icon} nodeId={nodeId} selected={selected}>
        {children}
      </NodeBody>

      {handles}
    </div>

    {title && <NodeTitle className={tw`mt-1`}>{title}</NodeTitle>}
  </div>
);

export interface NodeSettingsProps {
  nodeId: Uint8Array;
}

interface NodeSettingsContainerProps {
  children: ReactNode;
  className?: string;
  headerSlot?: ReactNode;
  nodeId: Uint8Array;
  title: string;
}

export const NodeSettingsContainer = ({
  children,
  className,
  headerSlot,
  nodeId,
  title,
}: NodeSettingsContainerProps) => {
  const nodeCollection = useApiCollection(NodeCollectionSchema);

  const { name } =
    useLiveQuery(
      (_) =>
        _.from({ item: nodeCollection })
          .where((_) => eq(_.item.nodeId, nodeId))
          .select((_) => pick(_.item, 'name'))
          .findOne(),
      [nodeCollection, nodeId],
    ).data ?? create(NodeSchema);

  return (
    <div className={tw`flex h-full flex-col`}>
      <div className={tw`flex items-center gap-4 border-b border-slate-200 bg-white px-5 py-2`}>
        <div className='min-w-0'>
          <div className={tw`text-md leading-5 text-slate-400`}>{name}</div>
          <div className={tw`truncate text-sm leading-5 font-medium text-slate-800`}>{title}</div>
        </div>

        <NodeStateIndicator nodeId={nodeId} />

        <div className={tw`flex-1`} />

        {headerSlot}

        <div className={tw`w-4`} />

        <Button className={tw`p-1`} slot='close' variant='ghost'>
          <FiX className={tw`size-5 text-slate-500`} />
        </Button>
      </div>

      <div className={twMerge(tw`size-full p-5`, className)}>{children}</div>
    </div>
  );
};

interface NodeSettingsBodyProps {
  children: ReactNode;
  input?: (nodeExecutionId: Uint8Array) => ReactNode;
  nodeId: Uint8Array;
  output?: (nodeExecutionId: Uint8Array) => ReactNode;
  settingsHeader?: ReactNode;
  title: string;
}

export const NodeSettingsBody = ({ children, input, nodeId, output, settingsHeader, title }: NodeSettingsBodyProps) => {
  const executionCollection = useApiCollection(NodeExecutionCollectionSchema);

  const { data: executions } = useLiveQuery(
    (_) => {
      const item = _.from({ item: executionCollection })
        .where((_) => eq(_.item.nodeId, nodeId))
        .fn.select((_) => ({
          ...pick(_.item, 'nodeExecutionId', 'name'),
          order: Ulid.construct(_.item.nodeExecutionId).toCanonical(),
        }));

      return _.from({ item }).orderBy((_) => _.item.order, 'desc');
    },
    [executionCollection, nodeId],
  );

  const latestExecutionId = pipe(
    Array.head(executions),
    Option.map((_) => _.nodeExecutionId),
    Option.getOrNull,
  );

  const latestExecutionIdCan = latestExecutionId ? Ulid.construct(latestExecutionId).toCanonical() : null;

  const [selectedExecKey, setSelectedExecKey] = useState<Key | null>(latestExecutionId ? 'latest' : null);

  const selectedExecutionId =
    selectedExecKey === 'latest'
      ? latestExecutionId
      : typeof selectedExecKey === 'string'
        ? Ulid.fromCanonical(selectedExecKey).bytes
        : null;

  const selectedExecutionIdCan = selectedExecutionId ? Ulid.construct(selectedExecutionId).toCanonical() : null;

  // Fix React Aria over-rendering non-visible components
  // https://github.com/adobe/react-spectrum/issues/8783#issuecomment-3233350825
  // TODO: move the workaround to an improved select component
  const [isExecListOpen, setIsExecListOpen] = useState(false);
  const execItems = isExecListOpen
    ? executions
    : executions.filter((_) => Ulid.construct(_.nodeExecutionId).toCanonical() === selectedExecutionIdCan);

  const nodeSettingsLayout = useDefaultLayout({ id: 'node-settings' });

  return (
    <NodeSettingsContainer
      className={tw`p-0`}
      headerSlot={
        executions.length > 1 && (
          <Select
            aria-label='Node execution'
            isOpen={isExecListOpen}
            items={execItems}
            onChange={setSelectedExecKey}
            onOpenChange={setIsExecListOpen}
            value={selectedExecKey}
          >
            {(_) => {
              let key = Ulid.construct(_.nodeExecutionId).toCanonical();
              if (key === latestExecutionIdCan) key = 'latest';

              return <SelectItem id={key}>{_.name}</SelectItem>;
            }}
          </Select>
        )
      }
      nodeId={nodeId}
      title={title}
    >
      <PanelGroup {...nodeSettingsLayout} className={tw`flex-1`} orientation='horizontal'>
        <Panel className={tw`flex min-h-0 flex-col`} defaultSize='30%' maxSize='40%' minSize='10%'>
          <div
            className={tw`border-b border-slate-200 p-5 text-base leading-5 font-semibold tracking-tight text-slate-800`}
          >
            Input
          </div>
          <div className={tw`flex-1 overflow-auto p-5`}>
            {!selectedExecutionId ? (
              <div className={tw`flex flex-col items-center py-14 text-center`}>
                <SearchEmptyIllustration />
                <div className={tw`mt-4 text-sm leading-5 font-semibold tracking-tight text-slate-800`}>
                  No input data yet
                </div>
                <div className={tw`w-48 text-md leading-4 tracking-tight text-slate-500`}>
                  The executed result from previous nodes will appear here
                </div>
              </div>
            ) : input ? (
              input(selectedExecutionId)
            ) : (
              <NodeSettingsBasicInput nodeExecutionId={selectedExecutionId} />
            )}
          </div>
        </Panel>

        <PanelResizeHandle direction='horizontal' />

        <Panel className={tw`flex min-h-0 flex-col`} defaultSize='40%' maxSize='60%' minSize='10%'>
          <div
            className={tw`
              flex items-center justify-between border-b border-slate-200 p-5 text-base leading-5 font-semibold
              tracking-tight text-slate-800
            `}
          >
            <span>Settings</span>
            {settingsHeader}
          </div>

          <div className={tw`flex-1 overflow-auto p-5`}>{children}</div>
        </Panel>

        <PanelResizeHandle direction='horizontal' />

        <Panel className={tw`flex min-h-0 flex-col`} defaultSize='30%' maxSize='40%' minSize='10%'>
          <div
            className={tw`border-b border-slate-200 p-5 text-base leading-5 font-semibold tracking-tight text-slate-800`}
          >
            Output
          </div>

          <div className={tw`flex-1 overflow-auto p-5`}>
            {!selectedExecutionId ? (
              <div className={tw`flex flex-col items-center py-14 text-center`}>
                <SearchEmptyIllustration />
                <div className={tw`mt-4 text-sm leading-5 font-semibold tracking-tight text-slate-800`}>
                  No output data yet
                </div>
                <div className={tw`w-48 text-md leading-4 tracking-tight text-slate-500`}>
                  The executed result from this node will appear here
                </div>
              </div>
            ) : output ? (
              output(selectedExecutionId)
            ) : (
              <NodeSettingsBasicOutput nodeExecutionId={selectedExecutionId} />
            )}
          </div>
        </Panel>
      </PanelGroup>
    </NodeSettingsContainer>
  );
};

export interface NodeSettingsInputProps {
  nodeExecutionId: Uint8Array;
}

const NodeSettingsBasicInput = ({ nodeExecutionId }: NodeSettingsInputProps) => {
  const collection = useApiCollection(NodeExecutionCollectionSchema);

  const { input } =
    useLiveQuery(
      (_) =>
        _.from({ item: collection })
          .where((_) => eq(_.item.nodeExecutionId, nodeExecutionId))
          .select((_) => pick(_.item, 'input'))
          .findOne(),
      [collection, nodeExecutionId],
    ).data ?? create(NodeExecutionSchema);

  return (
    <Tree aria-label='Input values' defaultExpandedKeys={['root']} items={jsonTreeItemProps(input)!}>
      {(_) => <JsonTreeItem {..._} />}
    </Tree>
  );
};

export interface NodeSettingsOutputProps {
  nodeExecutionId: Uint8Array;
}

const NodeSettingsBasicOutput = ({ nodeExecutionId }: NodeSettingsOutputProps) => {
  const collection = useApiCollection(NodeExecutionCollectionSchema);

  const { output } =
    useLiveQuery(
      (_) =>
        _.from({ item: collection })
          .where((_) => eq(_.item.nodeExecutionId, nodeExecutionId))
          .select((_) => pick(_.item, 'output'))
          .findOne(),
      [collection, nodeExecutionId],
    ).data ?? create(NodeExecutionSchema);

  return (
    <Tree aria-label='Output values' defaultExpandedKeys={['root']} items={jsonTreeItemProps(output)!}>
      {(_) => <JsonTreeItem {..._} />}
    </Tree>
  );
};
