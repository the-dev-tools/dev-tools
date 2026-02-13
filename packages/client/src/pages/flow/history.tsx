import { eq, useLiveQuery } from '@tanstack/react-db';
import { ReactFlowProvider } from '@xyflow/react';
import { Ulid } from 'id128';
import { Suspense, useRef } from 'react';
import { useTab, useTabList, useTabPanel } from 'react-aria';
import { Panel, Group as PanelGroup, useDefaultLayout } from 'react-resizable-panels';
import { Item, Node, TabListState, useTabListState } from 'react-stately';
import { twJoin } from 'tailwind-merge';
import { FlowVersion } from '@the-dev-tools/spec/buf/api/flow/v1/flow_pb';
import { FlowVersionCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/flow';
import { PanelResizeHandle } from '@the-dev-tools/ui/resizable-panel';
import { Spinner } from '@the-dev-tools/ui/spinner';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useApiCollection } from '~/shared/api';
import { routes } from '~/shared/routes';
import { FlowContext } from './context';
import { Flow, TopBar, TopBarWithControls } from './edit';

export const FlowHistoryPage = () => {
  const { flowId } = routes.dashboard.workspace.flow.route.useLoaderData();

  const collection = useApiCollection(FlowVersionCollectionSchema);

  const { data: versions } = useLiveQuery(
    (_) =>
      _.from({ item: collection })
        .where((_) => eq(_.item.flowId, flowId))
        .orderBy((_) => _.item.flowVersionId, 'desc'),
    [collection, flowId],
  );

  const state = useTabListState({
    children: ({ flowVersionId }) => (
      <Item key={Ulid.construct(flowVersionId).toCanonical()}>
        <Suspense
          fallback={
            <div className={tw`flex h-full items-center justify-center`}>
              <Spinner size='xl' />
            </div>
          }
        >
          <FlowContext.Provider value={{ flowId: flowVersionId, isReadOnly: true }}>
            <ReactFlowProvider>
              <div className={tw`flex h-full flex-col`}>
                <TopBarWithControls />
                <Flow key={Ulid.construct(flowVersionId).toCanonical()} />
              </div>
            </ReactFlowProvider>
          </FlowContext.Provider>
        </Suspense>
      </Item>
    ),
    items: versions,
  });

  const tabListRef = useRef(null);
  const { tabListProps } = useTabList({ items: versions, orientation: 'vertical' }, state, tabListRef);

  const flowHistoryLayout = useDefaultLayout({ id: 'flow-history' });

  return (
    <PanelGroup {...flowHistoryLayout} orientation='horizontal'>
      <Panel defaultSize='80%'>
        {!state.selectedKey && <TopBar />}
        <TabPanel state={state} />
      </Panel>

      <PanelResizeHandle direction='horizontal' />

      <Panel
        className={tw`flex flex-col bg-neutral-lower p-4 tracking-tight`}
        defaultSize='20%'
        maxSize='40%'
        minSize='10%'
        style={{ overflowY: 'auto' }}
      >
        <div className={tw`mb-4`}>
          <div className={tw`mb-0.5 text-sm leading-5 font-semibold text-on-neutral`}>Flow History</div>
          <div className={tw`text-xs leading-4 text-on-neutral-low`}>History of your flow responses</div>
        </div>
        <div className={tw`grid grid-cols-[auto_1fr] gap-x-0.5`}>
          <div className={tw`flex flex-col items-center gap-0.5`}>
            <div className={tw`flex-1`} />
            <div className={tw`size-2 rounded-full border border-accent p-px`}>
              <div className={tw`size-full rounded-full border border-inherit`} />
            </div>
            <div className={tw`w-px flex-1 bg-neutral`} />
          </div>

          <div className={tw`p-2 text-md leading-5 font-semibold tracking-tight text-accent`}>Current Version</div>

          <div className={tw`flex flex-col items-center gap-0.5`}>
            <div className={tw`w-px flex-1 bg-neutral`} />
            <div className={tw`size-2 rounded-full bg-neutral-high`} />
            <div className={tw`w-px flex-1 bg-neutral`} />
          </div>

          <div className={tw`p-2 text-md leading-5 font-semibold tracking-tight text-on-neutral`}>
            {versions.length} previous responses
          </div>

          <div className={tw`mb-2 w-px flex-1 justify-self-center bg-neutral`} />

          <div ref={tabListRef} {...tabListProps}>
            {[...state.collection].map((item) => (
              <Tab item={item} key={item.key} state={state} />
            ))}
          </div>
        </div>
      </Panel>
    </PanelGroup>
  );
};

interface TabProps {
  item: Node<FlowVersion>;
  state: TabListState<FlowVersion>;
}

const Tab = ({ item, state }: TabProps) => {
  const { key, value } = item;
  const ref = useRef(null);
  const { isSelected, tabProps } = useTab({ key }, state, ref);
  if (!value) return null;
  return (
    <div
      {...tabProps}
      className={twJoin(
        tabProps.className,
        tw`
          flex cursor-pointer items-center gap-1.5 rounded-md px-3 py-1.5 text-md leading-5 font-semibold
          text-on-neutral
        `,
        isSelected && tw`bg-neutral`,
      )}
      ref={ref}
    >
      {Ulid.construct(value.flowVersionId).time.toLocaleString()}
    </div>
  );
};

interface TabPanelProps {
  state: TabListState<FlowVersion>;
}

const TabPanel = ({ state }: TabPanelProps) => {
  const ref = useRef(null);
  const { tabPanelProps } = useTabPanel({}, state, ref);
  return (
    <div {...tabPanelProps} className={tw`size-full`} ref={ref}>
      {state.selectedItem?.rendered}
    </div>
  );
};
